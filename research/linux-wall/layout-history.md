# x86-64 coarse wall-clock layout history

Checked 2026-09-05 using upstream tag snapshots and the complete GitHub commit
lists for include/vdso/datapage.h and arch/x86/include/asm/{vgtod,vvar}.h.
Scope: ordinary x86-64 CLOCK_REALTIME_COARSE data, not all architectures, distro
backports, every field, or every VVAR mapping/protocol change. Downloaded source
snapshots and API responses are under /tmp/coarsetime-vvar-history.

Offsets below are derived from the x86-64 structure definitions, relative to
the start of the VVAR **time data page**, not to the vDSO ELF base or an arbitrary
/proc/self/maps entry. They are historical observations, not an ABI contract.

| Upstream releases | seq | coarse sec | coarse nsec |
| --- | ---: | ---: | ---: |
| 3.17–4.19 | 128 | 192 | 200 |
| 4.20–6.9 | 128 | 240 | 248 |
| 6.10–6.12 | 128 | 248 | 256 |
| 6.13–7.0 | 0 | 120 | 128 |

Three offset-changing transitions in that interval:

- **4.20:** [introduce vgtod_ts](https://github.com/torvalds/linux/commit/49116f2081ee)
  replaces named timestamp fields with an array indexed by clock ID. Coarse
  realtime seconds move from record offset 64 to 112.
- **6.10:** [add max_cycles](https://github.com/torvalds/linux/commit/d2e58ab5cda2)
  inserts an eight-byte field before mask/mult/shift/basetime. x86 selects
  GENERIC_VDSO_OVERFLOW_PROTECT, so the field is present there. Seconds move
  from record offset 112 to 120. Confirmed absent in v6.9, present in v6.10.
- **6.13:** [place data at the beginning of VVAR](https://github.com/torvalds/linux/commit/9f8514cfcdf0)
  moves the record from page offset 128 to 0. v6.12 still uses DECLARE_VVAR;
  v6.13's __arch_get_vdso_data returns &vvar_page directly.

Other changes must not be conflated with those offsets:

- **5.3:** the generic vDSO conversion preserves the first clock record's coarse
  timestamp offsets on x86-64.
- **5.6:** time namespaces introduce special pages with odd seq and TIMENS mode;
  ordinary timestamp offsets remain, but a reader must handle/reject the special
  page instead of spinning forever. This is a protocol/locator change.
- **6.15:** [vdso_clock refactoring](https://github.com/torvalds/linux/commit/886653e36639)
  restructures the complete data object and subsequent clock-record placement.
  The first coarse wall record retains its offsets on x86-64: architecture time
  data is empty there. It does affect other parts of the layout.
- **6.17:** auxiliary clock records are appended after clock_data; they do not
  shift the first coarse wall timestamp. v7.0 retains that first-record prefix.

Thus the needed coarse offsets lasted through **31 consecutive upstream release
series, 4.20–6.9**, though that is not 31 releases with an entirely unchanged
VVAR interface. The empirical history is much less volatile than a general
warning about private layouts suggests. It supports investigating a carefully
recognized-layout fast path with fallback; it does not establish that uname
version checks or a one-time matching timestamp are sufficient validation.
