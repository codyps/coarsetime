//go:build !purego

package coarsetime

import (
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
)

// Read the two layout-dependent offsets from the verified runtime bridge,
// rather than embedding a particular Go release's g/m layout. Fail closed if
// the runtime no longer uses the audited instruction sequence.
var runtimeGMOffset, runtimeMSignalOffset = mustResolveSignalOffsets()

func mustResolveSignalOffsets() (uintptr, uintptr) {
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
	return gm, ms
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
