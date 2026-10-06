#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path
import tempfile
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

    def test_quick_add_snapshot_requires_its_own_matching_reply(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "go.json").write_text(json.dumps({"pid": 42, "at": "now", "stats": {"HeapAlloc": 1024, "HeapObjects": 10}, "goroutines": 15}))
            (root / "native.json").write_text(json.dumps({"enabled": True, "boundStates": 3, "quickAddBound": 1}))
            for name in ("control", "quick", "stack-0"):
                (root / f"{name}.json").write_text(json.dumps({"label": "batch", "window": name, "metrics": {}}))
            with self.assertRaises(FileNotFoundError):
                analysis.snapshot_objects(root, "batch", 42)
            (root / "add.json").write_text(json.dumps({"label": "other", "window": "add", "metrics": {}}))
            with self.assertRaises(ValueError):
                analysis.snapshot_objects(root, "batch", 42)
            (root / "add.json").write_text(json.dumps({"label": "batch", "window": "add", "metrics": {}}))
            result = analysis.snapshot_objects(root, "batch", 42)
            self.assertEqual(set(result["views"]), {"control", "quick", "stack-0", "add"})
            with self.assertRaises(ValueError):
                analysis.snapshot_objects(root, "batch", 99)

    def test_cold_snapshot_keeps_historical_three_view_format(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "go.json").write_text(json.dumps({"pid": 42, "at": "now", "stats": {"HeapAlloc": 1024, "HeapObjects": 10}, "goroutines": 15}))
            (root / "native.json").write_text(json.dumps({"enabled": True, "boundStates": 2}))
            for name in ("control", "quick", "stack-0"):
                (root / f"{name}.json").write_text(json.dumps({"label": "cold", "window": name, "metrics": {}}))
            self.assertNotIn("add", analysis.snapshot_objects(root, "cold", 42)["views"])

    def test_lazy_control_and_recycled_add_snapshots(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "go.json").write_text(json.dumps({"pid": 42, "at": "now", "stats": {"HeapAlloc": 1024, "HeapObjects": 10}, "goroutines": 15}))
            for name in ("quick", "stack-0", "add", "control"):
                (root / f"{name}.json").write_text(json.dumps({"label": "batch", "window": name, "metrics": {}}))
            for names in (["stack-0"], ["control", "stack-0"], ["stack-0", "add"], ["quick", "stack-0"], ["quick", "stack-0", "add"], ["control", "quick", "stack-0"]):
                native = {"enabled": True, "boundStates": 1 + int("quick" in names) + int("add" in names), "quickAddBound": int("add" in names), "controllerViews": names}
                (root / "native.json").write_text(json.dumps(native))
                self.assertEqual(set(analysis.snapshot_objects(root, "batch", 42)["views"]), set(names))
            native["controllerViews"] = ["control", "quick"]
            (root / "native.json").write_text(json.dumps(native))
            with self.assertRaises(ValueError):
                analysis.snapshot_objects(root, "batch", 42)

    def test_normal_recovery_rejects_diagnostics_and_restart(self):
        p = {"status": "complete", "elapsedSeconds": 600}
        e = {"binarySHA256": "a", "rootArguments": "sidelet -data-dir test", "rootPID": 42, "rootStart": 100}
        rows = [{"timestamp": f"2026-10-05T00:09:{sec:02d}Z", "totalRSSBytes": 2**20, "totalFootprintBytes": 2**19} for sec in range(30, 60, 5)]
        rows.append({**rows[-1], "timestamp": "2026-10-05T00:10:01Z"})
        self.assertEqual(analysis.normal_recovery(p, e, e, rows, "2026-10-05T00:00:00Z", 600)["rssMedianMiB"], 1)
        for bad_profile, bad_environment in [
            ({**p, "elapsedSeconds": 599}, e),
            (p, {**e, "rootArguments": "sidelet -interaction-test"}),
            (p, {**e, "rootStart": 101}),
            (p, {**e, "binarySHA256": "b"}),
        ]:
            with self.subTest(environment=bad_environment):
                with self.assertRaises(ValueError):
                    analysis.normal_recovery(bad_profile, bad_environment, e, rows, "2026-10-05T00:00:00Z", 600)


if __name__ == "__main__":
    unittest.main()
