//go:build vvarprototype

package coarsetime

import (
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// EXPERIMENT ONLY: select the upstream x86-64 6.13–7.0 first clock-record
// layout explicitly, or recognize one audited vDSO code profile with detect.
// The struct starts at the beginning of the VVAR time page. Namespace pages
// have odd seq and are rejected by the bounded reader, falling back to Now.
type vvarPrototypeClock struct {
	seq     uint32
	padding [116]byte
	sec     uint64
	nsec    uint64
}

func vvarPrototypeAddress(base uintptr) *vvarPrototypeClock
func vvarPrototypeASM(p *vvarPrototypeClock) (sec, nsec int64, ok bool)

var vvarPrototypePointer = vvarPrototypeInit()

func vvarPrototypeInit() *vvarPrototypeClock {
	mode := os.Getenv("COARSETIME_VVAR_LAYOUT")
	if mode != "modern" && mode != "detect" {
		return nil
	}
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		return nil
	}
	var detected uintptr
	if mode == "detect" {
		detected = vvarPrototypeDetect(string(maps))
		if detected == 0 {
			return nil
		}
	}
	for _, line := range strings.Split(string(maps), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 6 || fields[5] != "[vvar]" || fields[1] != "r--p" {
			continue
		}
		lo, hi, ok := strings.Cut(fields[0], "-")
		if !ok {
			continue
		}
		base, e1 := strconv.ParseUint(lo, 16, 64)
		end, e2 := strconv.ParseUint(hi, 16, 64)
		if e1 != nil || e2 != nil || base == 0 || base%4096 != 0 || end <= base || end-base < 4096 {
			return nil
		}
		if mode == "detect" && uintptr(base) != detected {
			return nil
		}
		p := vvarPrototypeAddress(uintptr(base))
		// A sanity check for this opted-in experiment, NOT proof of ABI identity.
		for n := 0; n < 100; n++ {
			before := readRealtimeSyscall()
			sec, nsec, ok := vvarPrototypeRead(p)
			after := readRealtimeSyscall()
			if !ok {
				return nil
			}
			got := sec*1e9 + nsec
			if after >= before && (got < before || got > after) {
				return nil
			}
		}
		return p
	}
	return nil
}

func vvarPrototypeRead(p *vvarPrototypeClock) (sec, nsec int64, ok bool) {
	if p == nil {
		return 0, 0, false
	}
	for n := 0; n < 8; n++ {
		seq := atomic.LoadUint32(&p.seq)
		if seq&1 != 0 {
			continue
		}
		s := atomic.LoadUint64(&p.sec)
		ns := atomic.LoadUint64(&p.nsec)
		if seq == atomic.LoadUint32(&p.seq) && ns < 1e9 {
			return int64(s), int64(ns), true
		}
	}
	return 0, 0, false
}
func vvarPrototypeNow() time.Time {
	sec, nsec, ok := vvarPrototypeRead(vvarPrototypePointer)
	if !ok {
		return Now()
	}
	return time.Unix(sec, nsec)
}
func vvarPrototypeUnixNano() int64 {
	sec, nsec, ok := vvarPrototypeRead(vvarPrototypePointer)
	if !ok {
		return UnixNano()
	}
	return sec*1e9 + nsec
}
func vvarPrototypeASMNow() time.Time {
	sec, nsec, ok := vvarPrototypeASM(vvarPrototypePointer)
	if !ok {
		return Now()
	}
	return time.Unix(sec, nsec)
}
func vvarPrototypeASMUnixNano() int64 {
	sec, nsec, ok := vvarPrototypeASM(vvarPrototypePointer)
	if !ok {
		return UnixNano()
	}
	return sec*1e9 + nsec
}
