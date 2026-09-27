//go:build !purego

package coarsetime

import (
	"encoding/binary"
	"testing"
)

func TestSignalOffsets(t *testing.T) {
	if runtimeGMOffset == 0 || runtimeMSignalOffset == 0 {
		t.Fatal("signal-stack offsets were not resolved")
	}
	// Include a prologue instruction and vary offsets to ensure the decoder
	// extracts operands instead of assuming a particular runtime layout.
	words := []uint32{0xf81f0ffe, 0xf9400388 | 7<<10, 0xf9400103 | 11<<10, 0xeb03039f, 0x54000480}
	code := make([]byte, len(words)*4)
	for i, w := range words {
		binary.LittleEndian.PutUint32(code[4*i:], w)
	}
	gm, ms, err := decodeSignalOffsets(code)
	if err != nil || gm != 56 || ms != 88 {
		t.Fatalf("offsets = %d, %d, %v", gm, ms, err)
	}
	for _, bad := range [][]byte{nil, code[:len(code)-1], make([]byte, 64)} {
		if _, _, err := decodeSignalOffsets(bad); err == nil {
			t.Fatal("accepted unknown runtime code")
		}
	}
	code[4] ^= 1 // Different destination register must not be accepted.
	if _, _, err := decodeSignalOffsets(code); err == nil {
		t.Fatal("accepted changed register")
	}
}
