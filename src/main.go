package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/console"
	"github.com/perpen/blink11/lib"
	"github.com/perpen/blink11/logging"
	"github.com/perpen/blink11/memory"
)

var exitChan chan error

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: blink11 CONFIG\n")
		os.Exit(2)
	}

	// Lock file
	lockPath := "/tmp/blink11.lock"
	_, err := os.Stat(lockPath)
	lib.Assert(err == nil || os.IsNotExist(err), "stat", "err", err, "lock", lockPath)
	if err == nil {
		slog.Error("already running", "lock", lockPath)
		os.Exit(1)
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE, 0)
	lib.Assert(err == nil, "cannot create lock", "err", err)
	defer os.Remove(lockPath)
	defer lockFile.Close()

	err = raisePriority()
	lib.Assert(err == nil, "cannot raise process priority", "err", err)

	cfg := config.ParseConfig(os.Args[1])

	logging.Init(cfg)
	audioLogger := logging.AudioLogger
	pidpLogger := logging.PidpLogger

	signalChan := make(chan os.Signal, 5)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signalChan)

	memory.Init(cfg.Memory_path)
	err = console.Start(cfg.Audio, cfg.Pidp, pidpLogger, audioLogger)
	lib.Check(err == nil, "console did not start", "err", err)
	defer console.Stop()
	go eventLoop(cfg)

	exitChan = make(chan error)
	exitHook := func(s string) {
		go func() {
			exitChan <- fmt.Errorf("%s", s)
		}()
	}
	lib.SetExitHook(exitHook)

	select {
	case err = <-exitChan:
		if err == nil {
			slog.Info("exiting")
		} else {
			slog.Error("exiting", "err", err)
		}
	case s := <-signalChan:
		slog.Error("signalled", "signal", s)
	}
}

func Debug(flags int, format string, args ...any) {
	logging.Debug(flags, format, args...)
}

// xxx measure impact
func raisePriority() error {
	Debug(logging.LOG_MAIN, "chrt")
	pid := os.Getpid()
	return exec.Command("sudo", "chrt", "-a", "-f", "-p", "15", strconv.Itoa(pid)).Run()
}
