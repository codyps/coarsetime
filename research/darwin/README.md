# Darwin wall-clock implementation: source investigation

Implementation update (2026-09-27): the production calendar reader now uses the
kernel mapping described below. See [calendar reader notes](calendar/README.md)
for its approximation contract, fallback behavior, and current validation limits.
Earlier cached-wall measurements are historical.

Examined 2026-09-05. Apple source was downloaded as shallow Git clones:

| Repository | Release tag | Commit | Local directory |
| --- | --- | --- | --- |
| [XNU](https://github.com/apple-oss-distributions/xnu) | xnu-12377.1.9 | f6217f891ac0bb64f3d375211650a4c1ff8ca1ea | `xnu/` |
| [Libc](https://github.com/apple-oss-distributions/Libc) | Libc-1752.100.10 | 71bbe350ab79eef58113991d817ccc6165061a64 | `Libc/` |

Downloaded Apple source trees (including `history/xnu-*/` snapshots) are excluded
from Git. Research notes, release manifests, download records, and benchmark
results are retained. To restore the two pinned primary checkouts locally:

```sh
git clone --depth 1 --branch xnu-12377.1.9 https://github.com/apple-oss-distributions/xnu.git research/darwin/xnu
git clone --depth 1 --branch Libc-1752.100.10 https://github.com/apple-oss-distributions/Libc.git research/darwin/Libc
```

Run those commands from the project root. Historical source links require the
corresponding files listed in `history/downloads.json`, retrieved from their
Apple release tags. The local source links in these notes resolve after download.

These were the repositories' `main` heads at download time. The host reports macOS 26.6.2 (25G83), Darwin 25.6.0, XNU 12377.161.14, x86_64. The downloaded kernel is therefore **not the exact running kernel**. Findings below describe the pinned source; installed-library disassembly and runtime clock-adjustment experiments were not performed. No clock settings or project implementation files were changed.

## Core mechanism

Darwin maintains calendar time as a software mapping from Mach absolute time:

```
calendar(now) = calendar_base + scaled(now - absolute_base)
```

The kernel owns the base and NTP-adjusted scale. It exports a shorter-lived snapshot through the commpage, so ordinary wall-clock reads can perform this calculation entirely in userspace. This is not a hardware RTC read on every call, nor a counter incremented by every timer interrupt.

## Userspace interfaces

* `gettimeofday()` first calls `__commpage_gettimeofday()`. On failure it calls the `__gettimeofday` syscall stub. Result: Unix-epoch seconds and microseconds. Time-zone handling is separate libc logic.
* In this Libc snapshot, `clock_gettime(CLOCK_REALTIME)` calls `gettimeofday()` and converts its timeval to a timespec. `clock_gettime_nsec_np(CLOCK_REALTIME)` also uses `gettimeofday()`. Their nanosecond fields/units do **not** give them nanosecond granularity: they derive from microseconds.
* `mach_get_times()` can return calendar, absolute, and continuous time together. Its commpage helper returns the same absolute sample used for the calendar calculation. It retries across changes to the continuous-time base, and has a syscall fallback. Its calendar timespec also derives from microseconds.
* Mach's `CALENDAR_CLOCK` service reaches `calend_gettime()` and then `clock_get_calendar_nanotime()`. That kernel path converts the internal fractional value to nanoseconds; it is distinct from libc's microsecond-derived realtime path. Representation granularity does not establish accuracy.

Sources: [Libc gettimeofday](Libc/sys/gettimeofday.c), [Libc clock_gettime](Libc/gen/clock_gettime.c), [mach_get_times](xnu/libsyscall/wrappers/mach_get_times.c), [Mach clock operations](xnu/osfmk/kern/clock_oldops.c).

## Commpage layout, calculation, and synchronization

`new_commpage_timeofday_data_t` has five 64-bit fields:

| Field | Meaning |
| --- | --- |
| `TimeStamp_tick` | Anchor in Mach absolute ticks; zero means invalid |
| `TimeStamp_sec` | Calendar seconds at the anchor |
| `TimeStamp_frac` | Fractional second in units of 2^-64 seconds |
| `Ticks_scale` | Fractional-second units per absolute tick, including adjustment |
| `Ticks_per_sec` | Nominal absolute ticks per second; also the validity horizon |

Userspace reads the anchor tick, samples `mach_absolute_time()`, reads the other fields, and rereads the anchor tick. If it changed, it retries. ARM adds an explicit load barrier; the absolute-time implementation supplies instruction-ordering protection. The writer, serialized by the kernel clock lock, first publishes a zero anchor, writes the other fields, then publishes the new anchor. ARM writers use barriers between these stages. This is a timestamp-based consistency protocol, not an ordinary independent-field snapshot.

The reader rejects a zero anchor, `now - anchor >= Ticks_per_sec`, or seconds outside its signed-long range. Otherwise it adds `delta * Ticks_scale` to the fractional timestamp, propagates multiplication overflow and fractional carry into seconds, then converts the fraction to microseconds. The exact conversion uses the upper 32 fractional bits: `(1000000 * (frac >> 32)) >> 32`.

The kernel maps commpage data read-only into userspace while retaining a writable kernel mapping. Readers acquire no kernel lock and normally need no kernel transition. An ARM machine without supported userspace counter access can still trap inside `mach_absolute_time()`.

Sources: [layout](xnu/bsd/sys/commpage.h), [reader](xnu/libsyscall/wrappers/__commpage_gettimeofday.c), [Intel writer and mappings](xnu/osfmk/i386/commpage/commpage.c), [ARM writer](xnu/osfmk/arm/commpage/commpage.c).

## Underlying absolute clock

On x86_64, `mach_absolute_time()` reads the TSC with `rdtsc` and fences, then uses commpage TSC base, scale, shift, and nanosecond base to convert it. It validates a generation number around the operation. The x86 kernel reports a 1:1 Mach timebase ratio: returned absolute ticks are nanoseconds, not raw TSC cycles.

On arm64, it reads an architectural counter (`CNTVCT_EL0`, or a supported non-speculative variant), adds the commpage timebase offset, and retries if the offset changed. The offset handles the absolute clock's sleep semantics. The timebase ratio converts these ticks to nanoseconds. There is a kernel-trap fallback when direct counter reads are unavailable.

Absolute time excludes sleep; continuous time includes it. Neither is itself an epoch-based calendar timestamp.

Sources: [absolute-time assembly](xnu/libsyscall/wrappers/mach_absolute_time.s), [Intel rtclock](xnu/osfmk/i386/rtclock.c), [ARM rtclock](xnu/osfmk/arm/rtclock.c).

## Kernel calendar state and maintenance

`clock_calend` in `osfmk/kern/clock.c:279` contains:

* `offset_count`: absolute tick anchor for the current scale.
* `boottime`: calendar epoch component.
* `offset`: accumulated scaled elapsed time, including added sleep.
* `bintime`: combined calendar base (`boottime + offset`).
* `tick_scale_x`, `s_scale_ns`, and `s_adj_nsx`: fixed-point rate/conversion parameters.

A `bintime` holds whole seconds and a 64-bit binary fraction. `get_scaled_time()` subtracts `offset_count` and calls `scale_delta()`. For long intervals, `scale_delta()` handles whole seconds separately from residual ticks, using extra fractional adjustment to retain precision. Normal calendar readers run under `clock_lock()` at clock interrupt priority.

**Boot:** `clock_initialize_calendar()` reads platform UTC via `PEGetUTCTimeOfDay()`, reads uptime, derives the UTC-minus-uptime epoch, and initializes nominal scaling. The platform call dispatches through `gIOPlatform->getUTCTimeOfDay()`. The generic XNU platform expert contains fallback/stub methods; this checkout alone does not identify the exact RTC hardware transaction for each Mac or iPhone.

**Read fallback:** BSD `gettimeofday()` calls `clock_gettimeofday_and_absolute_time()`. It computes calendar time under the clock lock and refreshes the shared commpage anchor and scale before returning/copying out the result. Thus a read can refresh the fast path for other processes too.

**NTP and slewing:** `ntp_adjtime()` supplies discipline parameters; `adjtime()` supplies a pending correction. `clock_update_calendar()` first accumulates elapsed time using the old scale, moves the absolute anchor to now, asks `ntp_update_second()` for the next adjustment, computes new scales, and republishes the commpage. This keeps successive rate changes continuous. The NTP timer has a one-second period **while adjustment is evolving**, and can stop when the adjustment stabilizes. A fixed frequency correction can remain active without a permanent periodic updater.

In this implementation, pending `adjtime()` corrections larger than one second use 5 ms/s slew; smaller corrections use up to 500 microseconds/s, with the final remainder consumed separately. These rates describe this particular adjustment path, not an overall accuracy guarantee or every synchronization policy.

**Explicit clock steps:** `settimeofday()` reaches `clock_set_calendar_microtime()`. It invalidates the commpage, shifts the epoch/boot-time values by the requested difference, republishes wall-clock data, updates the platform UTC clock with `PESetUTCTimeOfDay()`, and emits calendar notifications. A separate mutex serializes the potentially blocking platform update. Wall time can jump backward or forward.

**Sleep/wake:** the calendar wake path invalidates the commpage and adds elapsed sleep to the calendar offset/base. On configurations with a continuous hardware clock, it derives sleep from continuous-minus-absolute time and applies the current NTP frequency correction to the sleep interval. Legacy paths use a platform monotonic clock or platform UTC to estimate the missing interval, with checks against negative sleep. Subsequent wall-clock reads refresh the invalidated snapshot.

Sources: [calendar implementation](xnu/osfmk/kern/clock.c), [NTP discipline and timer](xnu/bsd/kern/kern_ntptime.c), [BSD syscalls](xnu/bsd/kern/kern_time.c), [platform boundary](xnu/iokit/Kernel/IOPlatformExpert.cpp).

## Implications for coarsetime

The wall-clock commpage anchor is **not a bounded-staleness coarse realtime clock**. Its one-second rule bounds permitted interpolation from the anchor, not how frequently the kernel must update the anchor. When adjustments are stable and there are no fallback reads, the stored anchor can remain old.

`mach_approximate_time()` uses a different commpage slot. It returns a cached absolute timestamp when supported, otherwise `mach_absolute_time()`. Scheduler context-switch and quantum-expiration paths publish approximate samples. These updates do not publish calendar anchors or apply NTP wall-clock corrections.

Consequently, simply adding a startup Unix offset to approximate absolute time misses later clock steps, slews, and sleep. Substituting approximate ticks into the calendar reader is also not automatically safe: a cached approximate sample can precede a freshly published wall anchor, causing unsigned delta underflow; independent publication and invalidation must be handled. A private commpage implementation would need the same consistency, validity, overflow, and fallback logic plus a defined approximation contract. This is an inference from the two source paths, not a proposed implementation change.

Sources: [approximate reader](xnu/libsyscall/wrappers/mach_approximate_time.c), [context switching](xnu/osfmk/kern/sched_prim.c), [quantum expiration](xnu/osfmk/kern/priority.c).
