"""Command line: identity-liaison <command>."""

from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

from . import __version__
from .agent import Agent
from .chat import SideChat
from .config import Config, default_path, write_default
from .github import GitHub, find_token
from .instructions import FileRef, Instruction
from .model import Model, ModelError


def _load(args) -> Config:
    return Config.load(Path(args.config).expanduser() if args.config else None)


def _github(cfg: Config) -> GitHub:
    return GitHub(find_token(cfg["github"]["token"]), cfg["github"]["api_url"])


def _agent(args, cfg: Config) -> Agent:
    return Agent(cfg, _github(cfg), Model(cfg["model"]), dry_run=getattr(args, "dry_run", False))


def cmd_init(args) -> int:
    path = Path(args.config).expanduser() if args.config else default_path()
    if write_default(path, force=args.force):
        print(f"Wrote config: {path}")
    else:
        print(f"Config already exists: {path}  (use --force to replace it)")
    print("Next: edit it if needed, then run  identity-liaison check")
    return 0


def cmd_check(args) -> int:
    cfg = _load(args)
    ok = True
    print(f"Config: {cfg.path} {'(found)' if cfg.path and cfg.path.exists() else '(not found; using defaults)'}")
    try:
        gh = _github(cfg)
        login = gh.whoami()
        print(f"PASS  GitHub signed in as {login}")
        if not cfg.is_owner(login):
            print(f"NOTE  {login} is not in [github] owners; its issues will be ignored")
        for repo in cfg["github"]["instruction_repos"]:
            branch = gh.default_branch(repo)
            print(f"PASS  instruction repo {repo} reachable (default branch {branch})")
    except Exception as e:  # noqa: BLE001
        ok = False
        print(f"FAIL  GitHub: {e}")
    model = Model(cfg["model"])
    try:
        model.health()
        used, ctx, exact = model.measure([{"role": "user", "content": "Hello TASS"}])
        print(f"PASS  model at {model.url}: context {ctx} tokens"
              + ("" if exact else " (server does not report it; using context_fallback)"))
    except ModelError as e:
        ok = False
        print(f"FAIL  {e}")
    print(f"Write mode: {cfg['write']['mode']}   Owners: {', '.join(cfg['github']['owners'])}")
    return 0 if ok else 1


def cmd_read(args) -> int:
    cfg = _load(args)
    agent = Agent(cfg, _github(cfg), Model(cfg["model"]), log=lambda *_: None)
    for s in agent.read_sources([FileRef.parse(x) for x in args.files]):
        print(f"===== {s.ref} =====\n{s.text}")
    return 0


def cmd_ask(args) -> int:
    cfg = _load(args)
    agent = _agent(args, cfg)
    instr = Instruction(title=args.task[:80], task=args.task,
                        read=[FileRef.parse(x) for x in args.read],
                        change=[FileRef.parse(x) for x in args.change])
    agent.show(agent.execute(instr))
    print("\n(Preview only: nothing was written to GitHub. Open an issue to have changes committed.)")
    return 0


def cmd_new(args) -> int:
    cfg = _load(args)
    gh = _github(cfg)
    repo = args.repo or cfg["github"]["instruction_repos"][0]
    parts = []
    if args.read:
        parts.append("read:\n" + "\n".join(f"- {x}" for x in args.read))
    if args.change:
        parts.append("change:\n" + "\n".join(f"- {x}" for x in args.change))
    parts.append("task:\n" + args.task)
    label = cfg["github"]["label"]
    gh.ensure_label(repo, label, "1d76db", "Instruction for identity-liaison")
    issue = gh.create_issue(repo, args.title or args.task[:70], "\n\n".join(parts), [label])
    print(f"Created {issue['html_url']}")
    return 0


def cmd_once(args) -> int:
    cfg = _load(args)
    n = _agent(args, cfg).pulse()
    print(f"\nHandled {n} instruction(s).")
    return 0


def _chat(args, cfg: Config) -> SideChat:
    return SideChat(cfg, _github(cfg), Model(cfg["model"]), dry_run=getattr(args, "dry_run", False))


