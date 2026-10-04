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
    counts = {kind: verify_cycles(events, kind, count) for kind, count in expected.items()}
    with Path(profiles["interaction"]["csv"]).open() as f:
        rows = list(csv.DictReader(f))
    recovery = recovery_window(rows, phases["lastClosedAt"], plan["interaction"]["recoverySeconds"])
    objects = {}
    for label in phases["snapshots"]:
        directory = root / "memory" / label
        go = read(directory / "go.json")
        native = read(directory / "native.json")
        views = {name: read(directory / f"{name}.json")["metrics"] for name in ("control", "quick", "stack-0")}
        if go["pid"] != environments["interaction"]["rootPID"]:
            raise ValueError(f"Snapshot {label} came from a different process")
        objects[label] = {
            "at": go["at"], "heapAllocKiB": go["stats"]["HeapAlloc"] / 1024,
            "heapObjects": go["stats"]["HeapObjects"], "goroutines": go["goroutines"],
            "native": native, "views": views,
        }
    report = {
        "evidenceComplete": True,
        "binarySHA256": environments["idle"]["binarySHA256"],
        "cycles": counts, "profiles": profiles, "recovery": recovery, "objects": objects,
        "entryRetries": [json.loads(line) for line in (root / "retries.jsonl").read_text().splitlines()] if (root / "retries.jsonl").exists() else [],
        "excludedSnapshots": phases.get("excludedSnapshots", {}),
        "interpretation": "Evidence completeness only. RSS/footprint changes do not prove a leak or its absence; no memory acceptance budget is inferred.",
    }
    (root / "analysis.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({key: report[key] for key in ("evidenceComplete", "cycles", "recovery")}, indent=2))


if __name__ == "__main__":
    main()
