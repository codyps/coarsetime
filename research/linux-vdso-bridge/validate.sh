#!/usr/bin/env bash
# Run inside the repository's Nix shell, with GOTOOLCHAIN selecting the version.
set -euo pipefail
cd "$(dirname "$0")"
artifacts=$(mktemp -d /tmp/coarsetime-bridge-validation-XXXXXX)
printf 'Artifacts: %s\n' "$artifacts"
go version
go env GOOS GOARCH GOEXPERIMENT
uname -sr
export CGO_ENABLED=0
run_case() {
    local name=$1
    shift
    printf '\nCASE %s\n' "$name"
    go test -c "$@" -o "$artifacts/$name.test" .
    if [[ $name == default ]]; then
        "$artifacts/$name.test" -test.run='^TestDedicatedLayout$' -test.v
    fi
    "$artifacts/$name.test" -test.v
}
run_case default
run_case stripped -ldflags='-s -w'
run_case pie -buildmode=pie
run_case pie-stripped -buildmode=pie -ldflags='-s -w' -trimpath
run_case checkptr -gcflags=all=-d=checkptr=2
printf '\nCASE vet\n'
go vet ./...
printf '\nCASE race (race detector requires CGO_ENABLED=1)\n'
CGO_ENABLED=1 go test -race -v ./...
printf '\nCASE benchmark\n'
"$artifacts/default.test" -test.run='^$' -test.bench=. -test.benchmem -test.benchtime=300ms -test.count=5
