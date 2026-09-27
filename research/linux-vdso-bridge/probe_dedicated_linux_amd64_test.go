//go:build go1.27 && !go1.28 && !goexperiment.runtimesecret

package bridge

import "testing"

func checkDedicatedStack(t *testing.T) {
	s, ns, result := dedicatedCall(stackProbeAddress(), 5)
	if result != 0 || s != 123 || ns != 456 {
		t.Fatalf("dedicated stack probe: %d %d %d", s, ns, result)
	}
}
