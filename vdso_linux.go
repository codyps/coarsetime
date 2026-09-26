//go:build !purego && linux && amd64

package coarsetime

import (
	"debug/elf"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// This runtime assembly bridge switches to an ABI-aligned system stack and
// restores Go's stack and g afterwards. vDSO functions cannot call back into Go.
// No runtime struct offsets or private vDSO resolver variables are referenced.
//
//go:linkname asmcgocall runtime.asmcgocall
//go:noescape
func asmcgocall(fn, arg unsafe.Pointer) int32

func coarseTrampolineAddress() unsafe.Pointer
func realtimeTrampolineAddress() unsafe.Pointer

var coarseTrampoline = coarseTrampolineAddress()
var realtimeTrampoline = realtimeTrampolineAddress()
var coarseVDSO = resolveCoarseVDSO()

func readCoarseVDSO(ts *syscall.Timespec) bool   { return readClockVDSO(ts, coarseTrampoline) }
func readRealtimeVDSO(ts *syscall.Timespec) bool { return readClockVDSO(ts, realtimeTrampoline) }

func readClockVDSO(ts *syscall.Timespec, trampoline unsafe.Pointer) bool {
	if coarseVDSO == 0 {
		return false
	}
	args := struct {
		fn uintptr
		ts syscall.Timespec
	}{fn: coarseVDSO}
	if asmcgocall(trampoline, unsafe.Pointer(&args)) != 0 {
		return false
	}
	*ts = args.ts
	return true
}

// Resolve only at initialization. Read through /proc/self/mem rather than
// dereferencing unbounded ELF pointers. Restricted procfs or an unfamiliar ELF
// layout simply disables the optimization; the syscall remains available.
func resolveCoarseVDSO() uintptr {
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return 0
	}
	base, size := vdsoMapping(string(maps))
	if size == 0 {
		return 0
	}
	mem, err := os.Open("/proc/self/mem")
	if err != nil {
		return 0
	}
	defer mem.Close()
	f, err := elf.NewFile(io.NewSectionReader(mem, int64(base), int64(size)))
	if err != nil {
		return 0
	}
	defer f.Close()
	name, version, machine := "__vdso_clock_gettime", "LINUX_2.6", elf.EM_X86_64
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || f.Machine != machine || f.Type != elf.ET_DYN {
		return 0
	}
	symbols, err := f.DynamicSymbols()
	if err != nil {
		return 0
	}
	for i, sym := range symbols {
		if sym.Name != name || elf.ST_TYPE(sym.Info) != elf.STT_FUNC || sym.Section == elf.SHN_UNDEF {
			continue
		}
		if sym.Value >= size || sym.Size > size-sym.Value || !vdsoVersion(f, i+1, version) {
			return 0
		}
		// Require the function's first byte to lie in an executable load segment.
		for _, p := range f.Progs {
			if p.Type == elf.PT_LOAD && p.Flags&elf.PF_X != 0 && sym.Value >= p.Vaddr && sym.Value-p.Vaddr < p.Memsz {
				return uintptr(base + sym.Value)
			}
		}
	}
	return 0
}

func vdsoMapping(maps string) (base, size uint64) {
	for _, line := range strings.Split(maps, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 6 || fields[5] != "[vdso]" || !strings.Contains(fields[1], "x") {
			continue
		}
		lo, hi, ok := strings.Cut(fields[0], "-")
		if !ok {
			return 0, 0
		}
		start, e1 := strconv.ParseUint(lo, 16, 63)
		end, e2 := strconv.ParseUint(hi, 16, 63)
		// Reject unexpectedly large or malformed mapping ranges.
		if e1 != nil || e2 != nil || end <= start || end-start > 1<<20 {
			return 0, 0
		}
		return start, end - start
	}
	return 0, 0
}

// Match the symbol's GNU version index against the kernel's version definition.
// DynamicSymbols omits ELF symbol zero, hence the caller adds one to its index.
func vdsoVersion(f *elf.File, symbol int, want string) bool {
	vs, vd := f.Section(".gnu.version"), f.Section(".gnu.version_d")
	if vs == nil || vd == nil || int(vd.Link) >= len(f.Sections) {
		return false
	}
	versions, e1 := vs.Data()
	defs, e2 := vd.Data()
	names, e3 := f.Sections[vd.Link].Data()
	if e1 != nil || e2 != nil || e3 != nil || symbol >= len(versions)/2 {
		return false
	}
	order := f.ByteOrder
	index := order.Uint16(versions[symbol*2:]) & 0x7fff
	for off := uint64(0); off+20 <= uint64(len(defs)); {
		d := defs[off:]
		if order.Uint16(d) != 1 {
			return false
		}
		if order.Uint16(d[4:]) == index {
			aux := off + uint64(order.Uint32(d[12:]))
			if aux+8 > uint64(len(defs)) {
				return false
			}
			name := uint64(order.Uint32(defs[aux:]))
			if name >= uint64(len(names)) {
				return false
			}
			tail := string(names[name:])
			end := strings.IndexByte(tail, 0)
			return end >= 0 && tail[:end] == want
		}
		next := uint64(order.Uint32(d[16:]))
		if next < 20 {
			return false
		}
		off += next
	}
	return false
}
