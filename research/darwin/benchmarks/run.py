#!/usr/bin/env python3
"""Run already-built prototypes serially in reproducibly shuffled order."""
import argparse
import json
import random
import re
import statistics
import subprocess
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('binary', type=Path)
parser.add_argument('--rounds', type=int, default=5)
parser.add_argument('--benchtime', default='500ms')
parser.add_argument('--output', type=Path, default=Path(__file__).parent)
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
operations = ['Instant', 'Since', 'UnixNano', 'Now']
variants = ['Libc', 'CheckedASM', 'OnceASM', 'OnceAtomic']
names = ['BenchmarkPrototype' + op + variant for op in operations for variant in variants]
names += ['BenchmarkTimeNow', 'BenchmarkTimeSince']
rng = random.Random(20260905)
values = {name: [] for name in names}
pattern = re.compile(r'^(Benchmark\w+)-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$', re.M)
with (args.output / 'raw.txt').open('w') as log:
    for iteration in range(args.rounds):
        order = names.copy()
        rng.shuffle(order)
        for name in order:
            command = [str(args.binary.resolve()), '-test.run=^$', '-test.bench=^' + name + '$',
                       '-test.benchtime=' + args.benchtime, '-test.benchmem', '-test.count=1']
            result = subprocess.run(command, capture_output=True, text=True, check=True)
            log.write(f'Round {iteration + 1}: {command!r}\n{result.stdout}{result.stderr}\n')
            log.flush()
            matches = pattern.findall(result.stdout)
            if len(matches) != 1 or matches[0][0] != name:
                raise RuntimeError('Missing benchmark result: ' + result.stdout)
            _, ns, memory, allocations = matches[0]
            if int(memory) or int(allocations):
                raise RuntimeError('Unexpected allocation: ' + result.stdout)
            values[name].append(float(ns))
        print(f'Completed round {iteration + 1}/{args.rounds}', flush=True)
summary = {name: {'median_ns': statistics.median(samples), 'min_ns': min(samples),
                  'max_ns': max(samples), 'samples_ns': samples} for name, samples in values.items()}
(args.output / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
for op in operations:
    print(op, ' | '.join(f'{v}: {summary["BenchmarkPrototype" + op + v]["median_ns"]:.3f}' for v in variants))
for name in names[-2:]:
    print(name, summary[name]['median_ns'])
