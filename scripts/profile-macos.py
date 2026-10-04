#!/usr/bin/env python3
"""Read-only Sidelet + its WebKit coalition sampling. Requires a GUI login session."""
import argparse
import csv
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import statistics
import subprocess
import time

ROOT = Path(__file__).resolve().parent.parent


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def snapshot(helper, pid, identity=None):
    processes = json.loads(command(str(helper), str(pid)))
    root = next((p for p in processes if p["pid"] == pid), None)
    if root is None or root["name"] not in {"sidelet", "sidelet-spike"}:
        raise RuntimeError("Root Sidelet process is absent; sampling is incomplete")
    if identity is not None and root["start"] != identity:
        raise RuntimeError("Root PID was reused; sampling is incomplete")
    coalition = (root["resourceCoalition"], root["jetsamCoalition"])
    if not all(coalition):
        raise RuntimeError("No valid coalition IDs; refusing ambiguous WebKit attribution")
    members = [p for p in processes if (p["resourceCoalition"], p["jetsamCoalition"]) == coalition]
    return root, members


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pid", required=True, type=int)
    parser.add_argument("--duration", type=float, default=600)
    parser.add_argument("--interval", type=float, default=5)
    parser.add_argument("--warmup", type=float, default=30)
    parser.add_argument("--label", default="idle-one-stack")
    parser.add_argument("--finish-file", type=Path,
                        help="finish normally after a controller creates this file; duration remains the safety limit")
    parser.add_argument("--output", type=Path, default=ROOT / "build/results")
    args = parser.parse_args()
    if platform.system() != "Darwin" or args.pid <= 0 or args.duration <= 0 or args.interval <= 0 or args.warmup < 0:
        parser.error("macOS, a positive PID, duration and interval are required")
    if args.finish_file and args.finish_file.exists():
        parser.error("finish-file already exists; use a new path for this run")
    args.output.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    prefix = args.output / f"macos-{args.label}-{stamp}"
    helper = args.output / "macos-process-snapshot"
    subprocess.run(["clang", "-Wall", "-Wextra", "-Werror", "-mmacosx-version-min=13.0",
                    str(ROOT / "scripts/macos-process-snapshot.c"), "-o", str(helper)], check=True)
    root, members = snapshot(helper, args.pid)
    binary = Path(root["executable"])
    environment = {
        "collectedAt": datetime.now(timezone.utc).isoformat(), "label": args.label,
        "os": command("sw_vers"), "architecture": platform.machine(),
        "cpu": command("sysctl", "-n", "machdep.cpu.brand_string"), "logicalCores": os.cpu_count(),
        "memoryBytes": int(command("sysctl", "-n", "hw.memsize")),
        "rootPID": args.pid, "rootStart": root["start"],
        "rootArguments": command("ps", "-p", str(args.pid), "-o", "args="),
        "binarySHA256": hashlib.sha256(binary.read_bytes()).hexdigest(),
        "durationRequestedSeconds": args.duration, "intervalSeconds": args.interval,
        "warmupSeconds": args.warmup,
        "finishFile": str(args.finish_file) if args.finish_file else None,
        "attribution": "root + WebKit sharing BOTH resource and jetsam coalition IDs; XNU private diagnostic ABI",
        "initialMembers": members,
        "cpuConvention": "CPU deltas in Mach ticks converted with mach_timebase_info; 100% = one busy core. Also report divided by logical core count.",
        "memoryConvention": "residentBytes = RSS; footprintBytes = ri_phys_footprint, includes charged/compressed memory. Sums can include shared pages and are not Windows Private Bytes.",
        "timing": "monotonic wall clock, first sample has no CPU delta; sleep/suspend extends wall time",
    }
    prefix.with_suffix(".environment.json").write_text(json.dumps(environment, indent=2)+"\n")
    fields = ["timestamp", "elapsedSeconds", "processCount", "webContentCount", "memberPIDs",
              "rootRSSBytes", "webkitRSSBytes", "totalRSSBytes", "rootFootprintBytes",
              "webkitFootprintBytes", "totalFootprintBytes", "rootCPUPercentOneCore",
              "webkitCPUPercentOneCore", "totalCPUPercentOneCore", "totalCPUPercentNormalized", "newMembers"]
    rows, previous, previous_at = [], {}, None
    status, failure, completion_reason = "complete", None, "duration"
    print(f"Warmup {args.warmup}s; monitoring root PID {args.pid}",flush=True)
    time.sleep(args.warmup)
    start = time.monotonic()
    try:
        with prefix.with_suffix(".csv").open("w", newline="") as f, \
                prefix.with_suffix(".processes.jsonl").open("w") as process_file:
            writer = csv.DictWriter(f, fieldnames=fields)
            writer.writeheader()
            while True:
                root, members = snapshot(helper, args.pid, environment["rootStart"])
                at = time.monotonic()
                webkit = [p for p in members if p["pid"] != args.pid]
                cpu_root, cpu_webkit, new = 0, 0, []
                for p in members:
                    key = (p["pid"], p["start"])
                    if key in previous:
                        delta = max(0, p["cpuNanoseconds"]-previous[key])
                    else:
                        delta = 0
                        if previous_at is not None:
                            new.append(p["pid"])
                    if p["pid"] == args.pid:
                        cpu_root += delta
                    else:
                        cpu_webkit += delta
                dt = at-previous_at if previous_at is not None else None
                root_percent = 100*cpu_root/1e9/dt if dt else None
                webkit_percent = 100*cpu_webkit/1e9/dt if dt else None
                total_percent = root_percent+webkit_percent if dt else None
                row = {
                    "timestamp": datetime.now(timezone.utc).isoformat(), "elapsedSeconds": round(at-start, 4),
                    "processCount": len(members), "webContentCount": sum(p["name"] == "com.apple.WebKit.WebContent" for p in webkit),
                    "memberPIDs": ";".join(str(p["pid"]) for p in members),
                    "rootRSSBytes": root["residentBytes"], "webkitRSSBytes": sum(p["residentBytes"] for p in webkit),
                    "rootFootprintBytes": root["footprintBytes"], "webkitFootprintBytes": sum(p["footprintBytes"] for p in webkit),
                    "rootCPUPercentOneCore": root_percent, "webkitCPUPercentOneCore": webkit_percent,
                    "totalCPUPercentOneCore": total_percent,
                    "totalCPUPercentNormalized": total_percent/environment["logicalCores"] if dt else None,
                    "newMembers": ";".join(map(str,new)),
                }
                row["totalRSSBytes"] = row["rootRSSBytes"]+row["webkitRSSBytes"]
                row["totalFootprintBytes"] = row["rootFootprintBytes"]+row["webkitFootprintBytes"]
                writer.writerow(row); f.flush(); rows.append(row)
                process_file.write(json.dumps({"timestamp": row["timestamp"],
                                               "elapsedSeconds": row["elapsedSeconds"],
                                               "processes": members})+"\n")
                process_file.flush()
                previous = {(p["pid"],p["start"]): p["cpuNanoseconds"] for p in members}
                previous_at = at
                if len(rows) == 1 or int(row["elapsedSeconds"])//60 != int(rows[-2]["elapsedSeconds"])//60:
                    print(f"{row['elapsedSeconds']:.0f}s processes={len(members)} RSS={row['totalRSSBytes']/2**20:.1f}MiB footprint={row['totalFootprintBytes']/2**20:.1f}MiB CPU={total_percent}", flush=True)
                remaining = args.duration-(at-start)
                if args.finish_file and args.finish_file.exists():
                    completion_reason = "finish-file"
                    break
                if remaining <= 0:
                    break
                time.sleep(min(args.interval,remaining))
    except (Exception, KeyboardInterrupt) as exc:
        status, failure = "incomplete", str(exc) or "interrupted"
        completion_reason = "interrupted"
    summary = {"status": status, "failure": failure, "label": args.label,
               "samples": len(rows), "elapsedSeconds": rows[-1]["elapsedSeconds"] if rows else 0,
               "processCountRange": [min(r["processCount"] for r in rows), max(r["processCount"] for r in rows)] if rows else [],
               "completionReason": completion_reason,
               "csv": str(prefix.with_suffix(".csv")), "environment": str(prefix.with_suffix(".environment.json")),
               "processSamples": str(prefix.with_suffix(".processes.jsonl"))}
    for field in ["rootRSSBytes", "webkitRSSBytes", "totalRSSBytes", "rootFootprintBytes",
                  "webkitFootprintBytes", "totalFootprintBytes", "totalCPUPercentOneCore", "totalCPUPercentNormalized"]:
        values = [r[field] for r in rows if r[field] is not None]
        if values:
            summary[field] = {"first": values[0], "last": values[-1], "min": min(values),
                              "max": max(values), "median": statistics.median(values)}
    valid = rows[1:]
    if valid:
        elapsed = rows[-1]["elapsedSeconds"]-rows[0]["elapsedSeconds"]
        total_cpu = sum(r["totalCPUPercentOneCore"]*(r["elapsedSeconds"]-prev["elapsedSeconds"]) for prev,r in zip(rows,valid))
        summary["weightedCPUPercentOneCore"] = total_cpu/elapsed
        summary["weightedCPUPercentNormalized"] = total_cpu/elapsed/environment["logicalCores"]
        summary["membershipChanged"] = any(r["memberPIDs"] != rows[0]["memberPIDs"] for r in rows)
    prefix.with_suffix(".summary.json").write_text(json.dumps(summary,indent=2)+"\n")
    print(json.dumps(summary,indent=2),flush=True)
    if status != "complete":
        raise SystemExit(1)


if __name__ == "__main__":
    main()
