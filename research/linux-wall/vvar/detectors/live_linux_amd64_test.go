package vvardetectors

import (
	"coarsetime"
	"os"
	"testing"
	"time"
)

func TestLive(t *testing.T) {
	for _, method := range []string{"btf", "instructions"} {
		t.Run(method, func(t *testing.T) {
			c, e := Live(method)
			if e != nil {
				if os.Getenv("VVAR_REQUIRE_LIVE") == "1" {
					t.Fatal(e)
				}
				t.Skip(e)
			}
			t.Logf("active: %+v", c.Layout)
			for n := 0; n < 100000; n++ {
				before := coarsetime.Now()
				got := c.Now()
				after := coarsetime.Now()
				if !after.Before(before) && (got.Before(before) || got.After(after)) {
					t.Fatal("outside bracket")
				}
				if got != time.Unix(0, got.UnixNano()) {
					t.Fatal("time representation")
				}
			}
		})
	}
}

var sink time.Time

func BenchmarkCurrentNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		sink = coarsetime.Now()
	}
}
func BenchmarkBTFNow(b *testing.B)          { benchmark(b, "btf") }
func BenchmarkInstructionsNow(b *testing.B) { benchmark(b, "instructions") }
func benchmark(b *testing.B, method string) {
	c, e := Live(method)
	if e != nil {
		b.Fatal(e)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sink = c.Now()
	}
}
func TestReadFallback(t *testing.T) {
	seq := uint32(1)
	sec, ns := uint64(1), uint64(1)
	c := &Clock{seq: &seq, sec: &sec, nsec: &ns}
	for _, clock := range []*Clock{nil, c} {
		before := coarsetime.Now()
		got := clock.Now()
		after := coarsetime.Now()
		if !after.Before(before) && (got.Before(before) || got.After(after)) {
			t.Fatal("fallback")
		}
	}
}
