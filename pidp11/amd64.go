//go:build amd64
// +build amd64

package pidp11

import "syscall"

// time.Sleep() does not work for low durations
func nanosleep(ns int64) {
	// Only compiles with 32bit
	ts := syscall.Timespec{Sec: 0, Nsec: ns}
	//ts := syscall.Timespec{Sec: 0, Nsec: int64(ns)}
	syscall.Nanosleep(&ts, nil)
}
