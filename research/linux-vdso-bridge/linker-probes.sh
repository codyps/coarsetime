#!/usr/bin/env bash
set -euo pipefail
work=$(mktemp -d /tmp/coarsetime-linker-probes-XXXXXX)
printf 'Probe directory: %s\n' "$work"
go version
for mode in linkname assembly address; do
    mkdir "$work/$mode"
    cat > "$work/$mode/go.mod" <<'MOD'
module example.com/bridgeprobe

go 1.23
MOD
    if [[ $mode == linkname ]]; then
        cat > "$work/$mode/main.go" <<'GO'
package main
import "unsafe"
//go:linkname bridge runtime.asmcgocall
func bridge(fn, arg unsafe.Pointer) int32
func main() { bridge(nil, nil) }
GO
        touch "$work/$mode/probe.s"
    elif [[ $mode == assembly ]]; then
        cat > "$work/$mode/main.go" <<'GO'
package main
import "unsafe"
func bridge(fn, arg unsafe.Pointer) int32
func main() { bridge(nil, nil) }
GO
        cat > "$work/$mode/probe.s" <<'ASM'
#include "textflag.h"
TEXT ·bridge(SB),NOSPLIT|NOFRAME,$0-20
    JMP runtime·asmcgocall(SB)
ASM
    else
        cat > "$work/$mode/main.go" <<'GO'
package main
func bridge() uintptr
func main() { println(bridge()) }
GO
        cat > "$work/$mode/probe.s" <<'ASM'
#include "textflag.h"
TEXT ·bridge(SB),NOSPLIT,$0-8
    MOVQ $runtime·asmcgocall(SB), AX
    MOVQ AX, ret+0(FP)
    RET
ASM
    fi
    printf '\nCASE %s (expected link failure on Go 1.27)\n' "$mode"
    if CGO_ENABLED=0 go -C "$work/$mode" build -o "$work/$mode.test" .; then
        echo 'UNEXPECTED BUILD SUCCESS'
        exit 1
    fi
    printf '\nCASE %s with -checklinkname=0 (build only)\n' "$mode"
    CGO_ENABLED=0 go -C "$work/$mode" build -ldflags=-checklinkname=0 -o "$work/$mode.test" .
done
