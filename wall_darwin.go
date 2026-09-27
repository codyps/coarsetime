//go:build !purego && darwin && (amd64 || arm64)

package coarsetime

import (
	"sync/atomic"
	"time"

	"github.com/codyps/coarsetime/internal/darwinwall"
)

// The new calendar layout is present in macOS releases supported by Go 1.23+.
// These are private XNU offsets; see research/darwin/calendar/README.md.
func darwinWallAddresses() (page *darwinwall.Page, approximate *atomic.Uint64, supported bool)

var darwinCalendarPage, darwinCalendarApproximate, darwinCalendarSupported = darwinWallAddresses()

func readWallUnixNano() int64 {
	if darwinCalendarSupported {
		if ns, ok := darwinwall.Read(darwinCalendarPage, darwinCalendarApproximate); ok {
			return ns
		}
	}
	return readDarwinWallFallback()
}

// The standard library lets libSystem handle expired or unavailable mappings.
// This is an on-demand wall read, not polling or a local correction refresh.
//
//go:noinline
func readDarwinWallFallback() int64 { return time.Now().UnixNano() }
