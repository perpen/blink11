package lib

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

var exitHook func(string)

func SetExitHook(f func(string)) {
	exitHook = f
}

// Invokes slog.Error() then prints a stack trace.
// Then calls exit hook, or panics.
func Assert(b bool, format string, args ...any) {
	if !b {
		slog.Error("assertion failed: "+format, args...)
		// xxx how to get the string?
		if exitHook == nil {
			panic(fmt.Errorf("assertion failed"))
		}
		debug.PrintStack()
		exitHook("assertion failed")
	}
}

// Same as Assert, without stack trace
func Check(b bool, format string, args ...any) {
	if !b {
		slog.Error("check failed: "+format, args...)
		if exitHook == nil {
			panic(fmt.Errorf("check failed"))
		}
		exitHook("check failed")
	}
}

type Timer struct {
	name  string
	start time.Time
}

func StartTimer(name string) Timer {
	slog.Info("timer " + name + " starting")
	return Timer{
		name:  name,
		start: time.Now(),
	}
}

func (timer Timer) Stop() {
	Assert(!timer.start.IsZero(), "timer.Stop called twice")
	slog.Info("timer " + timer.name, "duration", time.Now().Sub(timer.start))
	timer.start = time.Time{}
}
