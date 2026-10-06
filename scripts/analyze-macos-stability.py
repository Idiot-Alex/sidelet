#!/usr/bin/env python3
"""Summarize installed-app idle, UI-cycle and recovery evidence; no leak verdict."""
import argparse
import csv
from datetime import datetime
import json
from pathlib import Path
import statistics


def timestamp(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()


def read(path):
    return json.loads(path.read_text())


def verify_cycles(events, kind, count):
    selected = [e for e in events if e["kind"] == kind]
    if len(selected) != count or sorted(e["cycle"] for e in selected) != list(range(1, count + 1)):
        raise ValueError(f"{kind}: missing or duplicate cycles, expected {count}")
    if any(e.get("success") is not True for e in selected):
        raise ValueError(f"{kind}: unsuccessful UI cycle")
    return len(selected)


def recovery_window(rows, closed_at, duration=600):
    start = timestamp(closed_at)
    end = timestamp(rows[-1]["timestamp"])
    if end - start < duration:
        raise ValueError("Recovery does not cover the requested full duration")
    selected = [r for r in rows if duration - 30 <= timestamp(r["timestamp"]) - start <= duration]
    if len(selected) < 5:
        raise ValueError("Too few samples in the final recovery window")
    return {
        "observedSeconds": end - start,
        "windowSecondsAfterClose": [duration - 30, duration],
        "samples": len(selected),
        "rssMedianMiB": statistics.median(float(r["totalRSSBytes"]) / 2**20 for r in selected),
        "footprintMedianMiB": statistics.median(float(r["totalFootprintBytes"]) / 2**20 for r in selected),
    }


def validate_profiles(idle, interaction, idle_env, interaction_env):
    for profile in (idle, interaction):
        if profile["status"] != "complete":
            raise ValueError("Incomplete process sampling")
    if idle["elapsedSeconds"] < 600:
        raise ValueError("Idle sampling is shorter than ten minutes")
    if idle_env["binarySHA256"] != interaction_env["binarySHA256"]:
        raise ValueError("Idle and interaction used different application binaries")
    if "-memory-dir" in idle_env["rootArguments"] or "-interaction-test" in idle_env["rootArguments"]:
        raise ValueError("Idle baseline enabled extra diagnostics")


def snapshot_objects(directory, label, root_pid):
    go = read(directory / "go.json")
    native = read(directory / "native.json")
    if go["pid"] != root_pid:
        raise ValueError(f"Snapshot {label} came from a different process")
    if not native.get("enabled"):
        raise ValueError(f"Snapshot {label} has no native instrumentation")
    quick_add = native.get("quickAddBound", 0)
    declared = native.get("controllerViews")
    quick = int("quick" in declared) if declared is not None else 1
    stack_count = native["boundStates"] - quick - quick_add
    if quick_add not in (0, 1) or stack_count < 1:
        raise ValueError(f"Snapshot {label} has invalid bound window counts")
    names = ["control"] + (["quick"] if quick else []) + [f"stack-{i}" for i in range(stack_count)]
    if quick_add:
        names.append("add")
    if declared is not None:
        expected = set(names) - {"control"}
        if "control" in declared:
            expected.add("control")
        if len(declared) != len(set(declared)) or set(declared) != expected:
            raise ValueError(f"Snapshot {label} has invalid controller view names")
        names = declared
    views = {}
    for name in names:
        reply = read(directory / f"{name}.json")
        if reply.get("label") != label or reply.get("window") != name:
            raise ValueError(f"Snapshot {label} has a mismatched {name} reply")
        views[name] = reply["metrics"]
    return {
        "at": go["at"], "heapAllocKiB": go["stats"]["HeapAlloc"] / 1024,
        "heapObjects": go["stats"]["HeapObjects"], "goroutines": go["goroutines"],
        "native": native, "views": views,
    }


def normal_recovery(profile, environment, idle_environment, rows, closed_at, duration):
    if profile["status"] != "complete" or profile["elapsedSeconds"] < duration:
        raise ValueError("Incomplete normal-mode recovery sampling")
    if environment["binarySHA256"] != idle_environment["binarySHA256"]:
        raise ValueError("Normal recovery used a different application binary")
    if "-memory-dir" in environment["rootArguments"] or "-interaction-test" in environment["rootArguments"]:
        raise ValueError("Normal recovery enabled extra diagnostics")
    if environment["rootPID"] != idle_environment["rootPID"] or environment["rootStart"] != idle_environment["rootStart"]:
        raise ValueError("Normal recovery restarted the idle instance")
    return recovery_window(rows, closed_at, duration)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", type=Path, required=True)
    args = parser.parse_args()
    root = args.dir
    plan = read(root / "plan.json")
    phases = read(root / "phases.json")
    profiles, environments = {}, {}
    for kind in ("idle", "interaction"):
        matches = list(root.glob(f"macos-installed-{kind}-*.summary.json"))
        if len(matches) != 1:
            raise ValueError(f"Expected one {kind} profile, found {len(matches)}")
        profiles[kind] = read(matches[0])
        environments[kind] = read(Path(profiles[kind]["environment"]))
    validate_profiles(profiles["idle"], profiles["interaction"], environments["idle"], environments["interaction"])
    events = [json.loads(line) for line in (root / "events.jsonl").read_text().splitlines()]
    expected = {
        "card": sum(plan["interaction"]["cardBatches"]),
        "settings": sum(plan["interaction"]["settingsBatches"]),
        "temporary": plan["interaction"]["temporaryTaskCycles"],
        "snooze": plan["interaction"]["snoozeRestoreCycles"],
        "undo": plan["interaction"]["completeUndoCycles"],
    }
    if "quickAddBatches" in plan["interaction"]:
        expected["quick-add"] = sum(plan["interaction"]["quickAddBatches"])
    if "normal" in plan:
        expected["quick-add-normal"] = plan["normal"]["quickAddCycles"]
    expected.update(plan.get("workflow", {}))
    counts = {kind: verify_cycles(events, kind, count) for kind, count in expected.items()}
    with Path(profiles["interaction"]["csv"]).open() as f:
        rows = list(csv.DictReader(f))
    recovery = recovery_window(rows, phases["lastClosedAt"], plan["interaction"]["recoverySeconds"])
    objects = {}
    for label in phases["snapshots"]:
        directory = root / "memory" / label
        objects[label] = snapshot_objects(directory, label, environments["interaction"]["rootPID"])
    normal = None
    if "normal" in plan:
        matches = list(root.glob("macos-installed-normal-recovery-*.summary.json"))
        if len(matches) != 1:
            raise ValueError(f"Expected one normal recovery profile, found {len(matches)}")
        profile = read(matches[0])
        environment = read(Path(profile["environment"]))
        with Path(profile["csv"]).open() as f:
            normal_rows = list(csv.DictReader(f))
        normal = {"profile": profile, "recovery": normal_recovery(
            profile, environment, environments["idle"], normal_rows,
            phases["normalClosedAt"], plan["normal"]["recoverySeconds"])}
    report = {
        "evidenceComplete": True,
        "binarySHA256": environments["idle"]["binarySHA256"],
        "cycles": counts, "profiles": profiles, "recovery": recovery, "objects": objects,
        "normalRecovery": normal,
        "interruptedRecoveries": phases.get("interruptedRecoveries", []),
        "entryRetries": [json.loads(line) for line in (root / "retries.jsonl").read_text().splitlines()] if (root / "retries.jsonl").exists() else [],
        "excludedSnapshots": phases.get("excludedSnapshots", {}),
        "interpretation": "Evidence completeness only. RSS/footprint changes do not prove a leak or its absence; no memory acceptance budget is inferred.",
    }
    (root / "analysis.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({key: report[key] for key in ("evidenceComplete", "cycles", "recovery")}, indent=2))


if __name__ == "__main__":
    main()
