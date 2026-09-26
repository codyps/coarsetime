package vvardetectors

import (
	"debug/elf"
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusInstructions(t *testing.T) {
	paths, e := filepath.Glob("../corpus/*/vdso.so")
	if e != nil || len(paths) != 6 {
		t.Fatal("corpus unavailable")
	}
	for _, p := range paths {
		t.Run(filepath.Base(filepath.Dir(p)), func(t *testing.T) {
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			i, e := ParseImage(b)
			if e != nil {
				t.Fatal(e)
			}
			l, e := DetectInstructions(i)
			if e != nil {
				t.Fatal(e)
			}
			t.Logf("layout %+v", l)
		})
	}
}

func TestBTFImages(t *testing.T) {
	cache := os.Getenv("VVAR_KERNEL_CACHE")
	if cache == "" {
		t.Skip("set VVAR_KERNEL_CACHE to the corpus download cache")
	}
	names := []string{"ubuntu-5.15", "ubuntu-6.8", "debian-6.1", "debian-6.12", "fedora-43"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			f, e := os.Open(filepath.Join(cache, name, "vmlinux"))
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			ef, e := elf.NewFile(f)
			if e != nil {
				t.Fatal(e)
			}
			section := ef.Section(".BTF")
			if section == nil {
				t.Fatal("BTF absent")
			}
			btf, e := section.Data()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := os.ReadFile(filepath.Join("../corpus", name, "vdso.so"))
			if e != nil {
				t.Fatal(e)
			}
			i, e := ParseImage(raw)
			if e != nil {
				t.Fatal(e)
			}
			got, e := DetectBTF(i, btf)
			if e != nil {
				t.Fatal(e)
			}
			want, e := DetectInstructions(i)
			if e != nil || got != want {
				t.Fatalf("BTF %+v, instructions %+v, %v", got, want, e)
			}
			t.Logf("BTF matches instruction layout %+v", got)
			ts, e := parseBTF(btf)
			if e != nil {
				t.Fatal(e)
			}
			_ = ts
			bad := append([]byte(nil), btf...)
			bad[0] ^= 1
			if _, e := DetectBTF(i, bad); e == nil {
				t.Fatal("bad BTF identity accepted")
			}
		})
	}
}

func TestInstructionRejection(t *testing.T) {
	b, e := os.ReadFile("../corpus/arch/vdso.so")
	if e != nil {
		t.Fatal(e)
	}
	original, e := ParseImage(b)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name     string
		address  uint64
		old, new byte
	}{
		{"missing even guard", 0xf41, 1, 0},
		{"compare wrong sequence", 0xf59, 0x10, 0x08},
		{"byte sequence comparison", 0xf58, 0x39, 0x38},
		{"alias sec and nsec", 0xf52, 0x10, 0x08},
		{"return on changed sequence", 0xf5b, 0x84, 0x85},
		{"overwrite seconds output", 0xf56, 8, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := *original
			i.text = append([]byte(nil), original.text...)
			off := tc.address - i.start
			if i.text[off] != tc.old {
				t.Fatal("fixture changed")
			}
			i.text[off] = tc.new
			if l, e := DetectInstructions(&i); e == nil {
				t.Fatalf("accepted mutation: %+v", l)
			}
		})
	}
	i := *original
	i.text = i.text[:8]
	if _, e := DetectInstructions(&i); e == nil {
		t.Fatal("truncated accepted")
	}
}

func TestBTFRejection(t *testing.T) {
	raw, e := os.ReadFile("../corpus/arch/vdso.so")
	if e != nil {
		t.Fatal(e)
	}
	i, e := ParseImage(raw)
	if e != nil {
		t.Fatal(e)
	}
	fixture := func() types {
		return types{{}, {kind: 1, size: 4}, {kind: 1, size: 8}, {kind: 4, name: "vdso_timestamp", size: 16, members: []member{{"sec", 2, 0}, {"nsec", 2, 64}}}, {kind: 3, elem: 3, count: 12}, {kind: 4, name: "vdso_clock", size: 232, members: []member{{"seq", 1, 0}, {"basetime", 4, 320}}}}
	}
	want, e := DetectInstructions(i)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := detectBTFTypes(i, fixture()); e != nil || got != want {
		t.Fatalf("valid synthetic metadata: %+v %v", got, e)
	}
	cases := []struct {
		name   string
		mutate func(types)
	}{
		{"wrong seq width", func(ts types) { ts[1].size = 8 }},
		{"unaligned seq", func(ts types) { ts[5].members[0].off = 1 }},
		{"missing clock index", func(ts types) { ts[4].count = 5 }},
		{"oversized stride", func(ts types) { ts[3].size = 1024 }},
		{"bad reference", func(ts types) { ts[4].elem = 99 }},
		{"alias fields", func(ts types) { ts[3].members[1].off = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := fixture()
			tc.mutate(ts)
			if l, e := detectBTFTypes(i, ts); e == nil {
				t.Fatalf("accepted %+v", l)
			}
		})
	}
	for _, bad := range [][]byte{nil, make([]byte, 24), []byte("invalid BTF")} {
		if _, e := DetectBTF(i, bad); e == nil {
			t.Fatal("malformed accepted")
		}
	}
}
