"""Parse instruction issues.

An instruction issue body looks like:

    read:
    - Arameair/TASS:cards/TASS.md
    - Arameair/identity@teams:teams/*.md

    change:
    - Arameair/identity@teams:teams/TEAM_LIAISON.md

    task:
    Add a rule that a DONE report without evidence is answered with a
    request for evidence, not escalated to the Project Lead.

`read:` and `change:` are optional. With no `change:` the Liaison only answers
in a comment. Text outside the sections is treated as part of the task.
A file reference is  owner/repo[@branch]:path  ("*" allowed in read paths).
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

REF = re.compile(r"^(?P<repo>[\w.-]+/[\w.-]+)(?:@(?P<ref>[^:\s]+))?:(?P<path>\S+)$")
SECTION = re.compile(r"^\s*(?:#+\s*)?(read|change|task)\s*:\s*(.*)$", re.I)
BULLET = re.compile(r"^\s*[-*+]\s+(?:\[[ xX]\]\s+)?`?(.+?)`?\s*$")


class InstructionError(ValueError):
    pass


@dataclass
class FileRef:
    repo: str
    ref: str | None
    path: str

    @classmethod
    def parse(cls, text: str) -> "FileRef":
        m = REF.match(text.strip())
        if not m:
            raise InstructionError(
                f"Can't understand file reference `{text.strip()}`. "
                "Use owner/repo:path or owner/repo@branch:path"
            )
        return cls(m["repo"], m["ref"], m["path"].lstrip("/"))

    @property
    def is_glob(self) -> bool:
        return any(c in self.path for c in "*?[")

    def __str__(self) -> str:
        return f"{self.repo}@{self.ref}:{self.path}" if self.ref else f"{self.repo}:{self.path}"


@dataclass
class Instruction:
    title: str
    task: str
    read: list[FileRef] = field(default_factory=list)
    change: list[FileRef] = field(default_factory=list)


def parse(title: str, body: str | None) -> Instruction:
    read: list[FileRef] = []
    change: list[FileRef] = []
    task_lines: list[str] = []
    section = "task"
    for line in (body or "").replace("\r\n", "\n").split("\n"):
        m = SECTION.match(line)
        if m:
            section = m.group(1).lower()
            rest = m.group(2).strip()
            if rest:
                if section == "task":
                    task_lines.append(rest)
                else:
                    (read if section == "read" else change).append(FileRef.parse(rest.strip("`")))
            continue
        if section in ("read", "change"):
            if not line.strip():
                continue
            b = BULLET.match(line)
            if not b:
                # A non-bullet line ends the list and belongs to the task.
                section = "task"
                task_lines.append(line)
                continue
            ref = FileRef.parse(b.group(1))
            if section == "change" and ref.is_glob:
                raise InstructionError(f"`change:` needs exact file paths, not patterns: `{ref}`")
            (read if section == "read" else change).append(ref)
        else:
            task_lines.append(line)
    task = "\n".join(task_lines).strip()
    if not task:
        task = title.strip()
    if not task:
        raise InstructionError("The issue has no task text.")
    return Instruction(title=title.strip(), task=task, read=read, change=change)
