#!/usr/bin/env python3
"""Check one manual session without performing UI actions or promoting P0."""
import argparse
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import re
import tempfile


def parse_app(text, offset):
    events = []
    zone = timezone(timedelta(seconds=offset))
    for line in text.splitlines():
        match = re.match(r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d+) (.*)", line)
        if not match:
            continue
        moment = datetime.strptime(match[1], "%Y/%m/%d %H:%M:%S.%f").replace(tzinfo=zone).timestamp()
        message = match[2]
        event = {"time": moment, "message": message}
        if message.startswith("keyboard request source="):
            event.update(kind="keyboard", source=message.split("=", 1)[1])
        elif message.startswith("focus captured token="):
            event.update(kind="capture", **json.loads(message.split("=", 1)[1]))
        elif message.startswith("focus restore requested"):
            restore = re.fullmatch(r"focus restore requested success=(true|false) token=(\{.*\})", message)
            if not restore:
                raise ValueError("Malformed restore record")
            event.update(kind="restore", success=restore[1] == "true", **json.loads(restore[2]))
        elif message.startswith("focus boundary="):
            focus = re.fullmatch(r"focus boundary=(\S+) (\{.*\})", message)
            if not focus:
                raise ValueError("Malformed focus record")
            event.update(kind="focus", boundary=focus[1], **json.loads(focus[2]))
        elif message.startswith("input window="):
            mode = re.fullmatch(r"input window=(\S+) mode=(\S+)", message)
            if mode:
                event.update(kind="mode", window=mode[1], mode=mode[2])
        elif message.startswith("native window="):
            native = re.fullmatch(r"native window=(\S+) (\{.*\})", message)
            if not native:
                raise ValueError("Malformed native record")
            event.update(kind="native", window=native[1], **json.loads(native[2]))
        elif message.startswith("hover window="):
            event.update(kind="hover")
        events.append(event)
    return events


def parse_fixture(text):
    records = [json.loads(line[9:]) for line in text.splitlines() if line.startswith("evidence=")]
    if any(not isinstance(record.get("unixTime"), (int, float)) for record in records):
        raise ValueError("Fixture has no wall-clock timestamps; rebuild it and start a fresh session")
    sequences = [record["sequence"] for record in records]
    if sequences != sorted(set(sequences)):
        raise ValueError("Duplicate or unordered fixture records")
    return records


def focused(record, pid):
    return (record.get("foregroundPID") == pid and record.get("fixturePID") == pid
            and record.get("appActive") is True and record.get("windowKey") is True
            and record.get("inputFocused") is True)


def contains(rect, x, y):
    return rect["x"] <= x < rect["x"] + rect["width"] and rect["y"] <= y < rect["y"] + rect["height"]


def point_kind(snapshot, x, y):
    """Use installed native rectangles, not an arbitrary click in the other app."""
    if snapshot.get("activeInput") or snapshot.get("key") or not snapshot.get("visible"):
        return None
    if not contains(snapshot["frame"], x, y):
        return None
    rectangles = [item["frame"] for item in snapshot.get("inputRouting", [])]
    if not rectangles or any(contains(rect, x, y) for rect in rectangles):
        return None
    rectangles.sort(key=lambda rect: rect["y"])
    for first, second in zip(rectangles, rectangles[1:]):
        if (first["y"] + first["height"] <= y < second["y"]
                and max(first["x"], second["x"]) <= x
                < min(first["x"] + first["width"], second["x"] + second["width"])):
            return "gap"
    return "transparent"


