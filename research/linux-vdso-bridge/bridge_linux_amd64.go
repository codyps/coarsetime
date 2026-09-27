//go:build linux && amd64

// Package bridge contains research prototypes, not a supported library API.
package bridge

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var clockEntry = resolveCoarseVDSO()
var runtimeBridge, runtimeBridgeError = resolveRuntimeBridge()
var trampoline = trampolineAddress()

//go:noescape
func indirectCall(fn, arg unsafe.Pointer) int32
func trampolineAddress() unsafe.Pointer

// The pointer-free argument record may live on the goroutine stack. No vDSO
// callback into Go is allowed; the bridge does not release the P.
type clockArgs struct {
	fn uintptr
	id int64
	ts syscall.Timespec
}

func Indirect(id int32) (int64, int64, int32) {
	args := clockArgs{fn: clockEntry, id: int64(id)}
	result := indirectCall(trampoline, unsafe.Pointer(&args))
	return args.ts.Sec, args.ts.Nsec, result
}

// Read the file, not guessed memory preceding runtime.Func. The PC/line table
// survives -s -w; an anchor converts ELF addresses to process addresses for PIE.
// Selecting only asm_amd64.s excludes the identically named ABIInternal wrapper.
func resolveRuntimeBridge() (uintptr, error) {
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	text := f.Section(".text")
	if text == nil {
		return 0, fmt.Errorf("missing ELF text")
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
		return 0, err
	}
	table, err := gosym.NewTable(nil, gosym.NewLineTable(data, text.Addr))
	if err != nil {
		return 0, err
	}
	anchorPC := reflect.ValueOf(runtime.Gosched).Pointer()
	anchor := table.LookupFunc("runtime.Gosched")
	if anchor == nil {
		return 0, fmt.Errorf("missing relocation anchor")
	}
	bias := anchorPC - uintptr(anchor.Entry)
	var found uintptr
	for _, fn := range table.Funcs {
		if fn.Name != "runtime.asmcgocall" {
			continue
		}
		file, _, _ := table.PCToLine(fn.Entry)
		if !strings.HasSuffix(file, "/runtime/asm_amd64.s") && file != "runtime/asm_amd64.s" {
			continue
		}
		pc := uintptr(fn.Entry) + bias
		live := runtime.FuncForPC(pc)
		if live == nil || live.Entry() != pc || live.Name() != fn.Name {
			return 0, fmt.Errorf("runtime entry validation failed")
		}
		liveFile, _ := live.FileLine(pc)
		if liveFile != file {
			return 0, fmt.Errorf("runtime source validation failed")
		}
		if found != 0 {
			return 0, fmt.Errorf("ambiguous assembly entry")
		}
		found = pc
	}
	if found == 0 {
		return 0, fmt.Errorf("asmcgocall assembly entry not found")
	}
	return found, nil
}
