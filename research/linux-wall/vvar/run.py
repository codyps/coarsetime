"""Run shuffled Linux VVAR benchmarks; requires /tmp/coarsetime-vvar.test."""
import pathlib,random,re,statistics,subprocess
root=pathlib.Path(__file__).resolve().parent
names=['CurrentNow','GoNow','ASMNow','CurrentUnixNano','GoUnixNano','ASMUnixNano']
rng=random.Random(20260905)
raw=[];samples={name:[] for name in names}
for trial in range(5):
 order=names.copy();rng.shuffle(order)
 for name in order:
  cmd=['docker','run','--rm','--network','none','-e','COARSETIME_REQUIRE_VDSO=1','-e','COARSETIME_VVAR_LAYOUT=modern','-v','/tmp/coarsetime-vvar.test:/test:ro','python:3.12-slim','/test','-test.run=^Test(VvarPrototype|VDSO)$','-test.v','-test.bench=^BenchmarkVvar'+name+'$','-test.benchmem','-test.benchtime=300ms']
  text=subprocess.run(cmd,check=True,capture_output=True,text=True).stdout
  raw.append(f'Round {trial+1}, {name}\n'+text)
  ns,size,alloc=re.search(r'BenchmarkVvar'+name+r'-\d+\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op',text).groups()
  assert size==alloc=='0'
  samples[name].append(float(ns))
(root/'raw.txt').write_text('\n'.join(line.rstrip() for line in '\n'.join(raw).splitlines())+'\n')
summary='\n'.join(f'{name}: {statistics.median(values):.4f} ns/op; samples={values}' for name,values in samples.items())+'\n'
(root/'summary.txt').write_text(summary)
print(summary)