def cmd_chat_once(args) -> int:
    cfg = _load(args)
    n = _chat(args, cfg).pulse()
    print(f"\nReplied to {n} side chat(s).")
    return 0


def cmd_run(args) -> int:
    cfg = _load(args)
    agent = chat = None
    pulse = int(cfg["run"]["pulse_seconds"])
    chat_repos = cfg["chat"]["repos"]
    chat_pulse = int(cfg["chat"]["pulse_seconds"]) if chat_repos else pulse
    tick = min(pulse, chat_pulse)
    print(f"identity-liaison {__version__} (model {cfg['model']['url']}, mode {cfg['write']['mode']})")
    print(f"  instructions: {', '.join(cfg['github']['instruction_repos'])} every {pulse}s")
    if chat_repos:
        print(f"  side chat:    {', '.join(chat_repos)} (label '{cfg['chat']['label']}') every {chat_pulse}s")
    print("Ctrl+C to stop.")
    next_task = next_chat = 0.0
    try:
        while True:
            now = time.monotonic()
            stamp = time.strftime("%H:%M:%S")
            if chat_repos and now >= next_chat:
                next_chat = now + chat_pulse
                try:
                    chat = chat or _chat(args, cfg)
                    n = chat.pulse()
                    if n:
                        print(f"[{stamp}] side chat: replied {n}")
                except Exception as e:  # noqa: BLE001 — keep the loop alive
                    print(f"[{stamp}] side chat failed: {e}")
            if now >= next_task:
                next_task = now + pulse
                try:
                    # Set up inside the loop so a missing token is reported each pulse
                    # instead of crashing (and restart-looping) the container.
                    agent = agent or _agent(args, cfg)
                    n = agent.pulse()
                    print(f"[{stamp}] pulse: {n} instruction(s)")
                except Exception as e:  # noqa: BLE001 — keep the loop alive
                    print(f"[{stamp}] pulse failed: {e}")
            time.sleep(max(1.0, min(next_chat if chat_repos else next_task, next_task) - time.monotonic()))
    except KeyboardInterrupt:
        print("\nStopped.")
    return 0


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="identity-liaison",
                                description="Take instructions from GitHub issues and update md cards with a local model.")
    p.add_argument("--version", action="version", version=__version__)
    p.add_argument("--config", help=f"config file (default {default_path()})")
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("init", help="write the default config file")
    s.add_argument("--force", action="store_true")
    s.set_defaults(fn=cmd_init)

    s = sub.add_parser("check", help="check GitHub sign-in and the local model")
    s.set_defaults(fn=cmd_check)

    s = sub.add_parser("read", help="print md files from GitHub, e.g. Arameair/identity@teams:teams/*.md")
    s.add_argument("files", nargs="+")
    s.set_defaults(fn=cmd_read)

    s = sub.add_parser("ask", help="try an instruction locally and preview the result (writes nothing)")
    s.add_argument("task")
    s.add_argument("--read", action="append", default=[], metavar="REF")
    s.add_argument("--change", action="append", default=[], metavar="REF")
    s.set_defaults(fn=cmd_ask)

    s = sub.add_parser("new", help="open an instruction issue on GitHub")
    s.add_argument("task")
    s.add_argument("--title")
    s.add_argument("--repo", help="instruction repo (default: first in config)")
    s.add_argument("--read", action="append", default=[], metavar="REF")
    s.add_argument("--change", action="append", default=[], metavar="REF")
    s.set_defaults(fn=cmd_new)

    for name, help_ in (("once", "handle waiting instructions once, then exit"),
                        ("run", "keep running and check GitHub every pulse")):
        s = sub.add_parser(name, help=help_)
        s.add_argument("--dry-run", action="store_true",
                       help="read and think, but don't comment, label or write anything")
        s.set_defaults(fn=cmd_once if name == "once" else cmd_run)

    s = sub.add_parser("chat-once", help="answer waiting side chat messages once, then exit")
    s.add_argument("--dry-run", action="store_true", help="print replies instead of posting them")
    s.set_defaults(fn=cmd_chat_once)

    args = p.parse_args(argv)
    try:
        return args.fn(args)
    except Exception as e:  # noqa: BLE001
        print(f"ERROR: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
