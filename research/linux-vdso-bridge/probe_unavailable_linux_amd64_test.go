//go:build !go1.27 || go1.28 || goexperiment.runtimesecret

package bridge

import "testing"

func checkDedicatedStack(t *testing.T) {}
