"""Configuration loading. Defaults live in default_config.toml."""

from __future__ import annotations

import copy
import fnmatch
import os
import tomllib
from importlib import resources
from pathlib import Path


def default_text() -> str:
    return resources.files(__package__).joinpath("default_config.toml").read_text("utf-8")


def default_path() -> Path:
    env = os.environ.get("IDENTITY_LIAISON_CONFIG")
    if env:
        return Path(env).expanduser()
    base = os.environ.get("XDG_CONFIG_HOME") or str(Path.home() / ".config")
    return Path(base) / "identity-liaison" / "config.toml"


def _merge(base: dict, extra: dict) -> dict:
    out = copy.deepcopy(base)
    for key, value in extra.items():
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = _merge(out[key], value)
        else:
            out[key] = value
    return out


# Environment variables win over the config file (used by the Docker container).
ENV = {
    "IDENTITY_LIAISON_MODEL_URL": ("model", "url", str),
    "IDENTITY_LIAISON_MODE": ("write", "mode", str),
    "IDENTITY_LIAISON_REPOS": ("github", "instruction_repos", "list"),
    "IDENTITY_LIAISON_OWNERS": ("github", "owners", "list"),
    "IDENTITY_LIAISON_PULSE": ("run", "pulse_seconds", int),
    "IDENTITY_LIAISON_CHAT_REPOS": ("chat", "repos", "list"),
    "IDENTITY_LIAISON_CHAT_PULSE": ("chat", "pulse_seconds", int),
}


def _apply_env(data: dict) -> None:
    for name, (section, key, kind) in ENV.items():
        raw = os.environ.get(name, "").strip()
        if not raw:
            continue
        if kind == "list":
            data[section][key] = [x.strip() for x in raw.split(",") if x.strip()]
        else:
            data[section][key] = kind(raw)
    if data["write"]["mode"] not in ("pr", "direct"):
        raise ValueError(f"write mode must be \"pr\" or \"direct\", not {data['write']['mode']!r}")


class Config:
    def __init__(self, data: dict, path: Path | None = None):
        self.data = data
        self.path = path

    @classmethod
    def load(cls, path: Path | None = None) -> "Config":
        base = tomllib.loads(default_text())
        path = path or default_path()
        if path.exists():
            with open(path, "rb") as f:
                base = _merge(base, tomllib.load(f))
        _apply_env(base)
        return cls(base, path)

    def __getitem__(self, section: str) -> dict:
        return self.data[section]

    # --- permission checks -------------------------------------------------

    def may_read(self, repo: str) -> bool:
        return any(fnmatch.fnmatchcase(repo, p) for p in self["read"]["allow"])

    def may_write(self, repo: str, branch: str, path: str) -> bool:
        target = f"{repo}@{branch}:{path}"
        return any(fnmatch.fnmatchcase(target, p) for p in self["write"]["allow"])

    def is_owner(self, login: str) -> bool:
        owners = {o.lower() for o in self["github"]["owners"]}
        return login.lower() in owners

    def is_protected(self, branch: str) -> bool:
        return branch in self["write"]["protected_branches"]


def write_default(path: Path, force: bool = False) -> bool:
    if path.exists() and not force:
        return False
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(default_text(), "utf-8")
    return True
