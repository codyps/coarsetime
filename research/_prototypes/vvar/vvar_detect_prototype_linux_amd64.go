//go:build vvarprototype

package coarsetime

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"io"
	"os"
)

// This profile was audited against the complete mapped clock_gettime function
// on 7.0.14-orbstack-00380-ga7e0a2dc9535. It identifies code, not uname or build ID.
// CPU alternatives/compiler changes can cause safe false negatives.
const vvarPrototypeClockHash = "278894e4280a170ff9bad9c3352bed501d5eeae8ae73eac5ce4a1f2ba3557fa7"

func vvarPrototypeRecognize(image []byte) (dataDelta int64, ok bool) {
	f, err := elf.NewFile(bytes.NewReader(image))
	if err != nil {
		return 0, false
	}
	defer f.Close()
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || f.Machine != elf.EM_X86_64 || f.Type != elf.ET_DYN {
		return 0, false
	}
	syms, err := f.DynamicSymbols()
	if err != nil {
		return 0, false
	}
	for n, s := range syms {
		if s.Name != "__vdso_clock_gettime" {
			continue
		}
		if elf.ST_TYPE(s.Info) != elf.STT_FUNC || s.Section == elf.SHN_UNDEF || s.Value != 0xaf0 || s.Size != 1208 || !vdsoVersion(f, n+1, "LINUX_2.6") {
			return 0, false
		}
		for _, p := range f.Progs {
			if p.Type != elf.PT_LOAD || p.Flags&elf.PF_X == 0 || p.Off != p.Vaddr || s.Value < p.Vaddr {
				continue
			}
			off := s.Value - p.Vaddr
			if off > p.Filesz || s.Size > p.Filesz-off || p.Off > uint64(len(image)) || off > uint64(len(image))-p.Off {
				continue
			}
			start := p.Off + off
			if s.Size > uint64(len(image))-start {
				return 0, false
			}
			sum := sha256.Sum256(image[start : start+s.Size])
			// Compare without adding a hex package solely for this one profile.
			const digits = "0123456789abcdef"
			var encoded [64]byte
			for i, b := range sum {
				encoded[2*i] = digits[b>>4]
				encoded[2*i+1] = digits[b&15]
			}
			if string(encoded[:]) != vvarPrototypeClockHash {
				return 0, false
			}
			// Audited RIP-relative target: LEA at 0xcbc, next IP 0xcc3,
			// displacement -0x6cc3 => ELF base -0x6000. Coarse timestamp
			// address is that target + clock_id*16 + 0x28 (120 for ID 5).
			return -0x6000, true
		}
		return 0, false
	}
	return 0, false
}

func vvarPrototypeDetect(maps string) uintptr {
	base, size := vdsoMapping(maps)
	if base == 0 || size == 0 {
		return 0
	}
	mem, err := os.Open("/proc/self/mem")
	if err != nil {
		return 0
	}
	defer mem.Close()
	image := make([]byte, int(size))
	if _, err = io.ReadFull(io.NewSectionReader(mem, int64(base), int64(size)), image); err != nil {
		return 0
	}
	delta, ok := vvarPrototypeRecognize(image)
	if !ok || delta >= 0 || uint64(-delta) > base {
		return 0
	}
	return uintptr(base - uint64(-delta))
}
