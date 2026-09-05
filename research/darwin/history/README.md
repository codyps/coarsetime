# Commpage history comparison

Compared selected Apple XNU release files on 2026-09-05. Files are retained in release-tag subdirectories; `downloads.json` records successful downloads (status 0) and curl failures (22, unavailable paths). These are selected snapshots, not an exhaustive search for first-introducing commits. Canonical URLs have the form `https://github.com/apple-oss-distributions/xnu/blob/<tag>/<path>`.

## Observed changes

| Sample | Intel commpage observations |
| --- | --- |
| xnu-1228.0.2 | Version 7; gettimeofday generation at +0x06c, nanosecond base at +0x070, seconds base at +0x078 |
| xnu-1699.22.73 | Version 12; same three wall-time offsets |
| xnu-2422.1.72 | Version 13; header says NT_SHIFT default changed from 32 to 0 |
| xnu-3248.20.55 | Version 13; approximate time at +0x080 and support byte at +0x088 |
| xnu-3789.1.32 | Version 13; continuous-time base at +0x0c0; old wall-time reader still uses generation and two bases |
| xnu-4570.1.46 | Version 13; new five-uint64 wall-time record at +0x0d0; reader uses rate-adjusted binary fractions and a one-second validity horizon |
| xnu-4903.221.2 and xnu-6153.11.26 | Same five-field record and Intel offset |
| xnu-11215.1.10 and current xnu-12377.1.9 | Same five-field record and Intel offset; Intel version 14 |

The older x86 reader (3789) computes `elapsed = mach_absolute_time() - GTOD_NS_BASE`, adds whole elapsed seconds to `GTOD_SEC_BASE`, and converts the remainder to microseconds. It checks a separate generation field. It has no explicit one-second-age rejection in this function. That does not establish the old kernel's publication cadence or adjustment behavior; those paths were not traced here.

The 4570 reader instead uses `(TimeStamp_tick, TimeStamp_sec, TimeStamp_frac, Ticks_scale, Ticks_per_sec)` and rejects interpolation at one nominal second or more. That is the same field order and size as current XNU.

ARM's sampled 4570-through-current headers place the new wall-time record at +0x120. They preserve +0x040 for the old record explicitly for compatibility. Intel also retains its legacy field definitions; retained definitions alone do not prove the kernel still maintains those fields for every historical consumer.

## Stable fields do not mean identical readers

Diffing the 4570 and current gettimeofday helpers shows that the current reader samples `mach_absolute_time()` immediately after reading the anchor, then reads the other fields, then executes an explicit ARM `dmb ishld` before rechecking the anchor. The 4570 reader reads all fields before sampling absolute time and has no explicit trailing ARM barrier in this helper. The current reader also checks `TimeStamp_sec > __LONG_MAX__` and uses commpage access macros with sanitizer support.

## Broader layout evolution

Recent ARM headers have a compile-time `_COMM_PAGE_LAYOUT_VERSION` in addition to `_COMM_PAGE_VERSION`. Selected immutable fields have read-only variants, with legacy offsets retained. The header explains that platform binaries are typically SDK-coupled, and simulators need compatibility with older host kernels. It explicitly avoids using the runtime version in the kernel-writable page to select these security-sensitive fields. The five wall-time fields remain in the kernel-writable data page.

Thus neither a hardcoded address nor the single runtime version number is a complete cross-release compatibility mechanism. The version remained 13 across approximate-time, continuous-time, and new wall-time additions in these Intel samples. The 2017-era wall-time layout has nevertheless been stable across the sampled later releases; this is observed stability, not a public compatibility promise.
