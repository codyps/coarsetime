//go:build go1.27 && !go1.28 && !goexperiment.runtimesecret

// Adapted from Go's runtime/sys_linux_amd64.s nanotime1 (Go 1.27.1).
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by the BSD-style license in GO-LICENSE.
#include "textflag.h"

// Verified from DWARF in a Go 1.27.1 linux/amd64 executable. These offsets are
// private runtime implementation details. See layout_test.go and README.md.
#define g_m 48
#define g_sched_sp 56
#define m_g0 0
#define m_curg 184
#define m_vdsoSP 896
#define m_vdsoPC 904

// Expose the actual assembled constants to the DWARF verification test.
TEXT ·dedicatedOffsets(SB),NOSPLIT,$0-8
    MOVQ out+0(FP), AX
    MOVQ $g_m, 0(AX)
    MOVQ $g_sched_sp, 8(AX)
    MOVQ $0, 16(AX)
    MOVQ $m_g0, 24(AX)
    MOVQ $m_curg, 32(AX)
    MOVQ $m_vdsoSP, 40(AX)
    MOVQ $m_vdsoPC, 48(AX)
    RET

// A specialized non-callback bridge. No entersyscall, heap arguments, linkname,
// or foreign call on the goroutine stack. The only target is clock_gettime vDSO.
TEXT ·dedicatedCall(SB),NOSPLIT,$16-36
    MOVQ SP, R12
    MOVQ TLS, CX
    MOVQ 0(CX)(TLS*1), AX
    MOVQ g_m(AX), BX

    MOVQ m_vdsoPC(BX), CX
    MOVQ m_vdsoSP(BX), DX
    MOVQ CX, 0(SP)
    MOVQ DX, 8(SP)
    LEAQ fn+0(FP), DX
    MOVQ -8(DX), CX
    MOVQ CX, m_vdsoPC(BX)
    MOVQ DX, m_vdsoSP(BX)

    // Load incoming arguments before switching SP.
    MOVQ fn+0(FP), R11
    MOVL id+8(FP), DI
    CMPQ AX, m_curg(BX)
    JNE noswitch
    MOVQ m_g0(BX), DX
    MOVQ g_sched_sp(DX), SP
noswitch:
    SUBQ $16, SP
    ANDQ $~15, SP
    MOVQ $0, 0(SP)
    MOVQ $0, 8(SP)
    LEAQ 0(SP), SI
    CALL R11
    MOVQ 0(SP), CX
    MOVQ 8(SP), DX
    MOVQ R12, SP

    // Restore the same profiling state as the runtime's own vDSO path.
    MOVQ 8(SP), SI
    MOVQ SI, m_vdsoSP(BX)
    MOVQ 0(SP), SI
    MOVQ SI, m_vdsoPC(BX)
    MOVQ CX, sec+16(FP)
    MOVQ DX, nsec+24(FP)
    MOVL AX, result+32(FP)
    RET
