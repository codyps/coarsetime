//go:build vvarprototype

package coarsetime

import (
	"os"
	"testing"
)

func TestVvarPrototypeRecognition(t *testing.T) {
	for _, image := range [][]byte{nil, []byte("not ELF"), make([]byte, 4096)} {
		if _, ok := vvarPrototypeRecognize(image); ok {
			t.Fatal("malformed image recognized")
		}
	}
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Fatal(err)
	}
	base, size := vdsoMapping(string(maps))
	if base == 0 {
		t.Skip("no vDSO")
	}
	mem, err := os.Open("/proc/self/mem")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	image := make([]byte, int(size))
	if _, err := mem.ReadAt(image, int64(base)); err != nil {
		t.Fatal(err)
	}
	delta, ok := vvarPrototypeRecognize(image)
	if !ok {
		t.Skip("kernel code has no audited profile")
	}
	if delta != -0x6000 {
		t.Fatal("wrong data target")
	}
	// Every byte of the audited function contributes to selection. Checking
	// only a convenient instruction substring would fail this rejection test.
	for pos := 0xaf0; pos < 0xaf0+1208; pos++ {
		image[pos] ^= 1
		if _, ok := vvarPrototypeRecognize(image); ok {
			t.Fatalf("mutation at %#x accepted", pos)
		}
		image[pos] ^= 1
	}
	for _, pos := range []int{4, 5, 18} {
		image[pos] ^= 1
		if _, ok := vvarPrototypeRecognize(image); ok {
			t.Fatalf("ELF identity mutation at %d accepted", pos)
		}
		image[pos] ^= 1
	}
	t.Log("recognized full function; rejected all 1208 single-byte code mutations")
}
