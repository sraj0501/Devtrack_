#!/usr/bin/env python3
"""Check Sage's parity ledger; optionally reconcile the pinned reference via Git.

This is repository tooling, not a Sage runtime dependency.
"""

from __future__ import annotations

import argparse
import ast
from collections import Counter
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "docs/SAGE_PORT_PARITY_MATRIX.md"
BASELINE = "b85a1ab"
REFERENCE_FILES = ("tool/tests/test_ide_capture.py", "tool/tests/test_portable.py")
SCENARIO = re.compile(r"`([^`]+):(\d+)` `(test_\w+)`")
GO_TEST = re.compile(r"`([^`]+_test\.go)#(Test\w+)`")


def check(text: str, root: Path = ROOT) -> tuple[list[str], Counter]:
    errors: list[str] = []
    counts: Counter = Counter()
    scenarios: set[tuple[str, str]] = set()
    rows = [line for line in text.splitlines() if line.startswith("| `tool/tests/")]
    if len(rows) != 136:
        errors.append(f"expected 136 scenarios, found {len(rows)}")
    for number, row in enumerate(rows, 1):
        cells = [cell.strip() for cell in row.strip("|").split("|")]
        if len(cells) != 7:
            errors.append(f"row {number}: expected seven columns")
            continue
        scenario, owner, assignment, status, test, planned, note = cells
        match = SCENARIO.fullmatch(scenario)
        if not match or match[1] not in REFERENCE_FILES:
            errors.append(f"row {number}: invalid reference scenario")
            continue
        key = (match[1], match[3])
        if key in scenarios:
            errors.append(f"duplicate scenario: {key}")
        scenarios.add(key)
        if not owner or assignment not in {"SAGE-001", "SAGE-002", "SAGE-003", "SAGE-004"}:
            errors.append(f"{key}: missing owner or invalid assignment")
        if status not in {"implemented", "partial", "pending", "adapted"}:
            errors.append(f"{key}: invalid status {status!r}")
        counts[status] += 1
        if status in {"pending", "partial"} and not re.fullmatch(r"`Test\w+`", planned):
            errors.append(f"{key}: missing exact planned closure test")
        if not note or (status == "adapted" and "Adaptation:" not in note):
            errors.append(f"{key}: missing rationale")
        references = GO_TEST.findall(test)
        if status != "pending" and not references:
            errors.append(f"{key}: status requires existing Go test evidence")
        if test != "—" and not references:
            errors.append(f"{key}: malformed Go test reference")
        for filename, name in references:
            path = (root / "devtrack_client" / filename).resolve()
            if not path.is_relative_to((root / "devtrack_client").resolve()) or not path.is_file():
                errors.append(f"{key}: missing or unsafe Go test file {filename}")
                continue
            declaration = rf"(?m)^func {re.escape(name)}\(t \*testing\.T\)"
            if not re.search(declaration, path.read_text(encoding="utf-8")):
                errors.append(f"{key}: missing Go test {name} in {filename}")
    return errors, counts


def check_reference(text: str, repository: Path) -> list[str]:
    expected = set()
    for filename in REFERENCE_FILES:
        source = subprocess.run(
            ["git", "-C", str(repository), "show", f"{BASELINE}:{filename}"],
            check=True, capture_output=True, encoding="utf-8",
        ).stdout
        tree = ast.parse(source)
        for node in ast.walk(tree):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name.startswith("test_"):
                expected.add((filename, str(node.lineno), node.name))
    actual = set(SCENARIO.findall(text))
    errors = []
    if len(expected) != 136:
        errors.append(f"pinned reference contains {len(expected)} scenarios, expected 136")
    for item in sorted(expected - actual):
        errors.append(f"reference scenario missing or moved in matrix: {item}")
    for item in sorted(actual - expected):
        errors.append(f"matrix scenario absent from pinned reference: {item}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", type=Path, help="checkout containing Git commit b85a1ab")
    parser.add_argument("--require-complete", action="store_true", help="fail while any scenarios remain open")
    args = parser.parse_args()
    text = MATRIX.read_text(encoding="utf-8")
    errors, counts = check(text)
    if args.reference:
        try:
            errors.extend(check_reference(text, args.reference))
        except (OSError, subprocess.CalledProcessError, SyntaxError) as exc:
            errors.append(f"could not verify pinned reference: {exc}")
    if args.require_complete and (counts["pending"] or counts["partial"]):
        errors.append("parity remains incomplete")
    for error in errors:
        print(f"FAIL: {error}")
    print(
        f"Sage ledger: {sum(counts.values())} scenarios; "
        + ", ".join(f"{status}={counts[status]}" for status in ("implemented", "adapted", "partial", "pending"))
    )
    if not args.reference:
        print("Reference source not checked; use --reference PATH to reconcile b85a1ab.")
    if not errors:
        print("PASS: ledger consistency only; run Go tests separately for behavioral evidence.")
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
