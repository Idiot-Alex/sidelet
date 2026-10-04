#!/usr/bin/env python3
"""Verify evidence from the interactive native receiving panel fixture."""
import argparse
import json
from pathlib import Path
import re

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument("log",type=Path)
parser.add_argument("--fixture-pid",type=int,required=True)
args=parser.parse_args()
pattern=re.compile(r"web=(\{.*\}) forwarded=(\d+) renderKey=(\d) helperKey=(\d) foregroundPID=(\d+) clicks=(\d+)")
events=[]
for line in args.log.read_text().splitlines():
    match=pattern.fullmatch(line)
    if match:
        event=json.loads(match[1])
        event.update(forwarded=int(match[2]),renderKey=int(match[3]),helperKey=int(match[4]),foregroundPID=int(match[5]))
        events.append(event)
clicks=[e for e in events if e["type"]=="click"]
doubles=[e for e in events if e["type"]=="dblclick"]
checks={
    "first native click reaches WKWebView at the collapsed tag": bool(clicks) and (clicks[0]["x"],clicks[0]["y"],clicks[0]["detail"])==(290,34,1),
    "native receiving panel forwarded mouse down and up": bool(clicks) and clicks[0]["forwarded"]>=2,
    "expanded tag uses updated native and web coordinates": bool(doubles) and (doubles[0]["x"],doubles[0]["y"])==(250,34),
    "WKWebView receives double click with detail 2": bool(doubles) and doubles[0]["detail"]==2 and doubles[0]["forwarded"]>=6,
    "web pointer and click events are trusted": bool(events) and all(e["trusted"] for e in events),
    "render and receiving panels never take keyboard focus": bool(events) and all(e["renderKey"]==0 and e["helperKey"]==0 for e in events),
    "external foreground application stays unchanged": bool(events) and len({e["foregroundPID"] for e in events})==1 and events[0]["foregroundPID"]!=args.fixture_pid,
}
for name,passed in checks.items(): print(f"{'PASS' if passed else 'FAIL'} {name}")
print(f"Native → WKWebView assertions: {sum(not p for p in checks.values())} failures")
raise SystemExit(0 if all(checks.values()) else 1)
