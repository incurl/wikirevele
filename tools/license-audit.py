#!/usr/bin/env python3
"""Audit the licences of every Go and npm dependency.

The project must not ship a GPL or AGPL dependency, so the audit fails
when one is found. It also prints the licence breakdown, which is what
the THIRD_PARTY_NOTICES file is derived from.

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

# Copyleft licences that must never appear in the dependency tree.
FORBIDDEN = ("GPL", "AGPL", "LGPL", "SSPL", "EUPL", "OSL")

# The licences that are acceptable, by the token found in the file.
LICENCE_PATTERNS = (
    # MPL must be checked before the GPL family: the MPL text references
    # the GNU General Public License, so a naive substring match would
    # misclassify it.
    ("MPL-2.0", "mozilla public license"),
    ("AGPL", "gnu affero general public license"),
    ("LGPL", "gnu lesser general public license"),
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

    for directory in modules:
        licence = detect_licence(directory)
        breakdown[licence] += 1

        if licence in FORBIDDEN:
            violations.append((directory, licence))

    print("Licence breakdown:")
    for licence, count in sorted(breakdown.items(), key=lambda item: (-item[1], item[0])):
        print(f"  {licence:<14} {count}")

    total = len(modules)
    print(f"\n{total} dependencies audited.")

    if violations:
        print("\nForbidden licences found:", file=sys.stderr)
        for directory, licence in violations:
            print(f"  {licence}: {directory}", file=sys.stderr)
        print("\nFAIL: a copyleft dependency would be shipped.", file=sys.stderr)
        return 1

    print("PASS: no copyleft dependency would be shipped.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
