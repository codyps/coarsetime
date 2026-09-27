#!/usr/bin/env bash
# Compile first; measure sequentially, alternating versions and case order.
set -euo pipefail
cd "$(dirname "$0")"
artifacts=$(mktemp -d /tmp/coarsetime-direct-comparison-XXXXXX)
printf 'Artifacts: %s\n' "$artifacts"
versions=(go1.23.12 go1.26.6 go1.27.1)
for version in "${versions[@]}"; do
    GOTOOLCHAIN=$version CGO_ENABLED=0 go test -c -o "$artifacts/$version.test" .
    "$artifacts/$version.test" -test.run='^TestComparisonClocks$' -test.v
done
uname -sr
for round in {1..8}; do
    order=(0 1 2)
    if (( round % 2 == 0 )); then order=(2 1 0); fi
    for index in "${order[@]}"; do
        version=${versions[$index]}
        printf '\nROUND %s VERSION %s REVERSE %s\n' "$round" "$version" "$((round % 2))"
        COARSETIME_BENCH_REVERSE=$((round % 2)) "$artifacts/$version.test" \
            -test.run='^$' -test.bench='^BenchmarkBridgeComparison$' \
            -test.benchmem -test.benchtime=300ms -test.cpu=1
    done
done
GOTOOLCHAIN=go1.26.6 go tool objdump -s '(\.directRead|\.Indirect|\.indirectCall(\.abi0)?|runtime\.asmcgocall)$' \
    "$artifacts/go1.26.6.test" > "$artifacts/call-paths-go1.26.6.txt"
printf '\nDisassembly: %s/call-paths-go1.26.6.txt\n' "$artifacts"
