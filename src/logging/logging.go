package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/lib"
)

var debugFlags int
var PidpLogger *slog.Logger
var AudioLogger *slog.Logger

const (
	LOG_MAIN = 1 << iota
	LOG_PIDP
	LOG_LOOP
	LOG_MODE
	LOG_FEED
	LOG_NET
	LOG_HANDLER
	LOG_METER
	LOG_CONSOLE
	LOG_AUDIO
	LOG_EVENT
	LOG_CONTROLLER
	LOG_READER
	LOG_MEMORY
)

var LOG_INDEX_TO_NAME = map[int]string{
	LOG_MAIN:       "main",
	LOG_PIDP:       "pidp",
	LOG_LOOP:       "loop",
	LOG_MODE:       "mode",
	LOG_FEED:       "feed",
	LOG_NET:        "net",
	LOG_HANDLER:    "handler",
	LOG_METER:      "meter",
	LOG_CONSOLE:    "console",
	LOG_AUDIO:      "audio",
	LOG_EVENT:      "event",
	LOG_CONTROLLER: "controller",
	LOG_READER:     "reader",
	LOG_MEMORY:     "memory",
}

func Init(cfg config.Config) {
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: slog.LevelDebug,
		// TimeFormat: time.Kitchen,
		NoColor: true,
	})))

	debugFlags = 0
	for _, name := range cfg.Debug {
		debugFlags |= logIndexForName(strings.ToLower(name))
	}

	pidpLevel := slog.LevelInfo
	if debugFlags&LOG_PIDP != 0 {
		pidpLevel = slog.LevelDebug
	}
	PidpLogger = slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: pidpLevel,
		// TimeFormat: time.Kitchen,
		NoColor: true,
	}))
	audioLevel := slog.LevelInfo
	if debugFlags&LOG_AUDIO != 0 {
		audioLevel = slog.LevelDebug
	}
	AudioLogger = slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level: audioLevel,
		// TimeFormat: time.Kitchen,
		NoColor: true,
	}))
}

func Debug(flags int, format string, args ...any) {
	if flags&debugFlags != 0 {
		format := "[" + logIndexToAName(flags) + "] " + format
		slog.Debug(format, args...)
	}
}

func logIndexForName(s string) int {
	for i, name := range LOG_INDEX_TO_NAME {
		if s == name {
			return i
		}
	}
	lib.Assert(false, "no log for name", "name", s)
	return -1
}

func logIndexToAName(index int) string {
	for i, name := range LOG_INDEX_TO_NAME {
		if i&index > 0 {
			return name
		}
	}
	lib.Assert(false, "no log for index", "index", index)
	return ""
}
