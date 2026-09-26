package vvardetectors

import (
	"fmt"
	"golang.org/x/arch/x86/x86asm"
)

// A bounded abstract interpreter, not native execution or concrete-value probing.
// Timestamp values stay symbolic. Unknown operations and predicates reject.
const origin uint64 = 0x100000
const stack uint64 = 0x200000
const output uint64 = 0x300000
const stop uint64 = 0x400000

type value struct {
	n   uint64
	tag string
}

func c(n uint64) value { return value{n: n} }

type machine struct {
	image                    *Image
	pc                       uint64
	regs                     map[x86asm.Reg]value
	mem                      map[uint64]value
	firstOnly, change        bool
	seq                      uint64
	haveSeq                  bool
	attempt, stage           int
	even, validated, retried bool
	layout                   Layout
	condition                string
	z, carry                 bool
}

func norm(r x86asm.Reg) (x86asm.Reg, int) {
	// x86asm declares each register-width bank contiguously.
	if r >= x86asm.RAX && r <= x86asm.R15 {
		return r, 64
	}
	if r >= x86asm.EAX && r <= x86asm.R15L {
		return x86asm.RAX + (r - x86asm.EAX), 32
	}
	if r >= x86asm.AX && r <= x86asm.R15W {
		return x86asm.RAX + (r - x86asm.AX), 16
	}
	switch r {
	case x86asm.AL:
		return x86asm.RAX, 8
	case x86asm.CL:
		return x86asm.RCX, 8
	case x86asm.DL:
		return x86asm.RDX, 8
	case x86asm.BL:
		return x86asm.RBX, 8
	}
	if r >= x86asm.SPB && r <= x86asm.R15B {
		return x86asm.RSP + (r - x86asm.SPB), 8
	}
	return 0, 0
}
func (m *machine) reg(r x86asm.Reg) value {
	rr, w := norm(r)
	v, ok := m.regs[rr]
	if !ok {
		return value{tag: fmt.Sprint("unknown:", r)}
	}
	if v.tag == "" && w < 64 {
		v.n &= (uint64(1) << w) - 1
	}
	return v
}
func (m *machine) put(r x86asm.Reg, v value) error {
	rr, w := norm(r)
	if w < 32 {
		return fmt.Errorf("partial register write")
	}
	if w == 32 && v.tag == "" {
		v.n = uint64(uint32(v.n))
	}
	m.regs[rr] = v
	return nil
}
func (m *machine) addr(a x86asm.Mem, next uint64) (uint64, error) {
	n := uint64(a.Disp)
	if a.Segment != 0 {
		return 0, fmt.Errorf("segment access")
	}
	if a.Base == x86asm.RIP {
		n = next + uint64(int64(int32(a.Disp)))
	} else if a.Base != 0 {
		v := m.reg(a.Base)
		if v.tag != "" {
			return 0, fmt.Errorf("symbolic base")
		}
		n += v.n
	}
	if a.Index != 0 {
		v := m.reg(a.Index)
		if v.tag != "" {
			return 0, fmt.Errorf("symbolic index")
		}
		n += v.n * uint64(a.Scale)
	}
	return n, nil
}
func (m *machine) get(a x86asm.Arg, next uint64, width int) (value, error) {
	switch a := a.(type) {
	case x86asm.Reg:
		return m.reg(a), nil
	case x86asm.Imm:
		return c(uint64(a)), nil
	case x86asm.Mem:
		addr, err := m.addr(a, next)
		if err != nil {
			return value{}, err
		}
		if addr >= origin-0x10000 && addr < origin {
			if !m.haveSeq {
				if width != 4 {
					return value{}, fmt.Errorf("first kernel read not 32 bit")
				}
				m.seq = addr
				m.haveSeq = true
				m.layout.Seq = int64(addr) - int64(origin)
			}
			if addr == m.seq {
				if width != 4 {
					return value{}, fmt.Errorf("sequence width")
				}
				if m.stage == 0 {
					m.attempt++
					m.stage = 1
					m.even = false
					m.validated = false
					return value{tag: fmt.Sprint("seq", m.attempt)}, nil
				}
				if m.stage == 3 {
					m.stage = 4
					return value{tag: fmt.Sprint("verify", m.attempt)}, nil
				}
				return value{}, fmt.Errorf("unexpected sequence load")
			}
			if !m.even || width != 8 {
				return value{}, fmt.Errorf("unguarded or unexpected data load")
			}
			if m.stage == 1 {
				m.stage = 2
				if m.attempt == 1 {
					m.layout.Sec = int64(addr) - int64(origin)
				} else if m.layout.Sec != int64(addr)-int64(origin) {
					return value{}, fmt.Errorf("retry address")
				}
				return value{tag: fmt.Sprint("sec", m.attempt)}, nil
			}
			if m.stage == 2 {
				m.stage = 3
				if m.attempt == 1 {
					m.layout.Nsec = int64(addr) - int64(origin)
				} else if m.layout.Nsec != int64(addr)-int64(origin) {
					return value{}, fmt.Errorf("retry address")
				}
				return value{tag: fmt.Sprint("nsec", m.attempt)}, nil
			}
			return value{}, fmt.Errorf("extra data read")
		}
		if v, ok := m.mem[addr]; ok {
			return v, nil
		}
		return value{}, fmt.Errorf("uninitialized memory %#x", addr)
	}
	return value{}, fmt.Errorf("unsupported operand")
}
func (m *machine) set(a x86asm.Arg, v value, next uint64) error {
	switch a := a.(type) {
	case x86asm.Reg:
		return m.put(a, v)
	case x86asm.Mem:
		addr, err := m.addr(a, next)
		if err != nil {
			return err
		}
		if !(addr >= stack-4096 && addr <= stack || addr == output || addr == output+8) {
			return fmt.Errorf("unexpected store")
		}
		m.mem[addr] = v
		return nil
	}
	return fmt.Errorf("unsupported destination")
}
func analyze(i *Image, change, firstOnly bool) (Layout, error) {
	m := machine{image: i, pc: origin + i.entry, regs: map[x86asm.Reg]value{}, mem: map[uint64]value{stack: c(stop)}, change: change, firstOnly: firstOnly}
	m.regs[x86asm.RDI] = c(5)
	m.regs[x86asm.RSI] = c(output)
	m.regs[x86asm.RSP] = c(stack)
	for steps := 0; steps < 256; steps++ {
		off := m.pc - origin - i.start
		if off >= uint64(len(i.text)) {
			return Layout{}, fmt.Errorf("branch outside text")
		}
		if len(i.text[off:]) >= 4 && string(i.text[off:off+4]) == "\xf3\x0f\x1e\xfa" {
			m.pc += 4
			continue
		}
		in, err := x86asm.Decode(i.text[off:], 64)
		if err != nil {
			return Layout{}, err
		}
		next := m.pc + uint64(in.Len)
		width := in.MemBytes
		get := func(a x86asm.Arg) (value, error) { return m.get(a, next, width) }
		fail := func(e error) (Layout, error) {
			return Layout{}, fmt.Errorf("pc %#x %s: %w", m.pc-origin, in.String(), e)
		}
		switch in.Op {
		case x86asm.NOP, x86asm.PAUSE:
		case x86asm.PUSH:
			v, e := get(in.Args[0])
			if e != nil {
				return fail(e)
			}
			sp := m.reg(x86asm.RSP).n - 8
			m.regs[x86asm.RSP] = c(sp)
			m.mem[sp] = v
		case x86asm.POP:
			sp := m.reg(x86asm.RSP).n
			v, ok := m.mem[sp]
			if !ok {
				return fail(fmt.Errorf("bad pop"))
			}
			if e := m.set(in.Args[0], v, next); e != nil {
				return fail(e)
			}
			m.regs[x86asm.RSP] = c(sp + 8)
		case x86asm.MOV, x86asm.MOVSXD:
			v, e := get(in.Args[1])
			if e != nil {
				return fail(e)
			}
			if in.Op == x86asm.MOVSXD {
				if v.tag != "" {
					return fail(fmt.Errorf("symbolic sign extension"))
				}
				v.n = uint64(int64(int32(v.n)))
			}
			if firstOnly && m.haveSeq {
				return m.layout, nil
			}
			if _, ok := in.Args[0].(x86asm.Mem); ok && width != 8 {
				return fail(fmt.Errorf("non-64-bit store"))
			}
			if e = m.set(in.Args[0], v, next); e != nil {
				return fail(e)
			}
		case x86asm.LEA:
			a, ok := in.Args[1].(x86asm.Mem)
			if !ok {
				return fail(fmt.Errorf("bad lea"))
			}
			n, e := m.addr(a, next)
			if e != nil {
				return fail(e)
			}
			if e = m.set(in.Args[0], c(n), next); e != nil {
				return fail(e)
			}
		case x86asm.XOR, x86asm.ADD, x86asm.SUB, x86asm.SHL, x86asm.AND:
			a, e := get(in.Args[0])
			if e != nil {
				return fail(e)
			}
			b, e := get(in.Args[1])
			if e != nil {
				return fail(e)
			}
			if in.Op == x86asm.XOR && in.Args[0] == in.Args[1] {
				a = c(0)
			} else {
				if a.tag != "" || b.tag != "" {
					return fail(fmt.Errorf("symbolic arithmetic"))
				}
				switch in.Op {
				case x86asm.XOR:
					a.n ^= b.n
				case x86asm.ADD:
					a.n += b.n
				case x86asm.SUB:
					a.n -= b.n
				case x86asm.SHL:
					a.n <<= b.n & 63
				case x86asm.AND:
					a.n &= b.n
				}
			}
			m.condition = ""
			if in.Op == x86asm.XOR || in.Op == x86asm.AND {
				m.condition = "constant"
			}
			m.z = a.n == 0
			m.carry = false
			if e = m.set(in.Args[0], a, next); e != nil {
				return fail(e)
			}
		case x86asm.TEST, x86asm.CMP:
			a, e := get(in.Args[0])
			if e != nil {
				return fail(e)
			}
			b, e := get(in.Args[1])
			if e != nil {
				return fail(e)
			}
			if in.Op == x86asm.TEST && a.tag == fmt.Sprint("seq", m.attempt) && b.tag == "" && b.n == 1 {
				m.condition = "even"
				m.z = true
			} else if in.Op == x86asm.CMP && ((a.tag == fmt.Sprint("seq", m.attempt) && b.tag == fmt.Sprint("verify", m.attempt)) || (b.tag == fmt.Sprint("seq", m.attempt) && a.tag == fmt.Sprint("verify", m.attempt))) {
				for _, operand := range in.Args[:2] {
					if r, ok := operand.(x86asm.Reg); ok {
						_, w := norm(r)
						if w != 32 {
							return fail(fmt.Errorf("sequence comparison width"))
						}
					} else if _, ok := operand.(x86asm.Mem); !ok || width != 4 {
						return fail(fmt.Errorf("sequence comparison width"))
					}
				}
				m.condition = "sequence"
				m.z = !change || m.attempt > 1
			} else if a.tag == "" && b.tag == "" {
				m.condition = "constant"
				if in.Op == x86asm.TEST {
					m.z = a.n&b.n == 0
					m.carry = false
				} else {
					m.z = a.n == b.n
					m.carry = a.n < b.n
				}
			} else {
				return fail(fmt.Errorf("unknown predicate %v %v", a, b))
			}
		case x86asm.JMP, x86asm.JE, x86asm.JNE, x86asm.JA, x86asm.JAE, x86asm.JB, x86asm.JBE:
			rel, ok := in.Args[0].(x86asm.Rel)
			if !ok {
				return fail(fmt.Errorf("indirect branch"))
			}
			take := true
			if in.Op != x86asm.JMP {
				if m.condition == "" {
					return fail(fmt.Errorf("unknown flags"))
				}
				if m.condition != "constant" && in.Op != x86asm.JE && in.Op != x86asm.JNE {
					return fail(fmt.Errorf("unexpected symbolic condition"))
				}
				switch in.Op {
				case x86asm.JE:
					take = m.z
				case x86asm.JNE:
					take = !m.z
				case x86asm.JA:
					take = !m.z && !m.carry
				case x86asm.JAE:
					take = !m.carry
				case x86asm.JB:
					take = m.carry
				case x86asm.JBE:
					take = m.z || m.carry
				}
				if m.condition == "even" {
					m.even = true
				}
				if m.condition == "sequence" {
					if m.z {
						m.validated = true
					} else {
						m.stage = 0
						m.retried = true
					}
				}
				m.condition = ""
			}
			if take {
				next = uint64(int64(next) + int64(rel))
			}
		case x86asm.RET:
			if !m.validated || m.stage != 4 || m.reg(x86asm.RAX) != c(0) || m.mem[output].tag != fmt.Sprint("sec", m.attempt) || m.mem[output+8].tag != fmt.Sprint("nsec", m.attempt) || m.mem[m.reg(x86asm.RSP).n] != c(stop) || change && !m.retried {
				return fail(fmt.Errorf("unverified return"))
			}
			return m.layout, nil
		default:
			return fail(fmt.Errorf("unsupported instruction"))
		}
		m.pc = next
	}
	return Layout{}, fmt.Errorf("instruction budget")
}
func DetectInstructions(i *Image) (Layout, error) {
	a, err := analyze(i, false, false)
	if err != nil {
		return Layout{}, err
	}
	b, err := analyze(i, true, false)
	if err != nil {
		return Layout{}, err
	}
	if a != b {
		return Layout{}, fmt.Errorf("retry layout differs")
	}
	if a.Seq >= 0 || a.Sec >= 0 || a.Nsec >= 0 || a.Seq%4 != 0 || a.Sec%8 != 0 || a.Nsec%8 != 0 || a.Sec == a.Nsec || (a.Seq >= a.Sec && a.Seq < a.Sec+8) || (a.Seq >= a.Nsec && a.Seq < a.Nsec+8) {
		return Layout{}, fmt.Errorf("overlapping/unaligned fields")
	}
	return a, nil
}