def analyse(session, app_text, fixture_text, manual_confirmed=False):
    if session.get("schema") != 1 or session.get("status") != "ready":
        raise ValueError("Session is not ready or has an unsupported schema")
    own, foreign = session["sideletPID"], session["fixturePID"]
    if own <= 0 or foreign <= 0 or own == foreign:
        raise ValueError("Invalid session PIDs")
    starts = re.findall(r"platform=darwin pid=(\d+)", app_text)
    ready = re.findall(r"READY pid=(\d+) sideletPID=(\d+)", fixture_text)
    if starts != [str(own)] or ready != [(str(foreign), str(own))]:
        raise ValueError("Logs contain different or multiple process instances")
    app = parse_app(app_text, session["utcOffsetSeconds"])
    fixture = parse_fixture(fixture_text)
    if any(e.get("fixturePID", foreign) != foreign or e.get("sideletPID", own) != own for e in fixture):
        raise ValueError("Fixture evidence belongs to a different instance")
    checks = {}
    def check(name, passed, failed=False):
        checks[name] = "pass" if passed else "fail" if failed else "pending"
    check("manual-input-origin-confirmed", manual_confirmed)
    check("no-fixture-keyboard-entry", not any(
        e.get("source") == "fixture-request" for e in app if e.get("kind") == "keyboard"
    ), True)
    check("no-focus-refusal", "refused keyboard focus" not in app_text, True)
    requests = [i for i, e in enumerate(app) if e.get("kind") == "keyboard" and e["source"] == "global-shortcut"]
    check("actual-carbon-keyboard-entry", bool(requests))
    if requests:
        start = requests[-1]
        end = next((i for i in range(start + 1, len(app)) if app[i].get("kind") == "keyboard"), len(app))
        # Repeated shortcuts while KeyboardActive reuse the original capture.
        # Walk back within that interaction only, stopping at a fresh capture,
        # an exit/restore, or an intervening non-global keyboard entry.
        for previous in reversed(requests[:-1]):
            if any(e.get("kind") == "capture" for e in app[start:end]):
                break
            if any(e.get("kind") in ("restore", "keyboard") or
                   (e.get("kind") == "focus" and e.get("boundary") == "exit-modes")
                   for e in app[previous + 1:start]):
                break
            start = previous
        cycle = app[start:end]
        entry_time = cycle[0]["time"]
        captures = [e for e in cycle if e.get("kind") == "capture"]
        check("captures-external-fixture", bool(captures) and captures[0]["pid"] == foreign, bool(captures))
        check("external-input-before-entry", any(
            e["type"] == "focus" and e.get("reason") == "input-change" and e["unixTime"] < entry_time
            and "before-sidelet" in e.get("input", "") and focused(e, foreign) for e in fixture
        ))
        expected = iter([("stack-0", "KeyboardActive"), ("quick", "KeyboardActive"),
                         ("quick", "Editing"), ("quick", "KeyboardActive")])
        wanted = next(expected)
        for e in cycle:
            if e.get("kind") == "mode" and (e["window"], e["mode"]) == wanted:
                wanted = next(expected, None)
                if wanted is None:
                    break
        check("edit-cancel-keyboard-sequence", wanted is None)
        editing = [e for e in cycle if e.get("kind") == "focus" and e["boundary"] == "mode-Editing"]
        check("editing-has-own-system-foreground", any(
            e.get("foregroundPID") == own and e.get("appActive") is True and e.get("keyWindow", 0) > 0
            for e in editing
        ), bool(editing))
        cancelled = [e for e in cycle if editing and e.get("kind") == "focus"
                     and e["boundary"] == "mode-KeyboardActive" and e["time"] > editing[-1]["time"]]
        check("cancel-keeps-own-system-foreground", any(
            e.get("foregroundPID") == own and e.get("appActive") is True and e.get("keyWindow", 0) > 0
            for e in cancelled
        ), bool(cancelled))
        restores = [e for e in cycle if e.get("kind") == "restore"]
        restore = restores[-1] if restores else None
        check("requests-external-restore", bool(restore and restore["success"] and restore["pid"] == foreign), bool(restore))
        if restore:
            upper = app[end]["time"] if end < len(app) else float("inf")
            resumed = [e for e in fixture if restore["time"] <= e["unixTime"] < upper
                       and e["type"] == "focus" and e.get("reason") == "input-change"
                       and "after-escape" in e.get("input", "") and focused(e, foreign)]
            first = resumed[0] if resumed else None
            returned = [e for e in fixture if first and restore["time"] <= e["unixTime"] <= first["unixTime"]
                        and e["type"] == "focus" and e.get("reason") == "window-key" and focused(e, foreign)]
            intervened = any(first and restore["time"] <= e["unixTime"] <= first["unixTime"]
                             and e["type"] in ("mouse-down", "manual-activate", "test-entry") for e in fixture)
            check("actual-return-before-resumed-input", bool(returned))
            check("resumed-input-without-refocusing", bool(first) and not intervened, bool(first) and intervened)
        else:
            check("actual-return-before-resumed-input", False)
            check("resumed-input-without-refocusing", False)
    else:
        for name in ("captures-external-fixture", "external-input-before-entry", "edit-cancel-keyboard-sequence",
                     "editing-has-own-system-foreground", "cancel-keeps-own-system-foreground", "requests-external-restore",
                     "actual-return-before-resumed-input", "resumed-input-without-refocusing"):
            check(name, False)
    stacks = [e for e in app if e.get("kind") == "native" and e.get("window") == "stack-0"]
    hover_passed = False
    for entered in [e for e in app if e.get("kind") == "hover"]:
        snapshots = [e for e in stacks if entered["time"] <= e["time"] <= entered["time"] + .5]
        for snapshot in snapshots:
            if snapshot.get("key") or snapshot.get("activeInput") or snapshot.get("forwardedEvents", 0) <= 0:
                continue
            for e in fixture:
                if (e["type"] == "focus" and e.get("reason") == "input-change"
                        and e["unixTime"] >= snapshot["time"] and "after-hover" in e.get("input", "")
                        and focused(e, foreign)):
                    interrupted = any(snapshot["time"] < other["unixTime"] <= e["unixTime"]
                                      and (other["type"] in ("mouse-down", "manual-activate", "test-entry")
                                           or other.get("foregroundPID", foreign) != foreign) for other in fixture)
                    keyboard = any(snapshot["time"] < other["time"] <= e["unixTime"]
                                   and other.get("kind") == "keyboard" for other in app)
                    hover_passed |= not interrupted and not keyboard
    check("passive-native-hover-keeps-input", hover_passed)
    hits = set()
    for e in fixture:
        if e["type"] not in ("mouse-down", "scroll") or e.get("foregroundPID") != foreign:
            continue
        hit_time = e.get("eventUnixTime", e["unixTime"])
        earlier = [snapshot for snapshot in stacks if snapshot["time"] <= hit_time]
        if not earlier:
            continue
        location = point_kind(earlier[-1], e["screenX"], e["screenY"])
        if location and (e["type"] != "scroll" or e.get("beforeY") != e.get("afterY")):
            hits.add((location, e["type"]))
    for location in ("gap", "transparent"):
        for event in ("mouse-down", "scroll"):
            check(f"{location}-{event}-reaches-underlying", (location, event) in hits)
    environment = next((e for e in fixture if e["type"] == "environment"), {})
    return {
        "scope": "macos-manual-input-session", "checks": checks,
        "status": "fail" if "fail" in checks.values() else "pass" if all(v == "pass" for v in checks.values()) else "pending",
        "inputOrigin": "user-attested-manual" if manual_confirmed else "unconfirmed",
        "sideletPID": own, "fixturePID": foreign, "sideletSHA256": session["sideletSHA256"],
        "screens": environment.get("screens", []),
        "remaining": ["Passive card interaction", "other underlying applications",
                      "ordinary Space switching", "multiple physical displays / unplug / Dock"],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("session", type=Path)
    parser.add_argument("--manual-input-confirmed", action="store_true",
                        help="user declaration of real keyboard/mouse; never inferred from trusted events")
    args = parser.parse_args()
    try:
        directory = args.session.resolve()
        session = json.loads((directory / "session.json").read_text())
        # Logs always come from this session directory, not metadata-supplied paths.
        report = analyse(session, (directory / "sidelet.log").read_text(),
                         (directory / "fixture.log").read_text(), args.manual_input_confirmed)
        with tempfile.NamedTemporaryFile(mode="w", dir=directory, prefix=".report-", delete=False) as output:
            json.dump(report, output, ensure_ascii=False, indent=2)
            output.write("\n")
        Path(output.name).replace(directory / "report.json")
    except (ValueError, KeyError, TypeError, OSError) as error:
        print(f"Cannot validate this session: {error}")
        return 1
    for name, result in report["checks"].items():
        print(f"{result.upper():7} {name}")
    print(f"Session: {report['status']}; report: {directory / 'report.json'}")
    print("This report does not promote P0; the remaining scenarios still require acceptance.")
    return {"pass": 0, "fail": 1, "pending": 2}[report["status"]]


if __name__ == "__main__":
    raise SystemExit(main())
