#!/usr/bin/env python3
"""Run the split-wall-clock experiment without concurrent build jobs."""
import argparse
import json
import random
import re
import statistics
import subprocess
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('binary', type=Path)
args = parser.parse_args()
output = Path(__file__).parent
names = ['CurrentNow', 'CurrentUnixNano', 'FixedFresh', 'FixedAged',
         'RebaseFresh', 'RebaseAged', 'AddFresh', 'AddAged', 'RebaseBoundary']
names = ['BenchmarkSplit' + n for n in names]
pattern = re.compile(r'^(Benchmark\w+)-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$', re.M)
samples = {}
rng = random.Random(20260905)
with (output / 'raw.txt').open('w') as log:
    def run(name, duration):
        command = [str(args.binary.resolve()), '-test.run=^$', '-test.bench=^' + name + '$',
                   '-test.benchtime=' + duration, '-test.benchmem', '-test.count=1']
        result = subprocess.run(command, capture_output=True, text=True, check=True)
        log.write(f'{command!r}\n{result.stdout}{result.stderr}\n')
        log.flush()
        found = pattern.findall(result.stdout)
        if len(found) != 1 or found[0][0] != name:
            raise RuntimeError('Missing benchmark result: ' + result.stdout)
        _, ns, byte_count, allocation_count = found[0]
        samples.setdefault(name, []).append({'ns': float(ns), 'bytes': int(byte_count), 'allocs': int(allocation_count)})
    for i in range(5):
        order = names.copy()
        rng.shuffle(order)
        for name in order:
            run(name, '200ms')
        print(f'Short round {i + 1}/5 complete', flush=True)
    for i in range(3):
        run('BenchmarkSplitRebaseSteady', '3s')
        print(f'Long round {i + 1}/3 complete', flush=True)
summary = {name: {'median_ns': statistics.median(v['ns'] for v in values),
                  'min_ns': min(v['ns'] for v in values), 'max_ns': max(v['ns'] for v in values),
                  'samples': values} for name, values in samples.items()}
(output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
for name, result in sorted(summary.items()):
    print(name, result['median_ns'])
