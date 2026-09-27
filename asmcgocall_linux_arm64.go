//go:build !purego

package coarsetime

import (
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// Read the two layout-dependent offsets from the verified runtime bridge,
// rather than embedding a particular Go release's g/m layout. Fail closed if
// the runtime no longer uses the audited instruction sequence.
var runtimeGMOffset, runtimeMSignalOffset, runtimeVDSOPCOffset, runtimeVDSOSPOffset = mustResolveSignalOffsets()

func mustResolveSignalOffsets() (uintptr, uintptr, uintptr, uintptr) {
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	pc, err := resolveAsmcgocall(f, reflect.ValueOf(runtime.Gosched).Pointer())
	if err != nil || pc != runtimeAsmcgocall {
		panic("coarsetime: cannot verify ARM64 signal bridge")
	}
	text := f.Section(".text")
	pcln := f.Section(".gopclntab")
	if pcln == nil {
		pcln = f.Section(".data.rel.ro.gopclntab")
	}
	data, err := pcln.Data()
	if err != nil {
		panic(err)
	}
	table, err := gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		panic(err)
	}
	anchor := table.LookupFunc("runtime.Gosched")
	bias := reflect.ValueOf(runtime.Gosched).Pointer() - uintptr(anchor.Entry)
	entry := uint64(pc-bias) - text.Addr
	code := make([]byte, 64)
	if _, err := text.ReadAt(code, int64(entry)); err != nil {
		panic(err)
	}
	gm, ms, err := decodeSignalOffsets(code)
	if err != nil {
		panic("coarsetime: " + err.Error())
	}
	for _, fn := range table.Funcs {
		if fn.Name != "runtime.nanotime1" {
			continue
		}
		file, _, _ := table.PCToLine(fn.Entry)
		if !strings.HasSuffix(file, "runtime/sys_linux_arm64.s") {
			continue
		}
		code := make([]byte, 128)
		if _, err := text.ReadAt(code, int64(fn.Entry-text.Addr)); err != nil {
			panic(err)
		}
		pcOffset, spOffset, err := decodeVDSOOffsets(code)
		if err != nil {
			panic("coarsetime: " + err.Error())
		}
		return gm, ms, pcOffset, spOffset
	}
	panic("coarsetime: runtime.nanotime1 assembly entry not found")
}

func decodeSignalOffsets(code []byte) (uintptr, uintptr, error) {
	// LDR X8,[X28,#g_m]; LDR X3,[X8,#m_gsignal]; CMP X28,X3;
	// B.EQ ... . These are the first two loads in runtime.asmcgocall,
	// following its frame prologue and CBZ g. Only unsigned offsets vary.
	for i := 0; i+16 <= len(code); i += 4 {
		a := binary.LittleEndian.Uint32(code[i:])
		b := binary.LittleEndian.Uint32(code[i+4:])
		c := binary.LittleEndian.Uint32(code[i+8:])
		d := binary.LittleEndian.Uint32(code[i+12:])
		if a & ^uint32(0x003ffc00) == 0xf9400388 &&
			b & ^uint32(0x003ffc00) == 0xf9400103 &&
			c == 0xeb03039f && d&0xff00001f == 0x54000000 {
			return uintptr((a>>10)&4095) * 8, uintptr((b>>10)&4095) * 8, nil
		}
	}
	return 0, 0, fmt.Errorf("unrecognized runtime.asmcgocall ARM64 signal-stack loads")
}

// Recognize nanotime1's save/publish sequence, checking that the later stores
// use the same offsets as the loads. These fields are private runtime ABI.
func decodeVDSOOffsets(code []byte) (uintptr, uintptr, error) {
	const imm = uint32(0x003ffc00)
	for i := 0; i+28 <= len(code); i += 4 {
		w := func(n int) uint32 { return binary.LittleEndian.Uint32(code[i+4*n:]) }
		a, b := w(0), w(1)
		if a & ^imm == 0xf94002a2 && b & ^imm == 0xf94002a3 &&
			w(2) == 0xf90007e2 && w(3) == 0xf9000be3 &&
			w(4) & ^imm == 0x910003e2 &&
			w(5) == 0xf90002be|(a&imm) && w(6) == 0xf90002a2|(b&imm) {
			pc, sp := uintptr((a>>10)&4095)*8, uintptr((b>>10)&4095)*8
			if pc != 0 && sp != 0 && pc != sp {
				return pc, sp, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("unrecognized runtime.nanotime1 ARM64 traceback fields")
}
