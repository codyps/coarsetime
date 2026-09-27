package bridge

import (
	"debug/dwarf"
	"debug/elf"
	"testing"
)

// This checks the assembled constants against THIS executable's runtime DWARF,
// not a hand-written copy of the runtime structs. Run at least one unstripped
// -c build: go test's default temporary executable can omit DWARF.
func TestDedicatedLayout(t *testing.T) {
	if !dedicatedAvailable {
		t.Skip("dedicated bridge not enabled")
	}
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := f.DWARF()
	if err != nil {
		t.Skipf("DWARF unavailable (stripped test): %v", err)
	}
	keys := []string{"runtime.g.m", "runtime.g.sched", "runtime.gobuf.sp", "runtime.m.g0", "runtime.m.curg", "runtime.m.vdsoSP", "runtime.m.vdsoPC"}
	var values [7]uintptr
	dedicatedOffsets(&values)
	want := make(map[string]int64)
	for i, key := range keys {
		want[key] = int64(values[i])
	}
	r := d.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if e == nil {
			break
		}
		name, _ := e.Val(dwarf.AttrName).(string)
		if e.Tag != dwarf.TagStructType || (name != "runtime.g" && name != "runtime.m" && name != "runtime.gobuf") {
			continue
		}
		typ, err := d.Type(e.Offset)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range typ.(*dwarf.StructType).Field {
			key := name + "." + field.Name
			expected, ok := want[key]
			if !ok {
				continue
			}
			if field.ByteOffset != expected {
				t.Fatalf("%s offset %d, bridge assumes %d", key, field.ByteOffset, expected)
			}
			delete(want, key)
		}
	}
	if len(want) != 0 {
		t.Fatalf("unverified offsets: %v", want)
	}
}
