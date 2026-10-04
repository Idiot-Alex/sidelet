#!/usr/bin/env python3
"""Offline checks against false acceptance; no UI or synthesized input."""
from copy import deepcopy
from datetime import datetime, timezone
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("acceptance", Path(__file__).with_name("check-macos-acceptance.py"))
acceptance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(acceptance)

BASE = 1700000000
SESSION = {"schema": 1, "status": "ready", "sideletPID": 2, "fixturePID": 1,
           "utcOffsetSeconds": 0, "sideletSHA256": "synthetic-test-data"}


def app_line(seconds, message):
    moment = datetime.fromtimestamp(BASE + seconds, timezone.utc)
    return moment.strftime("%Y/%m/%d %H:%M:%S.%f") + " " + message


def sample():
    snapshot = {"frame": {"x": 0, "y": 0, "width": 100, "height": 200},
                "visible": True, "key": False, "activeInput": False, "forwardedEvents": 2,
                "inputRouting": [{"frame": {"x": 80, "y": y, "width": 20, "height": 20}} for y in (10, 40)]}
    own_focus = {"foregroundPID": 2, "appActive": True, "keyWindow": 12}
    messages = [
        (0, "Sidelet platform=darwin pid=2"),
        (.5, "native window=stack-0 " + json.dumps(snapshot)),
        (2, "keyboard request source=global-shortcut"),
        (2.1, 'focus captured token={"pid":1,"ownWindow":0}'),
        (2.2, "input window=stack-0 mode=KeyboardActive"),
        (3, "input window=quick mode=KeyboardActive"),
        (4, "input window=quick mode=Editing"),
        (4.01, "focus boundary=mode-Editing " + json.dumps(own_focus)),
        (5, "input window=quick mode=KeyboardActive"),
        (5.01, "focus boundary=mode-KeyboardActive " + json.dumps(own_focus)),
        (6, 'focus restore requested success=true token={"pid":1,"ownWindow":0}'),
        (6.05, "native window=stack-0 " + json.dumps(snapshot)),
        (11, "hover window=stack-0 metric={}"),
        (11.05, "native window=stack-0 " + json.dumps(snapshot)),
    ]
    events = []
    def record(seconds, type="focus", **values):
        event = {"unixTime": BASE + seconds, "sequence": len(events) + 1, "type": type,
                 "fixturePID": 1, "sideletPID": 2, "foregroundPID": 1, "appActive": True,
                 "windowKey": True, "inputFocused": True}
        event.update(values)
        events.append(event)
    record(.8, "environment", manualSession=True, screens=[{"displayID": 1}])
    record(1, reason="input-change", input="before-sidelet")
    record(2.3, reason="workspace-activation", foregroundPID=2)
    record(6.1, reason="window-key")
    record(6.2, reason="input-change", input="before-sidelet after-escape")
    record(11.3, reason="input-change", input="before-sidelet after-escape after-hover")
    record(12, "mouse-down", screenX=90, screenY=35)
    record(13, "scroll", screenX=90, screenY=35, beforeY=0, afterY=30)
    record(14, "mouse-down", screenX=40, screenY=15)
    record(15, "scroll", screenX=40, screenY=15, beforeY=30, afterY=60)
    return messages, events


def run(messages, events, confirmed=True, session=None):
    app = "\n".join(app_line(time, message) for time, message in messages)
    fixture = "READY pid=1 sideletPID=2\n" + "\n".join("evidence=" + json.dumps(event) for event in events)
    return acceptance.analyse(session or SESSION, app, fixture, confirmed)


