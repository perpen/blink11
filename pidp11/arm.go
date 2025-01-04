//go:build arm
// +build arm

package pidp11

import "syscall"

// time.Sleep() does not work for low durations
func nanosleep(ns int32) {
	ts := syscall.Timespec{Sec: 0, Nsec: ns}
	syscall.Nanosleep(&ts, nil)
}
