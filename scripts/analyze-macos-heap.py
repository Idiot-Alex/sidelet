#!/usr/bin/env python3
"""Compare two macOS heap reports; malloc classes are not a full JS heap."""
import argparse
import json
from pathlib import Path
import re


def parse_heap(text):
    total = re.search(r"^All zones: (\d+) nodes \((\d+) bytes\)$", text, re.M)
    if not total:
        raise ValueError("missing heap allocation totals")
    classes = {}
    for line in text.splitlines():
        match = re.fullmatch(r"\s*(\d+)\s+(\d+)\s+[\d.]+\s+(.+?)\s{2,}(ObjC|C\+\+|C|Swift)\s+(.*)", line)
        if match:
            count, size, name, kind, binary = match.groups()
            # heap can print several concrete allocation layouts with the
            # same public class name (e.g. NSArray). Sum those layouts.
            entry = classes.setdefault((name, kind, binary.strip()), {"count": 0, "bytes": 0})
            entry['count'] += int(count)
            entry['bytes'] += int(size)
    if not classes:
        raise ValueError("missing heap class rows")
    return {"nodes": int(total[1]), "bytes": int(total[2]), "classes": classes}


def compare(before, after):
    changes = []
    for key in before["classes"].keys() | after["classes"].keys():
        a = before["classes"].get(key, {"count": 0, "bytes": 0})
        b = after["classes"].get(key, {"count": 0, "bytes": 0})
        changes.append({"class": key[0], "type": key[1], "binary": key[2], "beforeCount": a["count"], "afterCount": b["count"],
                        "deltaCount": b["count"] - a["count"], "deltaBytes": b["bytes"] - a["bytes"]})
    return {"scope": "system heap malloc zones and recognized native classes; excludes exhaustive JavaScript object ownership",
            "beforeNodes": before["nodes"], "afterNodes": after["nodes"],
            "beforeBytes": before["bytes"], "afterBytes": after["bytes"],
            "deltaNodes": after["nodes"] - before["nodes"], "deltaBytes": after["bytes"] - before["bytes"],
            "classes": sorted(changes, key=lambda row: (-row["deltaBytes"], row["class"], row["type"], row["binary"]))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", required=True, type=Path)
    parser.add_argument("--after", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    result = compare(parse_heap(args.before.read_text()), parse_heap(args.after.read_text()))
    result["sources"] = {"before": str(args.before), "after": str(args.after)}
    args.output.write_text(json.dumps(result, indent=2) + "\n")
    print(f"malloc allocations: {result['deltaNodes']:+d} nodes, {result['deltaBytes']:+d} bytes")


if __name__ == "__main__":
    main()
