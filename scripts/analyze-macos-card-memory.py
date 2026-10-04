#!/usr/bin/env python3
"""Compare real UI card cycles and ten minutes of recovery using process samples."""
import argparse
from bisect import bisect_right
import csv
from datetime import datetime
import json
from pathlib import Path
import statistics


ROLES = ("go", "webContent", "gpu", "networking", "other", "total")


def timestamp(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()


def memory(processes, root_pid):
    result = {role: {"rssMiB": 0, "footprintMiB": 0} for role in ROLES}
    names = {"com.apple.WebKit.WebContent": "webContent",
             "com.apple.WebKit.GPU": "gpu",
             "com.apple.WebKit.Networking": "networking"}
    for process in processes:
        role = "go" if process["pid"] == root_pid else names.get(process["name"], "other")
        for field, source in (("rssMiB", "residentBytes"), ("footprintMiB", "footprintBytes")):
            value = process[source] / 2**20
            result[role][field] += value
            result["total"][field] += value
    return result


def median_window(rows, lower, upper):
    samples = [row for row in rows if lower <= row["at"] <= upper]
    if len(samples) < 10:
        raise ValueError("Fewer than ten samples in a required 30-second comparison window")
    result = {role: {field: statistics.median(row["memory"][role][field] for row in samples)
                     for field in ("rssMiB", "footprintMiB")} for role in ROLES}
    return {"samples": len(samples), "firstTimestamp": samples[0]["timestamp"],
            "lastTimestamp": samples[-1]["timestamp"], "memory": result}


def difference(after, before):
    return {role: {field: after["memory"][role][field] - before["memory"][role][field]
                   for field in ("rssMiB", "footprintMiB")} for role in ROLES}


def analyze(samples_path, events_path, phases_path):
    phases = json.loads(phases_path.read_text())
    events = [json.loads(line) for line in events_path.read_text().splitlines()]
    count = phases["requestedCycles"]
    if count != 100 or [e["cycle"] for e in events] != list(range(1, count + 1)):
        raise ValueError("Expected exactly 100 successfully recorded cycles")
    if not all(e["opened"] and e["restoredPassive"] for e in events):
        raise ValueError("At least one UI cycle failed")
    start, closed = timestamp(phases["startedAt"]), timestamp(phases["lastClosedAt"])
    cycle_times = [timestamp(e["at"]) for e in events]
    if cycle_times != sorted(cycle_times) or start > cycle_times[0] or closed != cycle_times[-1]:
        raise ValueError("UI phase timestamps are inconsistent")
    rows = []
    for line in samples_path.read_text().splitlines():
        sample = json.loads(line)
        roots = [p for p in sample["processes"] if p["pid"] == phases["rootPID"]]
        if len(roots) != 1 or roots[0]["name"] not in {"sidelet", "sidelet-spike"}:
            raise ValueError("Process sample does not contain the expected Sidelet root")
        rows.append({"at": timestamp(sample["timestamp"]), "timestamp": sample["timestamp"],
                     "memory": memory(sample["processes"], phases["rootPID"]),
                     "identities": frozenset((p["pid"], p["start"]) for p in sample["processes"])})
    if not rows or rows[0]["at"] > start - 30 or rows[-1]["at"] < closed + 600:
        raise ValueError("Need a 30-second baseline and full ten-minute post-close coverage")
    if [row["at"] for row in rows] != sorted(row["at"] for row in rows):
        raise ValueError("Process samples are not chronological")
    baseline = median_window(rows, start - 30, start)
    after_close = median_window(rows, closed, closed + 30)
    after_ten = median_window(rows, closed + 570, closed + 600)
    checkpoints = []
    for cycle in (20, 40, 60, 80, 100):
        at = cycle_times[cycle - 1]
        nearest = min(rows, key=lambda row: abs(row["at"] - at))
        if abs(nearest["at"] - at) > 2:
            raise ValueError("No process sample near a cycle checkpoint")
        checkpoints.append({"cycle": cycle, "timestamp": nearest["timestamp"],
                            "distanceFromUICloseSeconds": nearest["at"] - at,
                            "memory": nearest["memory"]})
    summary = {
        "cycles": count, "rootPID": phases["rootPID"],
        "activeSeconds": closed - start, "postCloseCoverageSeconds": rows[-1]["at"] - closed,
        "membershipChanged": any(row["identities"] != rows[0]["identities"] for row in rows),
        "processCountRange": [min(len(row["identities"]) for row in rows),
                              max(len(row["identities"]) for row in rows)],
        "baseline": baseline, "first30SecondsAfterClose": after_close,
        "last30SecondsOfTenMinuteRecovery": after_ten,
        "changeBaselineToClosed": difference(after_close, baseline),
        "changeBaselineToRecovered": difference(after_ten, baseline),
        "changeDuringRecovery": difference(after_ten, after_close),
        "cycleCheckpoints": checkpoints,
        "sources": {"samples": str(samples_path), "events": str(events_path), "phases": str(phases_path)},
        "convention": "30-second windows report medians; cycle checkpoints are nearest samples and may include UI/animation transients. WebContent is aggregated across its members. RSS sums may count shared pages. Retained memory is not proof of a leak.",
    }
    series = []
    for row in rows:
        phase = "baseline" if row["at"] < start else "active" if row["at"] <= closed else "recovery"
        item = {"timestamp": row["timestamp"], "secondsFromFirstCycle": row["at"] - start,
                "phase": phase, "completedCycles": bisect_right(cycle_times, row["at"])}
        for role in ROLES:
            for field in ("rssMiB", "footprintMiB"):
                item[role + field[0].upper() + field[1:]] = row["memory"][role][field]
        series.append(item)
    return summary, series


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samples", type=Path, required=True)
    parser.add_argument("--events", type=Path, required=True)
    parser.add_argument("--phases", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="output filename prefix")
    args = parser.parse_args()
    summary, series = analyze(args.samples, args.events, args.phases)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.with_suffix(".analysis.json").write_text(json.dumps(summary, indent=2) + "\n")
    with args.output.with_suffix(".series.csv").open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=list(series[0]))
        writer.writeheader()
        writer.writerows(series)
    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
