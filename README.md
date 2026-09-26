# coarsetime

Low-overhead wall-clock timestamps for Go. Windows/amd64 has a direct coarse
clock fast path; all other platforms use the standard library.

```go
import (
    "time"

    "github.com/codyps/coarsetime"
)

timestamp := coarsetime.UnixNano() // Coarse wall timestamp, nanosecond units.
started := coarsetime.Now()       // time.Time with Go's monotonic reading.
// ...
elapsed := time.Since(started)
```

`Now()` is equivalent to `time.Now()`. `UnixNano()` avoids constructing a
`time.Time` on Windows/amd64 by reading Windows' shared SystemTime clock.
It needs no cgo, allocation, background goroutine, or timer-resolution change.

The clock is coarse: nanosecond units do not imply nanosecond resolution.
Observed updates on the development machine were about 0.5–1 ms, but cadence
depends on the system, and no maximum staleness is promised. Wall time can jump
forwards or backwards. Use `time.Since(started)` for elapsed time, with
the same platform-dependent suspend behavior as Go's clock. Unix nanoseconds
have the same representable range as `time.Time.UnixNano`.

The Windows optimization depends on the same shared-page layout used by the
Go Windows/amd64 runtime. Build with `-tags=purego` to force the standard-library
implementation. Windows/arm64, Windows/386, Darwin, and other platforms already
use that fallback. The original unfinished Darwin trampoline has been removed;
a Darwin-specific optimization remains future work.

For precise Windows intervals, use QueryPerformanceCounter through a Windows
binding; for precise wall time, use GetSystemTimePreciseAsFileTime. These are
different use cases from this package's coarse timestamps. See the
[investigation and plan](WINDOWS_PLAN.md) and [API experiments](experiments/windows/README.md).

On the development Windows 11 / Ryzen 7940HS machine with Go 1.27.0, the public
`UnixNano()` API took a median **2.55 ns/op**, compared with **9.17 ns/op** for
`time.Now().UnixNano()` (about **3.6× faster**). Both allocated zero bytes.
These are five sequential 500 ms samples at GOMAXPROCS=1 on one machine;
`Now()` is the standard-library call, regardless of benchmark variation.
[Raw public API results](experiments/windows/public-api-benchmark-results.txt).

Requires Go 1.23 or later. Run:

```sh
go test ./...
go test -tags=purego ./...
go vet ./...
go test -run '^$' -bench . -benchmem -benchtime=500ms -count=5 -cpu=1
```

Tests cover current-clock consistency, concurrent readers, monotonic preservation
in `Now`, and Windows epoch/range conversions. CI also tests portable fallbacks.
Manual validation of clock changes, sleep/resume, Windows Server, and additional
hardware remains necessary before claiming coverage of those environments.
