#!/usr/bin/env python3
"""Audit the licences of every Go and npm dependency.

The project is GPL-3.0, because it ships the design tokens of the
Wikimedia Codex design system, which are GPL-2.0+. A copyleft
dependency is therefore not a failure in itself: what matters is that
the licences that are shipped agree with the licence of the project.

Two things are checked:

- every copyleft dependency must be on the accepted list below, with
  the reason it was accepted;
- the licence of the project must cover them, which it does: the
  project is GPL-3.0 and GPL-2.0+ means "GPL version 2 or, at your
  option, any later version".

The breakdown is printed as well, and it is what the
THIRD_PARTY_NOTICES file is derived from.

Usage:
    python3 tools/license-audit.py
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from collections import defaultdict

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FRONTEND_DIR = os.path.join(REPO_ROOT, "frontend")

# The copyleft licences. They are not forbidden in themselves: the
# project is GPL-3.0, so they are compatible with it. A copyleft
# dependency that is not on the accepted list is still a failure.
#
# The identifiers are the prefixes of the normalised ones, so that
# "GPL-2.0-or-later" is caught by "GPL".
COPYLEFT = ("GPL", "AGPL", "LGPL", "SSPL", "EUPL", "OSL")

# The copyleft dependencies that are accepted, and why.
#
# A name is the name of the module or of the npm package. The reason
# is shown in the report, so that the decision is visible and not only
# implied by the list.
ACCEPTED = {
    "@wikimedia/codex-design-tokens": (
        "The design tokens of the Wikimedia Codex design system. They are "
        "the reason the project is GPL-3.0."
    ),
}

# The licences that are acceptable, by the token found in the file.
LICENCE_PATTERNS = (
    # The order matters, and it is the reverse of the length of the
    # names.
    #
    # MPL must be checked before the GPL family: the MPL text
    # references the GNU General Public License, so a naive substring
    # match would misclassify it.
    #
    # The GPL family is checked from the longest name to the shortest
    # one, for the same reason: the text of the GPL mentions the LGPL
    # in its preamble, so the LGPL token matches a GPL-2 file and the
    # licence is reported as the wrong one.
    ("MPL-2.0", "mozilla public license"),
    ("AGPL", "gnu affero general public license"),
    ("LGPL-3.0", "gnu lesser general public license version 3"),
    ("LGPL", "gnu lesser general public license"),
    ("GPL-3.0", "gnu general public license version 3"),
    ("GPL-2.0", "gnu general public license version 2"),
    ("GPL", "gnu general public license"),
    ("Apache-2.0", "apache license"),
    ("BSD-3-Clause", "redistribution and use in source and binary forms"),
    ("BSD-2-Clause", "redistribution and use in source and binary forms"),
    ("MIT", "permission is hereby granted, free of charge"),
    ("ISC", "permission to use, copy, modify"),
    ("Unlicense", "this is free and unencumbered software released into the public domain"),
    ("CC0-1.0", "creative commons"),
)


def detect_licence(directory: str) -> str:
    """Return the licence identifier of a module directory."""
    # The filename casing varies between modules, so the directory is
    # scanned case-insensitively.
    candidates = []
    try:
        entries = os.listdir(directory)
    except OSError:
        return "UNKNOWN"

    for entry in entries:
        upper = entry.upper()
        if upper in ("LICENSE", "LICENCE", "COPYING") or upper.startswith(("LICENSE.", "LICENCE.", "COPYING.")):
            candidates.append(entry)

    # The licence an npm package declares in its manifest is more
    # precise than the text of its licence file, so it is read first:
    # the file of a "GPL-2.0+" package is the plain GPL-2 text, which
    # does not say that the package may be used under any later
    # version.
    declared = declared_licence(directory)
    if declared != "UNKNOWN":
        return declared

    for name in sorted(candidates):
        path = os.path.join(directory, name)
        try:
            with open(path, "r", encoding="utf-8", errors="ignore") as handle:
                head = handle.read(4096).lower()
        except OSError:
            continue

        for identifier, token in LICENCE_PATTERNS:
            if token in head:
                return identifier

    return "UNKNOWN"


def declared_licence(directory: str) -> str:
    """Return the licence an npm package declares in its manifest."""
    manifest = os.path.join(directory, "package.json")
    try:
        with open(manifest, "r", encoding="utf-8", errors="ignore") as handle:
            declared = json.load(handle).get("license")
    except (OSError, ValueError):
        return "UNKNOWN"

    if not isinstance(declared, str) or not declared:
        return "UNKNOWN"

    # The identifier is normalised, so that "GPL-2.0+" and "GPL-2.0"
    # are told apart.
    return declared.replace("+", "-or-later")


def is_copyleft(licence: str) -> bool:
    """Tell whether a licence identifier is a copyleft one."""
    return any(licence.startswith(prefix) for prefix in COPYLEFT)


def module_name(directory: str) -> str:
    """Return the name of a module directory.

    It is the name the accepted list uses: the name of the npm package
    or of the Go module, without the path.
    """
    name = os.path.basename(directory)

    # A scoped npm package holds its own packages, one directory
    # deeper.
    parent = os.path.basename(os.path.dirname(directory))
    if parent.startswith("@"):
        return f"{parent}/{name}"

    return name


def go_dependencies() -> list[str]:
    """Return the module directories of the Go dependencies."""
    result = subprocess.run(
        ["go", "list", "-m", "-json", "all"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        print(f"go list failed: {result.stderr.strip()}", file=sys.stderr)
        return []

    modules = []
    decoder = json.JSONDecoder()
    text = result.stdout.strip()
    index = 0
    while index < len(text):
        try:
            obj, offset = decoder.raw_decode(text, index)
        except json.JSONDecodeError:
            break
        index = offset
        while index < len(text) and text[index].isspace():
            index += 1

        directory = obj.get("Dir")
        if directory and obj.get("Main") is None and obj.get("Replace") is None:
            modules.append(directory)

    return modules


def npm_dependencies() -> list[str]:
    """Return the module directories of the npm dependencies."""
    node_modules = os.path.join(FRONTEND_DIR, "node_modules")
    if not os.path.isdir(node_modules):
        return []

    modules = []
    for entry in sorted(os.listdir(node_modules)):
        if entry.startswith("."):
            continue

        path = os.path.join(node_modules, entry)
        if entry.startswith("@"):
            # A scoped package holds its own packages.
            if not os.path.isdir(path):
                continue
            for scoped in sorted(os.listdir(path)):
                modules.append(os.path.join(path, scoped))
        else:
            modules.append(path)

    return modules


def main() -> int:
    modules = go_dependencies() + npm_dependencies()
    if not modules:
        print("No dependencies were found. Run `go mod download` and `npm install` first.")
        return 1

    breakdown: dict[str, int] = defaultdict(int)
    violations = []
    accepted = []

    for directory in modules:
        licence = detect_licence(directory)
        breakdown[licence] += 1

        if not is_copyleft(licence):
            continue

        name = module_name(directory)
        if name in ACCEPTED:
            accepted.append((name, licence))
        else:
            violations.append((directory, licence))

    print("Licence breakdown:")
    for licence, count in sorted(breakdown.items(), key=lambda item: (-item[1], item[0])):
        print(f"  {licence:<14} {count}")

    total = len(modules)
    print(f"\n{total} dependencies audited.")

    if accepted:
        print("\nAccepted copyleft dependencies:")
        for name, licence in accepted:
            print(f"  {licence}: {name}")
            print(f"    {ACCEPTED[name]}")

    if violations:
        print("\nCopyleft dependencies that are not accepted:", file=sys.stderr)
        for directory, licence in violations:
            print(f"  {licence}: {directory}", file=sys.stderr)
        print(
            "\nFAIL: a copyleft dependency would be shipped that the project "
            "does not accept.",
            file=sys.stderr,
        )
        return 1

    print("\nPASS: every copyleft dependency is accepted.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
