#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("analysis", Path(__file__).with_name("analyze-macos-stability.py"))
analysis = importlib.util.module_from_spec(spec)
spec.loader.exec_module(analysis)


class EvidenceTests(unittest.TestCase):
    def test_duplicate_successes_do_not_replace_a_missing_cycle(self):
        with self.assertRaises(ValueError):
            analysis.verify_cycles([{"kind": "card", "cycle": 1, "success": True}] * 2, "card", 2)

    def test_failed_cycle_is_not_counted_as_success(self):
        with self.assertRaises(ValueError):
            analysis.verify_cycles([{"kind": "card", "cycle": 1, "success": False}], "card", 1)

    def test_short_or_instrumented_or_different_binary_baseline_is_rejected(self):
        p = {"status": "complete", "elapsedSeconds": 600}
        e = {"binarySHA256": "a", "rootArguments": "sidelet -data-dir test"}
        analysis.validate_profiles(p, p, e, e)
        for bad_profile, bad_environment, other_environment in [
            ({**p, "elapsedSeconds": 599}, e, e),
            (p, {**e, "rootArguments": "sidelet -memory-dir test"}, e),
            (p, e, {**e, "binarySHA256": "b"}),
        ]:
            with self.subTest(bad_profile=bad_profile, bad_environment=bad_environment):
                with self.assertRaises(ValueError):
                    analysis.validate_profiles(bad_profile, p, bad_environment, other_environment)

    def test_recovery_excludes_interaction_peak(self):
        rows = [{"timestamp": f"2026-10-05T00:09:{sec:02d}Z", "totalRSSBytes": 2**20, "totalFootprintBytes": 2**19} for sec in range(30, 60, 5)]
        rows.insert(0, {"timestamp": "2026-10-05T00:00:01Z", "totalRSSBytes": 999 * 2**20, "totalFootprintBytes": 999 * 2**20})
        with self.assertRaises(ValueError):
            analysis.recovery_window(rows, "2026-10-05T00:00:00Z")
        rows.append({**rows[-1], "timestamp": "2026-10-05T00:10:01Z"})
        result = analysis.recovery_window(rows, "2026-10-05T00:00:00Z")
        self.assertEqual(result["rssMedianMiB"], 1)
        self.assertEqual(result["footprintMedianMiB"], .5)


if __name__ == "__main__":
    unittest.main()
