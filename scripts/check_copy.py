"""Flag portfolio copy that reads like boilerplate.

This is a style check, not an AI authorship detector. It makes no prediction
about who or what wrote the text.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


LONG_DASHES = {"\u2013", "\u2014", "\u2015"}
CLICHES = re.compile(
    r"\b(?:passionate about|cutting[- ]edge|game[- ]changing|leverage|"
    r"seamless(?:ly)?|robust|innovative|powerful|comprehensive|"
    r"showcase|delve|foster|unlock|transformative)\b",
    re.IGNORECASE,
)
CYRILLIC = re.compile(r"[\u0400-\u04ff]")
LINK_TARGET = re.compile(r"\]\([^)]*\)")


def _emoji(char: str) -> bool:
    point = ord(char)
    return (0x2600 <= point <= 0x27BF or 0x1F1E6 <= point <= 0x1FAFF
            or point in {0x200D, 0xFE0F})


def lint_text(content: str, *, profile: bool = False) -> list[tuple[int, str]]:
    issues: list[tuple[int, str]] = []
    fenced = False
    for number, line in enumerate(content.splitlines(), 1):
        if line.lstrip().startswith(("```", "~~~")):
            fenced = not fenced
            continue
        if fenced:
            continue
        prose = LINK_TARGET.sub("]", line)
        if any(char in prose for char in LONG_DASHES):
            issues.append((number, "replace the long dash with punctuation or a short hyphen"))
        match = CLICHES.search(prose)
        if match:
            issues.append((number, f"replace stock phrase: {match.group(0)}"))
        if profile:
            if CYRILLIC.search(prose):
                issues.append((number, "profile copy must be in English"))
            if any(_emoji(char) for char in prose):
                issues.append((number, "remove emoji or decorative symbols"))
            if "![" in prose:
                issues.append((number, "remove image badges and stickers from the profile"))
    return issues


def main() -> int:
    parser = argparse.ArgumentParser(description="Check public Markdown for copy style cues, not AI authorship.")
    parser.add_argument("paths", nargs="+", type=Path)
    parser.add_argument("--profile", action="store_true", help="also require English and no emoji or image badges")
    args = parser.parse_args()
    found = False
    for path in args.paths:
        try:
            content = path.read_text(encoding="utf-8")
        except OSError as exc:
            print(f"{path}: {exc}")
            found = True
            continue
        for line, message in lint_text(content, profile=args.profile):
            print(f"{path}:{line}: {message}")
            found = True
    if not found:
        print("Copy check passed. Human review is still required.")
    return 1 if found else 0


if __name__ == "__main__":
    raise SystemExit(main())
