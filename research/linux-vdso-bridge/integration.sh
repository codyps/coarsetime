#!/usr/bin/env bash
# Exercise the adopted indirect bridge through the production API in a clean
# temporary source directory. Run from the Nix development shell.
set -euo pipefail
research=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$research/../.." && pwd)
work=$(mktemp -d /tmp/coarsetime-bridge-integration-XXXXXX)
printf 'Integration directory: %s\n' "$work"
cp "$root"/*.go "$root"/*.s "$root/go.mod" "$work/"
cp -R "$root/internal" "$work/"
cat > "$work/research_require_linux_amd64.go" <<'GO'
//go:build !purego

package coarsetime
func init() {
    if runtimeAsmcgocall == 0 || coarseVDSO == 0 { panic("research requires a working runtime bridge and vDSO") }
}
GO
cd "$work"
go version
CGO_ENABLED=0 COARSETIME_REQUIRE_VDSO=1 go test -v ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=1 COARSETIME_REQUIRE_VDSO=1 go test -race ./...
CGO_ENABLED=0 COARSETIME_REQUIRE_VDSO=1 go test -buildmode=pie -ldflags='-s -w' -trimpath ./...
CGO_ENABLED=0 go test -c -o "$work/integration.test" .
printf 'Integration binary: %s/integration.test\n' "$work"
