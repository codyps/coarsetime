//go:build !purego

package coarsetime

import (
	"encoding/binary"
	"testing"
)

func TestSignalOffsets(t *testing.T) {
	if runtimeGMOffset == 0 || runtimeMSignalOffset == 0 || runtimeVDSOPCOffset == 0 || runtimeVDSOSPOffset == 0 {
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

func TestVDSOTracebackOffsets(t *testing.T) {
	words := []uint32{0xf94002a2 | 113<<10, 0xf94002a3 | 112<<10,
		0xf90007e2, 0xf9000be3, 0x9100c3e2,
		0xf90002be | 113<<10, 0xf90002a2 | 112<<10}
	code := make([]byte, len(words)*4)
	for i, w := range words {
		binary.LittleEndian.PutUint32(code[4*i:], w)
	}
	pc, sp, err := decodeVDSOOffsets(code)
	if err != nil || pc != 904 || sp != 896 {
		t.Fatalf("offsets = %d, %d, %v", pc, sp, err)
	}
	for _, bad := range [][]byte{nil, code[:len(code)-1], make([]byte, 128)} {
		if _, _, err := decodeVDSOOffsets(bad); err == nil {
			t.Fatal("accepted unknown traceback code")
		}
	}
	code[21] ^= 4 // Store no longer matches the corresponding load's offset.
	if _, _, err := decodeVDSOOffsets(code); err == nil {
		t.Fatal("accepted mismatched traceback fields")
	}
}