class AcceptanceTests(unittest.TestCase):
    def test_complete_session_does_not_remove_other_acceptance(self):
        report = run(*sample())
        self.assertEqual(report["status"], "pass")
        self.assertIn("multiple physical displays / unplug / Dock", report["remaining"])

    def test_registration_and_fixture_entry_cannot_count_as_global_key(self):
        messages, events = sample()
        messages = [(time, message.replace("source=global-shortcut", "source=fixture-request")) for time, message in messages]
        report = run(messages, events)
        self.assertEqual(report["checks"]["actual-carbon-keyboard-entry"], "pending")
        self.assertEqual(report["status"], "fail")

    def test_missing_manual_origin_never_passes(self):
        self.assertEqual(run(*sample(), confirmed=False)["status"], "pending")

    def test_registration_only_stays_pending(self):
        messages, events = sample()
        report = run(messages[:2] + [(1, "global-shortcut registered shortcut=Ctrl+Option+T exclusive=true")], events[:2])
        self.assertEqual(report["status"], "pending")
        self.assertEqual(report["checks"]["actual-carbon-keyboard-entry"], "pending")

    def test_repeated_global_shortcuts_reuse_same_interaction_capture(self):
        messages, events = sample()
        messages += [(2.4, "keyboard request source=global-shortcut"),
                     (2.5, "keyboard request source=global-shortcut")]
        report = run(sorted(messages), events)
        self.assertEqual(report["status"], "pass")

    def test_new_global_entry_after_exit_cannot_reuse_previous_cycle(self):
        messages, events = sample()
        messages += [(16, 'focus boundary=exit-modes {"foregroundPID":1,"appActive":false,"keyWindow":0}'),
                     (17, "keyboard request source=global-shortcut")]
        report = run(sorted(messages), events)
        for key in ("captures-external-fixture", "edit-cancel-keyboard-sequence",
                    "requests-external-restore"):
            self.assertEqual(report["checks"][key], "pending")

    def test_fresh_capture_is_not_replaced_by_previous_valid_capture(self):
        messages, events = sample()
        messages += [(17, "keyboard request source=global-shortcut"),
                     (17.1, 'focus captured token={"pid":3,"ownWindow":0}')]
        report = run(sorted(messages), events)
        self.assertEqual(report["checks"]["captures-external-fixture"], "fail")
        self.assertEqual(report["checks"]["edit-cancel-keyboard-sequence"], "pending")

    def test_non_global_entry_separates_global_keyboard_cycles(self):
        messages, events = sample()
        messages += [(2.4, "keyboard request source=tray-menu"),
                     (2.5, "keyboard request source=global-shortcut")]
        report = run(sorted(messages), events)
        self.assertEqual(report["checks"]["captures-external-fixture"], "pending")

    def test_refocusing_after_escape_is_a_failure(self):
        messages, events = sample()
        intervened = deepcopy(events[3])
        intervened.update(type="mouse-down", unixTime=BASE+6.15, screenX=20, screenY=80)
        events.insert(4, intervened)
        for index, event in enumerate(events):
            event["sequence"] = index + 1
        report = run(messages, events)
        self.assertEqual(report["checks"]["resumed-input-without-refocusing"], "fail")

    def test_stale_key_window_does_not_prove_foreground(self):
        messages, events = sample()
        events[3]["foregroundPID"] = 2
        report = run(messages, events)
        self.assertEqual(report["checks"]["actual-return-before-resumed-input"], "pending")

    def test_clicks_outside_overlay_and_stationary_scroll_do_not_count(self):
        messages, events = sample()
        for event in events:
            if event["type"] in ("mouse-down", "scroll"):
                event["screenX"] = 1000
                event["afterY"] = event.get("beforeY")
        report = run(messages, events)
        for key in ("gap-mouse-down-reaches-underlying", "transparent-scroll-reaches-underlying"):
            self.assertEqual(report["checks"][key], "pending")

    def test_mixed_instances_and_old_timestamps_are_rejected(self):
        messages, events = sample()
        with self.assertRaises(ValueError):
            run(messages, events, session={**SESSION, "sideletPID": 3})
        del events[0]["unixTime"]
        with self.assertRaises(ValueError):
            run(messages, events)

    def test_delayed_scroll_uses_geometry_at_delivery(self):
        messages, events = sample()
        scroll = next(event for event in events if event["type"] == "scroll")
        scroll.update(eventUnixTime=scroll["unixTime"], unixTime=BASE+13.5)
        covered = {"frame": {"x": 0, "y": 0, "width": 100, "height": 200},
                   "visible": True, "key": False, "activeInput": False,
                   "inputRouting": [{"frame": {"x": 0, "y": 0, "width": 100, "height": 200}}]}
        messages.append((13.2, "native window=stack-0 " + json.dumps(covered)))
        report = run(sorted(messages), events)
        self.assertEqual(report["checks"]["gap-scroll-reaches-underlying"], "pass")


if __name__ == "__main__":
    unittest.main()
