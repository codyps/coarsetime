//go:build linux && amd64 && go1.27 && !go1.28 && !goexperiment.runtimesecret

package bridge

// Only audited for Go 1.27 linux/amd64, without the runtimesecret experiment.
//
//go:noescape
func dedicatedCall(fn uintptr, id int32) (sec int64, nsec int64, result int32)

func Dedicated(id int32) (int64, int64, int32) {
	return dedicatedCall(clockEntry, id)
}

const dedicatedAvailable = true

//go:noescape
func dedicatedOffsets(out *[7]uintptr)
