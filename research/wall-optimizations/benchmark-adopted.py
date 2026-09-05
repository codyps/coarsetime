"""Compare public APIs with pre-adoption binaries; run platforms serially."""
import pathlib, random, re, statistics, subprocess
root = pathlib.Path(__file__).resolve().parent
rng = random.Random(72)
for platform in ('darwin', 'linux'):
    readings = {}
    output = []
    for trial in range(5):
        variants = ['baseline', 'adopted']
        rng.shuffle(variants)
        for variant in variants:
            binary = f'/tmp/coarsetime-{variant}-{platform}.test'
            cmd = [binary]
            if platform == 'linux':
                cmd = ['docker', 'run', '--rm', '--network', 'none', '-e',
                       'COARSETIME_REQUIRE_VDSO=1', '-v', f'{binary}:/test:ro',
                       'python:3.12-slim', '/test']
            cmd += ['-test.run=TestVDSO', '-test.bench=^(BenchmarkNow|BenchmarkUnixNano|BenchmarkInstantTime)$',
                    '-test.benchtime=300ms', '-test.benchmem']
            result = subprocess.run(cmd, check=True, capture_output=True, text=True).stdout
            output.append(f'{variant} trial {trial+1}\n{result}')
            for name, ns, size, allocs in re.findall(r'(Benchmark\w+)-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op', result):
                assert size == allocs == '0'
                readings.setdefault((variant, name), []).append(float(ns))
    (root / f'adopted-{platform}.txt').write_text('\n'.join(line.rstrip() for line in '\n'.join(output).splitlines())+'\n')
    summary = '\n'.join(f'{variant} {name}: median {statistics.median(values):.4f} ns/op; samples {values}' for (variant,name),values in sorted(readings.items()))+'\n'
    (root / f'adopted-{platform}-summary.txt').write_text(summary)
    print(platform+'\n'+summary, flush=True)
