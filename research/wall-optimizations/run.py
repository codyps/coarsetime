#!/usr/bin/env python3
"""Serial, shuffled wall-read comparisons against the unchanged public API."""
import argparse
import hashlib
import json
import random
import re
import statistics
import subprocess
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('binary', type=Path)
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--linux', action='store_true')
parser.add_argument('--filter', default='.*')
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
binary = str(args.binary.resolve())
listing = subprocess.check_output([binary, '-test.list=^BenchmarkWallOpt'], text=True)
names = [n for n in listing.splitlines() if n.startswith('BenchmarkWallOpt') and re.search(args.filter,n)]
if not names:
    raise RuntimeError('No benchmarks found')
pattern = re.compile(r'^(Benchmark\w+)-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$', re.M)
rng = random.Random(20260905)
samples = {name: [] for name in names}
with (args.output / 'raw.txt').open('w') as log:
    for iteration in range(5):
        order = names.copy()
        rng.shuffle(order)
        for name in order:
            # Linux executes its fast-path assertion in every process. Set
            # COARSETIME_REQUIRE_VDSO=1 in the launching environment.
            cmd = [binary, '-test.run=^TestVDSO$' if args.linux else '-test.run=^$',
                   '-test.bench=^' + name + '$', '-test.benchtime=300ms',
                   '-test.count=1', '-test.benchmem']
            result = subprocess.run(cmd, capture_output=True, text=True, check=True)
            log.write(repr(cmd) + '\n' + result.stdout + result.stderr + '\n')
            log.flush()
            matches = pattern.findall(result.stdout)
            if len(matches) != 1 or matches[0][0] != name:
                raise RuntimeError('Missing benchmark result: ' + result.stdout)
            _, ns, bytes_count, allocs = matches[0]
            if int(bytes_count) or int(allocs):
                raise RuntimeError('Unexpected allocation: ' + result.stdout)
            samples[name].append(float(ns))
        print(f'Completed {iteration + 1}/5 rounds', flush=True)
summary = {name: {'median_ns': statistics.median(v), 'min_ns': min(v),
                  'max_ns': max(v), 'samples_ns': v} for name, v in samples.items()}
(args.output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
(args.output / 'binary.sha256').write_text(hashlib.sha256(args.binary.read_bytes()).hexdigest() + '\n')
for name, result in summary.items():
    print(name, result['median_ns'])
