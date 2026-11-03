"""Local model client with the context guard from the nemotronTASS labs.

Works with any OpenAI-compatible server. With llama.cpp it reads the real
context size (/props), applies the real chat template (/apply-template) and
counts real tokens (/tokenize) before every request. Other servers fall back
to a conservative estimate.
"""

from __future__ import annotations

import json
import re
import urllib.error
import urllib.request

THINK = re.compile(r"<think>.*?</think>", re.S)


class ModelError(RuntimeError):
    pass


class Model:
    def __init__(self, cfg: dict):
        self.url = cfg["url"].rstrip("/")
        self.name = cfg.get("name", "local")
        self.timeout = int(cfg.get("timeout_seconds", 600))
        self.context_fallback = int(cfg.get("context_fallback", 8192))
        self.margin = int(cfg.get("safety_margin", 256))
        self.min_output = int(cfg.get("min_output_tokens", 512))
        self.thinking = bool(cfg.get("thinking", False))

    def _call(self, path: str, payload=None, timeout: int | None = None):
        data = None if payload is None else json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(self.url + path, data=data,
                                     headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=timeout or self.timeout) as r:
            return json.load(r)

    # --- health -------------------------------------------------------------

    def health(self) -> str:
        try:
            self._call("/health", timeout=5)
        except Exception as e:  # noqa: BLE001
            raise ModelError(f"Model not reachable at {self.url}: {e}") from None
        return self.url

    # --- context guard ------------------------------------------------------

    def measure(self, messages: list[dict]) -> tuple[int, int, bool]:
        """Return (prompt_tokens, context_size, exact)."""
        try:
            props = self._call("/props", timeout=10)
            ctx = props.get("default_generation_settings", {}).get("n_ctx")
            if not isinstance(ctx, int) or ctx <= 0:
                raise ValueError("no n_ctx")
            prompt = self._call("/apply-template", {"messages": messages}, timeout=30)["prompt"]
            tokens = self._call("/tokenize", {"content": prompt, "add_special": True}, timeout=30)["tokens"]
            return len(tokens), ctx, True
        except (urllib.error.URLError, ValueError, KeyError, TypeError, OSError):
            chars = sum(len(m["content"]) for m in messages) + 16 * len(messages)
            return chars // 3 + 1, self.context_fallback, False

    # --- generation ---------------------------------------------------------

    def chat(self, messages: list[dict]) -> tuple[str, dict]:
        used, ctx, exact = self.measure(messages)
        available = ctx - used - self.margin
        if available < self.min_output:
            raise ModelError(
                f"Context guard BLOCKED: prompt {used} tokens, context {ctx}, "
                f"only {max(available, 0)} left for the answer (need {self.min_output}). "
                "Read fewer files or raise the server context (-c)."
            )
        payload = {
            "model": self.name,
            "messages": messages,
            "max_tokens": available,
            "stream": False,
        }
        if not self.thinking:
            payload["chat_template_kwargs"] = {"enable_thinking": False}
        try:
            result = self._call("/v1/chat/completions", payload)
        except urllib.error.HTTPError as e:
            raise ModelError(f"Model HTTP {e.code}: {e.read()[:200]!r}") from None
        except (urllib.error.URLError, OSError) as e:
            raise ModelError(f"Model not reachable at {self.url}: {e}") from None
        choice = result["choices"][0]
        if choice.get("finish_reason") == "length":
            raise ModelError("Model output was truncated; nothing was written.")
        text = choice.get("message", {}).get("content") or ""
        text = THINK.sub("", text).strip()
        if not text:
            raise ModelError("Model returned no visible reply.")
        return text, {"prompt_tokens": used, "context": ctx, "exact_count": exact}
