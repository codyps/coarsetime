# Direct Darwin commpage prototypes

Prototype sources live under `research/_prototypes/` and run through the
[research overlay runner](../../README.md#same-package-prototypes).


The root package has three comparison prototypes under the `commpageprototype`
build tag. These results were collected before adoption. Darwin amd64 now uses
the once-checked atomic reader in ordinary builds; arm64 retains the libc bridge.
No public API is changed.

* **CheckedASM:** assembly reads the commpage support byte and, if supported,
  the aligned approximate timestamp on each call. A zero result uses libc.
* **OnceASM:** initialization reads the support byte; each call branches on the
  saved bool and calls a small assembly timestamp loader, or falls back to libc.
* **OnceAtomic:** initialization also obtains the slot pointer from assembly;
  the supported path uses Go `atomic.LoadUint64`. The uncommon libc bridge is
  outlined so that the Go reader and Instant wrapper fit the inlining budget.

The once-checked variants still test the saved capability on each call; they do
not repeatedly read the support byte from the commpage. The address and support
flag are architecture-specific. Both x86_64 and arm64 assembly are provided.
The fallback preserves existing `mach_approximate_time` behavior when the
approximate clock is unsupported. These prototypes assume the documented-in-XNU
commpage mapping exists; the support flag is not a general mapping/ABI probe.

Each variant has the same four operations as the library: Instant read, elapsed
conversion, UnixNano, and time.Time construction. Wall operations retain the
existing cached correction and `RefreshWallClock` contract; they do not read the
calendar commpage record and do not improve sleep/NTP/clock-step freshness.

Source files:

* [Readers and API-equivalent wrappers](../../_prototypes/darwin-commpage/commpage_prototype_darwin.go)
* [Intel assembly](../../_prototypes/darwin-commpage/commpage_prototype_darwin_amd64.s)
* [ARM64 assembly](../../_prototypes/darwin-commpage/commpage_prototype_darwin_arm64.s)
* [Tests and benchmarks](../../_prototypes/darwin-commpage/commpage_prototype_darwin_test.go)

## Validation

On the native Darwin amd64 host, direct support was true and the timestamp
pointer was `0x7fffffe00080`. All three readers were bracketed by libc reads;
10,000 samples per reader stayed within their brackets. Forced unsupported
branches of the once-checked readers were checked the same way. Progress,
concurrent atomic reads during GC and recursive stack growth, and cached-wall
mapping brackets passed. The complete package passed race, strict checkptr,
and vet checks with the prototype tag, and ordinary untagged tests passed.

ARM64 cross-compilation passed; ARM64 execution and performance are unverified.
No sleep/resume, manual clock-step, or older-OS runtime experiments were run.
Go 1.26.6 was used; no older Go compiler was tested for these prototypes.

The [Intel disassembly](amd64-disassembly.txt) confirms the atomic load is inside
the benchmark loop and that the supported atomic path has no function call.
At the time of this run, the libc baseline was the public API. After adoption,
explicit libc control wrappers preserve that baseline for reruns. Prototype wrappers preserve the
same result types and conversion work. Loops use direct calls, not a common
function-pointer dispatch that would hide differences in inlining.

## Reproduction

From the repository root (GOCACHE is set explicitly for this workspace sandbox):

```sh
export GOCACHE=/tmp/coarsetime-prototype-go-cache
python3 research/prototype.py darwin-commpage test -race .
python3 research/prototype.py darwin-commpage test -gcflags=all=-d=checkptr=2 .
python3 research/prototype.py darwin-commpage vet .
CGO_ENABLED=0 python3 research/prototype.py darwin-commpage test -c -o /tmp/coarsetime-prototype-amd64.test .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 python3 research/prototype.py darwin-commpage test -c -o /tmp/coarsetime-prototype-arm64.test .
python3 research/darwin/commpage/run.py /tmp/coarsetime-prototype-amd64.test
```

The runner invokes the already-built native binary serially, shuffling all 18
cases in each of five rounds with a fixed seed. Each case requests 500 ms.
No agent compilation or test jobs ran alongside benchmarks. Results describe
hot-loop throughput including the result store, not a dependency-serialized
memory-load latency or a clock freshness bound. They are host-specific.

Raw output is in [raw.txt](raw.txt); medians, ranges, and all samples are in
[summary.json](summary.json). The run aborts if a case is skipped or allocates.

## Results: 2026-09-05

Native Intel Mac, Darwin 25.6.0 / XNU 12377.161.14, Go 1.26.6,
CGO_ENABLED=0 benchmark binary. The benchmark output identifies the CPU as
Intel i9-9880H; a separate shell CPU-brand sysctl was denied by the sandbox. Values below are medians in ns/op; all cases allocated
zero bytes and zero objects.

| Operation | Current libc | Checked ASM | Once ASM | Once atomic | Atomic speedup |
| --- | ---: | ---: | ---: | ---: | ---: |
| NowInstant | 16.980 | 2.507 | 2.545 | 0.622 | 27.29x |
| Since | 19.720 | 4.800 | 5.207 | 3.148 | 6.26x |
| UnixNano | 19.870 | 3.938 | 4.417 | 2.255 | 8.81x |
| Now | 21.530 | 6.662 | 6.748 | 5.001 | 4.31x |

Controls: `time.Now()` 76.73 ns/op; `time.Since()` 34.64 ns/op.

Atomic variant sample ranges (minimum to maximum, ns/op):

* Instant: 0.618–0.631
* Since: 3.112–3.173
* UnixNano: 2.232–2.399
* Now: 4.914–5.115

The Go atomic variant wins by eliminating both the libc bridge and the Go
assembly call/return. On this Intel target its atomic load becomes an ordinary
MOVQ. It is the best candidate for a subsequent production change. ARM64's
atomic instruction selection and performance require native measurement before
claiming the same gain. The assembly variants remain useful comparisons: their
approximately 2.5 ns Instant reads demonstrate that most of the current cost
comes from the bridge even without inlining the final load.

These results do not measure cold-cache behavior or performance under heavy
scheduler activity. The subsequent amd64 adoption is measured in [adopted.txt](adopted.txt).
No commit or push was made. The hashes and disassembly alongside the original
results identify the pre-adoption experiment; they are retained as historical evidence.
