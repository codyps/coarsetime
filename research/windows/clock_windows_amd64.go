package windowsbench

import "time"

// Experimental Windows/amd64 shared-page access, not a public package API.
// The addresses and atomic 64-bit reads match Go's runtime/time_windows.h
// and runtime/time_windows_amd64.s. Other architectures need separate review.
func sharedFiletime() uint64
func sharedInterrupt() uint64
func sharedUnixNano() int64

const filetimeEpoch = 116444736000000000

func filetimeToTime(ticks uint64) time.Time {
	ns := int64(ticks-filetimeEpoch) * 100
	return time.Unix(ns/1e9, ns%1e9)
}

func sharedNow() time.Time { return filetimeToTime(sharedFiletime()) }
