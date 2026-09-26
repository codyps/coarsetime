package vvardetectors

import (
	"bytes"
	"debug/elf"
	"fmt"
)

type Layout struct{ Seq, Sec, Nsec int64 }
type Image struct {
	file         *elf.File
	text         []byte
	start, entry uint64
}

func ParseImage(data []byte) (*Image, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || f.Machine != elf.EM_X86_64 || f.Type != elf.ET_DYN {
		return nil, fmt.Errorf("not x86-64 vDSO")
	}
	t := f.Section(".text")
	if t == nil || t.Flags&elf.SHF_EXECINSTR == 0 {
		return nil, fmt.Errorf("no executable text")
	}
	text, err := t.Data()
	if err != nil || len(text) > 1<<20 {
		return nil, fmt.Errorf("invalid text")
	}
	syms, err := f.DynamicSymbols()
	if err != nil {
		return nil, err
	}
	for _, s := range syms {
		if s.Name == "__vdso_clock_gettime" && s.HasVersion && s.Version == "LINUX_2.6" && elf.ST_TYPE(s.Info) == elf.STT_FUNC && s.Value >= t.Addr && s.Value-t.Addr < uint64(len(text)) {
			return &Image{f, text, t.Addr, s.Value}, nil
		}
	}
	return nil, fmt.Errorf("no versioned clock entry")
}
