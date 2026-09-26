package vvardetectors

import (
	"github.com/codyps/coarsetime"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

func mappedAddress(addr uintptr) (ret unsafe.Pointer)

type Clock struct {
	seq       *uint32
	sec, nsec *uint64
	Layout    Layout
}

func (c *Clock) Now() time.Time {
	if c != nil {
		for n := 0; n < 8; n++ {
			seq := atomic.LoadUint32(c.seq)
			if seq&1 != 0 {
				continue
			}
			sec := atomic.LoadUint64(c.sec)
			ns := atomic.LoadUint64(c.nsec)
			if seq == atomic.LoadUint32(c.seq) && ns < 1e9 {
				return time.Unix(int64(sec), int64(ns))
			}
		}
	}
	return coarsetime.Now()
}
func Live(method string) (*Clock, error) {
	maps, e := os.ReadFile("/proc/self/maps")
	if e != nil {
		return nil, e
	}
	var vdso, end, vvar, vend uint64
	for _, line := range strings.Split(string(maps), "\n") {
		f := strings.Fields(line)
		if len(f) != 6 {
			continue
		}
		lo, hi, ok := strings.Cut(f[0], "-")
		if !ok {
			continue
		}
		l, a := strconv.ParseUint(lo, 16, 64)
		h, b := strconv.ParseUint(hi, 16, 64)
		if a != nil || b != nil || h <= l {
			continue
		}
		if f[5] == "[vdso]" && f[1] == "r-xp" {
			vdso, end = l, h
		}
		if f[5] == "[vvar]" && f[1] == "r--p" {
			vvar, vend = l, h
		}
	}
	if vdso == 0 || vvar == 0 || end-vdso > 1<<20 {
		return nil, fmt.Errorf("missing/bad mappings")
	}
	mem, e := os.Open("/proc/self/mem")
	if e != nil {
		return nil, e
	}
	defer mem.Close()
	buf := make([]byte, end-vdso)
	if _, e = mem.ReadAt(buf, int64(vdso)); e != nil {
		return nil, e
	}
	i, e := ParseImage(buf)
	if e != nil {
		return nil, e
	}
	var l Layout
	switch method {
	case "instructions":
		l, e = DetectInstructions(i)
	case "btf":
		var b []byte
		b, e = os.ReadFile("/sys/kernel/btf/vmlinux")
		if e == nil {
			l, e = DetectBTF(i, b)
		}
	default:
		e = fmt.Errorf("unknown method")
	}
	if e != nil {
		return nil, e
	}
	addresses := []uint64{}
	for n, off := range []int64{l.Seq, l.Sec, l.Nsec} {
		width := uint64(8)
		if n == 0 {
			width = 4
		}
		if off >= 0 || uint64(-off) > vdso {
			return nil, fmt.Errorf("bad offset")
		}
		p := vdso - uint64(-off)
		if p < vvar || p > vend || vend-p < width || p%width != 0 {
			return nil, fmt.Errorf("address outside VVAR")
		}
		addresses = append(addresses, p)
	}
	c := &Clock{(*uint32)(mappedAddress(uintptr(addresses[0]))), (*uint64)(mappedAddress(uintptr(addresses[1]))), (*uint64)(mappedAddress(uintptr(addresses[2]))), l}
	// Diagnostic sanity check, not the evidence used to derive the layout.
	if atomic.LoadUint32(c.seq)&1 != 0 {
		return nil, fmt.Errorf("odd sequence/namespace page: fallback")
	}
	for n := 0; n < 100; n++ {
		before := coarsetime.Now()
		got := c.Now()
		after := coarsetime.Now()
		if !after.Before(before) && (got.Before(before) || got.After(after)) {
			return nil, fmt.Errorf("clock bracket mismatch")
		}
	}
	return c, nil
}
