//go:build !purego && linux && (amd64 || arm64)

package coarsetime

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// Resolve once during package initialization. The assembly trampoline reads this
// address on every call.
var runtimeAsmcgocall = mustResolveAsmcgocall()

func mustResolveAsmcgocall() uintptr {
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		panic("coarsetime: resolve runtime.asmcgocall: " + err.Error())
	}
	defer f.Close()
	pc, err := resolveAsmcgocall(f, reflect.ValueOf(runtime.Gosched).Pointer())
	if err != nil {
		// A runtime compatibility failure must not silently turn all reads into
		// kernel syscalls, nor leave an unchecked indirect call target.
		panic("coarsetime: resolve runtime.asmcgocall: " + err.Error())
	}
	return pc
}

// Resolve from the executable's PC/line table, which survives -s -w. This uses
// neither ELF symbols/DWARF nor guessed addresses in live runtime memory.
// asmcgocall remains a private runtime ABI dependency; validate both the entry
// and its source against the running process before accepting it.
func resolveAsmcgocall(f *elf.File, anchorPC uintptr) (uintptr, error) {
	text := f.Section(".text")
	if text == nil {
		return 0, fmt.Errorf("missing ELF text section")
	}
	pcln := f.Section(".gopclntab")
	if pcln == nil {
		pcln = f.Section(".data.rel.ro.gopclntab")
	}
	if pcln == nil {
		return 0, fmt.Errorf("missing Go PC/line table")
	}
	data, err := pcln.Data()
	if err != nil {
		return 0, fmt.Errorf("read Go PC/line table: %w", err)
	}
	table, err := gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		return 0, fmt.Errorf("parse Go PC/line table: %w", err)
	}
	anchor := table.LookupFunc("runtime.Gosched")
	if anchor == nil {
		return 0, fmt.Errorf("missing runtime.Gosched relocation anchor")
	}
	liveAnchor := runtime.FuncForPC(anchorPC)
	if liveAnchor == nil || liveAnchor.Entry() != anchorPC || liveAnchor.Name() != anchor.Name {
		return 0, fmt.Errorf("invalid runtime.Gosched relocation anchor")
	}
	// ELF addresses are unrelocated in PIE executables. Use an ordinary Go
	// function with a known live address to account for the process load bias.
	bias := anchorPC - uintptr(anchor.Entry)
	var found uintptr
	for _, fn := range table.Funcs {
		if fn.Name != "runtime.asmcgocall" {
			continue
		}
		file, _, _ := table.PCToLine(fn.Entry)
		// The ABIInternal wrapper shares the same name in pclntab. Only the
		// assembly implementation has this source file; the wrapper is generated.
		if file != "runtime/asm_"+runtime.GOARCH+".s" && !strings.HasSuffix(file, "/runtime/asm_"+runtime.GOARCH+".s") {
			continue
		}
		pc := uintptr(fn.Entry) + bias
		live := runtime.FuncForPC(pc)
		if live == nil || live.Entry() != pc || live.Name() != fn.Name {
			return 0, fmt.Errorf("runtime.asmcgocall entry validation failed")
		}
		liveFile, _ := live.FileLine(pc)
		if liveFile != file {
			return 0, fmt.Errorf("runtime.asmcgocall source validation failed")
		}
		if found != 0 {
			return 0, fmt.Errorf("ambiguous runtime.asmcgocall assembly entry")
		}
		found = pc
	}
	if found == 0 {
		return 0, fmt.Errorf("runtime.asmcgocall assembly entry not found")
	}
	return found, nil
}
