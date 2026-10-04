"""Offline checks for phase boundaries and integrity of memory conclusions."""
from datetime import datetime, timedelta, timezone
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location(
    "card_memory_analysis", Path(__file__).with_name("analyze-macos-card-memory.py"))
analysis = importlib.util.module_from_spec(spec)
spec.loader.exec_module(analysis)


class RecoveryAnalysisTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        self.origin = datetime(2026, 10, 4, tzinfo=timezone.utc)
        self.phases = {"rootPID": 1, "requestedCycles": 100,
                       "startedAt": self.stamp(0), "lastClosedAt": self.stamp(100)}
        self.events = [{"cycle": i, "at": self.stamp(i), "opened": True,
                        "restoredPassive": True} for i in range(1, 101)]
        self.samples = []
        for second in range(-60, 701):
            before, recovered = second < 0, second > 670
            rss = [10 if before or recovered else 11,
                   8 if before else 9 if recovered else 14,
                   2 if before or recovered else 50 if second <= 100 else 5, 1]
            processes = [{"pid": pid, "start": 1, "name": name,
                          "residentBytes": mib * 2**20, "footprintBytes": mib * 2**19}
                         for pid, name, mib in zip(range(1, 5),
                                                  ["sidelet-spike", "com.apple.WebKit.WebContent",
                                                   "com.apple.WebKit.GPU", "com.apple.WebKit.Networking"], rss)]
            self.samples.append({"timestamp": self.stamp(second), "processes": processes})

    def stamp(self, second):
        return (self.origin + timedelta(seconds=second)).isoformat()

    def run_analysis(self):
        for name, records in (("samples", self.samples), ("events", self.events)):
            (self.path / name).write_text("\n".join(json.dumps(x) for x in records))
        (self.path / "phases").write_text(json.dumps(self.phases))
        return analysis.analyze(self.path / "samples", self.path / "events", self.path / "phases")

    def test_recovery_medians_exclude_interaction_spikes(self):
        summary, series = self.run_analysis()
        self.assertEqual(summary["baseline"]["memory"]["total"]["rssMiB"], 21)
        self.assertEqual(summary["first30SecondsAfterClose"]["memory"]["total"]["rssMiB"], 31)
        self.assertEqual(summary["last30SecondsOfTenMinuteRecovery"]["memory"]["total"]["rssMiB"], 22)
        self.assertEqual(summary["changeBaselineToRecovered"]["webContent"]["rssMiB"], 1)
        self.assertEqual(summary["changeBaselineToRecovered"]["go"]["rssMiB"], 0)
        self.assertEqual(summary["changeDuringRecovery"]["total"]["rssMiB"], -9)
        self.assertEqual(summary["postCloseCoverageSeconds"], 600)
        self.assertEqual(series[-1]["completedCycles"], 100)

    def test_short_recovery_cannot_claim_ten_minutes(self):
        self.samples.pop()
        with self.assertRaisesRegex(ValueError, "full ten-minute"):
            self.run_analysis()

    def test_pid_reuse_is_visible_even_with_constant_process_count(self):
        self.samples[400]["processes"][2]["start"] = 2
        summary, _ = self.run_analysis()
        self.assertTrue(summary["membershipChanged"])
        self.assertEqual(summary["processCountRange"], [4, 4])

    def test_missing_or_failed_ui_cycle_is_rejected(self):
        self.events[45]["restoredPassive"] = False
        with self.assertRaisesRegex(ValueError, "UI cycle failed"):
            self.run_analysis()
        self.events.pop()
        with self.assertRaisesRegex(ValueError, "exactly 100"):
            self.run_analysis()


if __name__ == "__main__":
    unittest.main()
