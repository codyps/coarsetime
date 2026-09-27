//go:build linux && amd64 && (!go1.27 || go1.28 || goexperiment.runtimesecret)

package bridge

const dedicatedAvailable = false

func dedicatedOffsets(out *[7]uintptr) { *out = [7]uintptr{} }

func Dedicated(id int32) (int64, int64, int32) {
	panic("dedicated bridge layout not audited for this toolchain")
}
