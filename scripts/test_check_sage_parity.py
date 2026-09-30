"""Fault-injection checks for the parity ledger gate (standard library only)."""

from pathlib import Path
import unittest
from unittest.mock import patch

import check_sage_parity as parity


class ParityGateTests(unittest.TestCase):
    def setUp(self):
        self.text = parity.MATRIX.read_text(encoding="utf-8")

    def test_current_ledger_has_all_scenarios_and_resolvable_evidence(self):
        errors, counts = parity.check(self.text)
        self.assertEqual(errors, [])
        self.assertEqual(sum(counts.values()), 136)

    def test_missing_scenario_fails(self):
        lines = self.text.splitlines()
        lines.remove(next(line for line in lines if line.startswith("| `tool/tests/")))
        errors, _ = parity.check("\n".join(lines))
        self.assertTrue(any("expected 136" in error for error in errors))

    def test_duplicate_scenario_cannot_hide_a_missing_one(self):
        rows = [line for line in self.text.splitlines() if line.startswith("| `tool/tests/")]
        errors, _ = parity.check(self.text.replace(rows[1], rows[0]))
        self.assertTrue(any("duplicate scenario" in error for error in errors))

    def test_nonexistent_go_test_fails(self):
        errors, _ = parity.check(self.text.replace("#TestParseDistinguishesSkipFromMalformed", "#TestMissingParityEvidence"))
        self.assertTrue(any("missing Go test TestMissingParityEvidence" in error for error in errors))

    def test_status_cannot_claim_completion_without_evidence(self):
        errors, _ = parity.check(self.text.replace("| pending |", "| implemented |", 1))
        self.assertTrue(any("requires existing Go test evidence" in error for error in errors))

    def test_missing_planned_closure_test_fails(self):
        errors, _ = parity.check(self.text.replace("`TestReferenceCapturesOriginalCommandOnceAndExcludesCliAndOldHistory`", "—"))
        self.assertTrue(any("missing exact planned" in error for error in errors))

    def test_adaptation_requires_rationale(self):
        errors, _ = parity.check(self.text.replace("| implemented |", "| adapted |", 1))
        self.assertTrue(any("missing rationale" in error for error in errors))

    def test_pinned_source_mismatch_fails(self):
        with patch.object(parity.subprocess, "run") as run:
            run.return_value.stdout = "def test_different():\n    pass\n"
            errors = parity.check_reference(self.text, Path("reference"))
        self.assertTrue(any("reference scenario missing" in error for error in errors))
        self.assertTrue(any("matrix scenario absent" in error for error in errors))
        self.assertTrue(all("b85a1ab:" in call.args[0][-1] for call in run.call_args_list))


if __name__ == "__main__":
    unittest.main()
