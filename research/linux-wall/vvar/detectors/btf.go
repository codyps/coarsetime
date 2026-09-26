package vvardetectors

import (
	"encoding/binary"
	"fmt"
)

type member struct {
	name     string
	typ, off uint32
}
type typ struct {
	name        string
	kind, size  uint32
	members     []member
	elem, count uint32
}
type types []typ

func parseBTF(data []byte) (types, error) {
	if len(data) < 24 || len(data) > 64<<20 {
		return nil, fmt.Errorf("BTF size")
	}
	u := binary.LittleEndian
	if u.Uint16(data) != 0xeb9f || data[2] != 1 {
		return nil, fmt.Errorf("BTF identity")
	}
	h := uint64(u.Uint32(data[4:]))
	to := h + uint64(u.Uint32(data[8:]))
	te := to + uint64(u.Uint32(data[12:]))
	so := h + uint64(u.Uint32(data[16:]))
	se := so + uint64(u.Uint32(data[20:]))
	if h < 24 || te > uint64(len(data)) || se > uint64(len(data)) || to > te || so > se {
		return nil, fmt.Errorf("BTF bounds")
	}
	str := data[so:se]
	name := func(n uint32) (string, error) {
		if uint64(n) >= uint64(len(str)) {
			return "", fmt.Errorf("string offset")
		}
		for j := int(n); j < len(str); j++ {
			if str[j] == 0 {
				return string(str[n:j]), nil
			}
		}
		return "", fmt.Errorf("string terminator")
	}
	ts := types{{}}
	for off := to; off < te; {
		if te-off < 12 || len(ts) > 1000000 {
			return nil, fmt.Errorf("BTF type bound")
		}
		b := data[off:te]
		info := u.Uint32(b[4:])
		kind := (info >> 24) & 31
		vlen := info & 65535
		size := u.Uint32(b[8:])
		n, err := name(u.Uint32(b))
		if err != nil {
			return nil, err
		}
		off += 12
		lens := map[uint32]uint64{1: 4, 2: 0, 3: 12, 4: uint64(vlen) * 12, 5: uint64(vlen) * 12, 6: uint64(vlen) * 8, 7: 0, 8: 0, 9: 0, 10: 0, 11: 0, 12: 0, 13: uint64(vlen) * 8, 14: 4, 15: uint64(vlen) * 12, 16: 0, 17: 4, 18: 0, 19: uint64(vlen) * 12}
		length, ok := lens[kind]
		if !ok || length > te-off {
			return nil, fmt.Errorf("type payload")
		}
		payload := data[off : off+length]
		t := typ{name: n, kind: kind, size: size}
		if kind == 4 || kind == 5 {
			for j := uint32(0); j < vlen; j++ {
				p := payload[j*12:]
				mn, e := name(u.Uint32(p))
				if e != nil {
					return nil, e
				}
				bits := u.Uint32(p[8:])
				if info>>31 != 0 {
					if bits>>24 != 0 {
						bits = 0xffffffff
					} else {
						bits &= 0xffffff
					}
				}
				t.members = append(t.members, member{mn, u.Uint32(p[4:]), bits})
			}
		}
		if kind == 3 {
			t.elem = u.Uint32(payload)
			t.count = u.Uint32(payload[8:])
		}
		ts = append(ts, t)
		off += length
	}
	return ts, nil
}
func (ts types) unwrap(id uint32) (uint32, error) {
	for n := 0; n < 32; n++ {
		if id == 0 || uint64(id) >= uint64(len(ts)) {
			return 0, fmt.Errorf("type reference")
		}
		k := ts[id].kind
		if k == 8 || k == 9 || k == 10 || k == 11 || k == 18 {
			id = ts[id].size
		} else {
			return id, nil
		}
	}
	return 0, fmt.Errorf("type cycle")
}
func (ts types) named(name string) (uint32, error) {
	var id uint32
	for j, t := range ts {
		if t.name == name && t.kind == 4 {
			if id != 0 {
				return 0, fmt.Errorf("ambiguous %s", name)
			}
			id = uint32(j)
		}
	}
	if id == 0 {
		return 0, fmt.Errorf("missing %s", name)
	}
	return id, nil
}
func (ts types) field(id uint32, name string, depth int) (member, error) {
	if depth > 16 {
		return member{}, fmt.Errorf("nested type bound")
	}
	id, err := ts.unwrap(id)
	if err != nil {
		return member{}, err
	}
	t := ts[id]
	if t.kind != 4 && t.kind != 5 {
		return member{}, fmt.Errorf("not aggregate")
	}
	var result member
	found := false
	for _, m := range t.members {
		var candidate member
		ok := false
		if m.name == name {
			candidate = m
			ok = true
		} else if m.name == "" {
			child, e := ts.field(m.typ, name, depth+1)
			if e == nil && m.off != 0xffffffff {
				candidate = child
				candidate.off += m.off
				ok = true
			}
		}
		if ok {
			if found || candidate.off == 0xffffffff || candidate.off%8 != 0 || uint64(candidate.off)/8 >= uint64(t.size) {
				return member{}, fmt.Errorf("ambiguous/invalid field")
			}
			result = candidate
			found = true
		}
	}
	if !found {
		return member{}, fmt.Errorf("missing field %s", name)
	}
	return result, nil
}
func (ts types) integer(id, width uint32) bool {
	id, e := ts.unwrap(id)
	return e == nil && ts[id].kind == 1 && ts[id].size == width
}
func DetectBTF(i *Image, data []byte) (Layout, error) {
	ts, e := parseBTF(data)
	if e != nil {
		return Layout{}, e
	}
	return detectBTFTypes(i, ts)
}

