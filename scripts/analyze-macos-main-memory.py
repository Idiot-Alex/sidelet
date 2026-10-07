#!/usr/bin/env python3
"""Validate repeated Main workflow evidence; keep vmmap categories separate from RSS."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sqlite3
import statistics

SIZE = r'\d+(?:\.\d+)?[BKMG]?'
REGION = re.compile(r'^(.+?)\s+(' + SIZE + r')\s+(' + SIZE + r')\s+(' + SIZE + r')\s+(?:' + SIZE + r')\s+(?:' + SIZE + r')\s+(?:' + SIZE + r')\s+(?:' + SIZE + r')\s+\d+')


def size_bytes(value):
    match = re.fullmatch(r'(\d+(?:\.\d+)?)([BKMG]?)', value)
    if not match:
        raise ValueError('Unrecognized vmmap size: ' + value)
    return float(match[1]) * {'': 1, 'B': 1, 'K': 1024, 'M': 1024**2, 'G': 1024**3}[match[2]]


def vmmap_categories(text):
    regions = {}
    region_table = re.split(r'^MALLOC ZONE\s', text, maxsplit=1, flags=re.MULTILINE)[0]
    for line in region_table.splitlines():
        match = REGION.match(line)
        if match:
            regions[match[1].strip()] = {'virtualBytes': size_bytes(match[2]),
                                        'residentBytes': size_bytes(match[3]),
                                        'dirtyBytes': size_bytes(match[4])}
    if 'owned unmapped (graphics)' not in regions or 'WebKit Malloc' not in regions:
        raise ValueError('Missing graphics or WebKit allocation categories; do not infer zero')
    return {'graphicsDirtyMiB': regions['owned unmapped (graphics)']['dirtyBytes'] / 2**20,
            'webkitMallocDirtyMiB': regions['WebKit Malloc']['dirtyBytes'] / 2**20,
            'jsTaggedDirtyMiB': sum(v['dirtyBytes'] for k, v in regions.items() if k.startswith(('JS VM ', 'JS JIT '))) / 2**20,
            'jsTaggedScope': 'VM/JIT region tags only; not the complete JavaScript object heap',
            'regions': regions}


def tables_digest(directory):
    with sqlite3.connect((directory / 'sidelet.sqlite3').resolve().as_uri() + '?mode=ro', uri=True) as db:
        assert db.execute('pragma integrity_check').fetchone() == ('ok',), 'SQLite integrity failure'
        assert not db.execute('pragma foreign_key_check').fetchall(), 'Foreign key failure'
        tables = {name: sorted(db.execute('select * from "' + name + '"').fetchall(), key=repr)
                  for name, in db.execute("select name from sqlite_master where type='table'")}
    return hashlib.sha256(json.dumps(tables, sort_keys=True, ensure_ascii=False).encode()).hexdigest(), len(tables['todos'])


def main_classification(directory):
    # Startup creates Control and Stack together; process launch order can race.
    # Do not treat first PID as Control. Main is the sole large graphics owner
    # in this focused scenario (small Stack and Add; no Card). Still an inference.
    candidates = []
    for path in (directory / 'vmmap').glob('*.txt'):
        text = path.read_text()
        categories = vmmap_categories(text)
        pid = re.search(r'^Process:.*\[(\d+)\]\s*$', text, re.MULTILINE)
        if not pid:
            raise ValueError('Missing vmmap process identity')
        if categories['graphicsDirtyMiB'] > 30:
            candidates.append((int(pid[1]), categories))
    if len(candidates) != 1:
        raise ValueError('Ambiguous Main attribution; need native page/PID mapping')
    return candidates[0]


def classification_probe(root):
    directory = root / 'classify'
    stages = []
    identity = members = main_pid = binary = None
    for block in (1, 2):
        stage = directory / f'block{block}'
        assert len(list((stage / 'vmmap').glob('*.txt'))) == 3
        pid, categories = main_classification(stage)
        paths = list(stage.glob('*.summary.json'))
        assert len(paths) == 1
        summary = json.loads(paths[0].read_text())
        assert summary['status'] == 'complete' and summary['samples'] == 3 and summary['elapsedSeconds'] >= 10
        assert not summary['membershipChanged']
        environment = json.loads(Path(summary['environment']).read_text())
        assert environment['warmupSeconds'] == 10 and environment['intervalSeconds'] == 5
        assert str((directory / 'data').resolve()) in environment['rootArguments']
        assert not any(flag in environment['rootArguments'] for flag in ('-memory-dir', '-interaction-test', '-trace-focus', '-trace-pointer', '-spike'))
        rows = [json.loads(line) for line in Path(summary['processSamples']).read_text().splitlines()]
        current_identity = (environment['rootPID'], environment['rootStart'])
        ids = {(p['pid'], p['start']) for p in rows[0]['processes']}
        assert len(ids) == 6
        if block == 1:
            identity, members, main_pid, binary = current_identity, ids, pid, environment['binarySHA256']
        assert current_identity == identity and members == ids and main_pid == pid and binary == environment['binarySHA256']
        native_root = next(p for p in rows[0]['processes'] if p['pid'] == identity[0])
        pair = (native_root['resourceCoalition'], native_root['jetsamCoalition'])
        assert all(pair)
        for row in rows:
            assert {(p['pid'], p['start']) for p in row['processes']} == members
            assert all((p['resourceCoalition'], p['jetsamCoalition']) == pair for p in row['processes'])
        stages.append({'block': block, 'mainPID': pid,
                       'mainFootprintMiB': statistics.median(next(p for p in row['processes'] if p['pid'] == pid)['footprintBytes'] for row in rows) / 2**20,
                       **{k: v for k, v in categories.items() if k != 'regions'}})
    return {'scope': 'separate classification-only replica; vmmap after each stage, excluded from unperturbed baseline',
            'stages': stages,
            'delta': {key: stages[1][key] - stages[0][key] for key in ('mainFootprintMiB', 'graphicsDirtyMiB', 'webkitMallocDirtyMiB', 'jsTaggedDirtyMiB')}}


def analyze(root):
    plan = json.loads((root / 'plan.json').read_text())
    assert plan['validBlocksPerRun'] == 2
    assert set(plan['validRuns']) == {'on2', 'off2'}
    reports, fingerprints, preferences = {}, set(), set()
    for label in plan['validRuns']:
        directory = root / label
        run = next(x for x in plan['runs'] if x['label'] == label)
        log = (directory / 'app.log').read_text()
        assert [int(x) for x in re.findall(r'quick-add opened source=.*?revision=(\d+)', log)] == list(range(1, 21))
        assert len(re.findall('quick-add focus restore success=true', log)) == 20
        events = [json.loads(line) for line in (directory / 'events.jsonl').read_text().splitlines()]
        assert [e['cycle'] for e in events if e['kind'] == 'cancel-only' and e['success']] == list(range(1, 21))
        assert [e['cycle'] for e in events if e['kind'] == 'settings' and e['success']] == list(range(1, 5))
        assert [e['block'] for e in events if e['kind'] == 'main-hidden'] == [1, 2]
        digest, count = tables_digest(directory / 'data')
        assert count == 13
        fingerprints.add(digest)
        preferences.add((directory / 'data/settings.json').read_bytes())
        stages = []
        root_identity = None
        members = None
        assert len(list((directory / 'vmmap').glob('*.txt'))) == 3
        main_pid, categories = main_classification(directory)
        for block in (1, 2):
            paths = list((directory / f'block{block}').glob('*.summary.json'))
            assert len(paths) == 1, 'Ambiguous stage samples'
            summary = json.loads(paths[0].read_text())
            assert summary['status'] == 'complete' and summary['samples'] == 7 and summary['elapsedSeconds'] >= 30
            assert not summary['membershipChanged']
            environment = json.loads(Path(summary['environment']).read_text())
            assert environment['warmupSeconds'] == 10 and environment['intervalSeconds'] == 5
            assert environment['binarySHA256'] == run['binarySHA256']
            args = environment['rootArguments']
            assert str((directory / 'data').resolve()) in args
            assert '-main ' in args and ('-shared-popup=false' in args) == (not run['shared'])
            assert not any(flag in args for flag in ('-memory-dir', '-interaction-test', '-trace-focus', '-trace-pointer', '-spike'))
            rows = [json.loads(line) for line in Path(summary['processSamples']).read_text().splitlines()]
            wc = sorted((p for p in rows[0]['processes'] if p['name'] == 'com.apple.WebKit.WebContent'), key=lambda p: p['start'])
            assert len(wc) == 3 and len(rows[0]['processes']) == 6
            assert any(p['pid'] == main_pid for p in wc), 'Classification PID absent from baseline'
            identity = (environment['rootPID'], environment['rootStart'])
            ids = {(p['pid'], p['start']) for p in rows[0]['processes']}
            if block == 1:
                root_identity, members = identity, ids
            assert identity == root_identity and ids == members
            native_root = next(p for p in rows[0]['processes'] if p['pid'] == identity[0])
            pair = (native_root['resourceCoalition'], native_root['jetsamCoalition'])
            assert all(pair)
            for row in rows:
                assert {(p['pid'], p['start']) for p in row['processes']} == members
                assert all((p['resourceCoalition'], p['jetsamCoalition']) == pair for p in row['processes'])
            main = [next(p for p in row['processes'] if p['pid'] == main_pid) for row in rows]
            stages.append({'block': block, 'mainFootprintMiB': statistics.median(p['footprintBytes'] for p in main) / 2**20,
                           'mainRSSMiB': statistics.median(p['residentBytes'] for p in main) / 2**20,
                           'totalFootprintMiB': summary['totalFootprintBytes']['median'] / 2**20,
                           'totalRSSMiB': summary['totalRSSBytes']['median'] / 2**20,
                           'cpuPercentOneCore': summary['weightedCPUPercentOneCore'], 'coalition': pair})
        reports[label] = {'shared': run['shared'], 'rootPID': root_identity[0], 'mainPID': main_pid,
                          'roleAttribution': 'sole WebContent with graphics dirty >30 MiB in Main/Stack/Add scenario; inference, not native page/PID mapping',
                          'stages': stages, 'mainChangeMiB': stages[1]['mainFootprintMiB'] - stages[0]['mainFootprintMiB'],
                          'classificationAfterBothStages': {k: v for k, v in categories.items() if k != 'regions'},
                          'tablesSHA256': digest}
    assert len(fingerprints) == 1 and len(preferences) == 1, 'Profiles do not match exactly'
    assert reports['on2']['stages'][0]['coalition'] != reports['off2']['stages'][0]['coalition']
    reports['scope'] = 'Focused repeated Main workflow; excludes Card, arrange and saves. No claimed optimization or leak verdict.'
    reports['dataExactlyEquivalent'] = True
    if plan.get('classificationProbe'):
        assert tables_digest(root / 'classify/data')[0] in fingerprints
        assert (root / 'classify/data/settings.json').read_bytes() in preferences
        reports['classificationProbe'] = classification_probe(root)
    return reports


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--dir', required=True, type=Path)
    args = parser.parse_args()
    result = analyze(args.dir)
    (args.dir / 'main-comparison.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))
