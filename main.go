package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	cfg := Parse()
	InitLogging(cfg)

	if err := raisePriority(); err != nil {
		panic(err)
	}

	// xxx capture exit signals to ensure pin is reverted to input on exit.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	done := make(chan bool, 0)
	mgr := NewManager(cfg, done)
	mgr.Start()
	defer mgr.Stop()

	Info("Press Data Knob to exit")
	select {
	case <-done:
	case sig := <-quit:
		mgr.cons.ShouldSpeak(true)
		mgr.cons.Speak(fmt.Sprintf("signal: %s", sig.String()))
		time.Sleep(3*time.Second)
		Err("signal: %v", sig)
	}
	Info("exiting")
}

// xxx measure impact
func raisePriority() error {
	Debug(LOG_MAIN, "chrt")
	pid := os.Getpid()
	return exec.Command("sudo", "chrt", "-a", "-f", "-p", "15", strconv.Itoa(pid)).Run()
}

func assert(b bool, format string, args ...interface{}) {
	if !b {
		panic(fmt.Sprintf("assertion failed: "+format, args...))
	}
}