func detectBTFTypes(i *Image, ts types) (Layout, error) {
	clock, e := ts.named("vdso_clock")
	if e != nil {
		clock, e = ts.named("vdso_data")
	}
	if e != nil {
		return Layout{}, e
	}
	seq, e := ts.field(clock, "seq", 0)
	if e != nil {
		return Layout{}, e
	}
	base, e := ts.field(clock, "basetime", 0)
	if e != nil {
		return Layout{}, e
	}
	arr, e := ts.unwrap(base.typ)
	if e != nil {
		return Layout{}, e
	}
	a := ts[arr]
	if a.kind != 3 || a.count <= 5 || a.count > 128 {
		return Layout{}, fmt.Errorf("clock array")
	}
	elem, e := ts.unwrap(a.elem)
	if e != nil {
		return Layout{}, e
	}
	s, e := ts.field(elem, "sec", 0)
	if e != nil {
		return Layout{}, e
	}
	ns, e := ts.field(elem, "nsec", 0)
	if e != nil {
		return Layout{}, e
	}
	if !ts.integer(seq.typ, 4) || !ts.integer(s.typ, 8) || !ts.integer(ns.typ, 8) || ts[elem].size > 1024 {
		return Layout{}, fmt.Errorf("timestamp widths")
	}
	// BTF establishes offsets but not the userspace address. Interpret only the
	// fixed-ID dispatch prefix to locate its first 32-bit kernel-data read.
	// This narrower witness is not a proof of the full synchronization protocol.
	prefix, e := analyze(i, false, true)
	if e != nil {
		return Layout{}, e
	}
	record := prefix.Seq - int64(seq.off/8)
	secOff := uint64(base.off/8) + 5*uint64(ts[elem].size) + uint64(s.off/8)
	nsOff := uint64(base.off/8) + 5*uint64(ts[elem].size) + uint64(ns.off/8)
	if secOff == nsOff || secOff%8 != 0 || nsOff%8 != 0 || (seq.off/8)%4 != 0 || secOff+8 > uint64(ts[clock].size) || nsOff+8 > uint64(ts[clock].size) {
		return Layout{}, fmt.Errorf("timestamp outside record")
	}
	return Layout{prefix.Seq, record + int64(secOff), record + int64(nsOff)}, nil
}
