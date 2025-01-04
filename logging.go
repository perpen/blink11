package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

var debugFlags int
var debugLogger *log.Logger
var warnLogger *log.Logger
var infoLogger *log.Logger

const (
	LOG_MAIN = 1 << iota
	LOG_PIDP
	LOG_MANAGER
	LOG_MODE
	LOG_FEED
	LOG_NET
	LOG_HANDLER
	LOG_METER
	LOG_CONS
	LOG_AUDIO
	LOG_EVENT
	LOG_COMMAND
	LOG_MEMORY
)

func InitLogging(cfg Config) {
	debugFlags = 0
	for _, name := range cfg.Logging {
		debugFlags |= logByName(strings.ToUpper(name))
	}
	debugLogger = log.New(os.Stdout, "", 0)
	warnLogger = debugLogger
	infoLogger = debugLogger
}

func logByName(name string) int {
	log, ok := map[string]int{
		"MAIN":    LOG_MAIN,
		"PIDP":    LOG_PIDP,
		"MANAGER": LOG_MANAGER,
		"MODE":    LOG_MODE,
		"FEED":    LOG_FEED,
		"NET":     LOG_NET,
		"HANDLER": LOG_HANDLER,
		"METER":   LOG_METER,
		"CONS":    LOG_CONS,
		"AUDIO":   LOG_AUDIO,
		"EVENT":   LOG_EVENT,
		"COMMAND": LOG_COMMAND,
		"MEMORY":  LOG_MEMORY,
	}[name]
	assert(ok, "unknown log category: %s", name)
	return log
}

func Debug(flags int, format string, args ...interface{}) {
	if flags&debugFlags != 0 {
		_, filepath, line, _ := runtime.Caller(1)
		file := path.Base(filepath)
		prefix := fmt.Sprintf("DEB %s:%d ", file, line)
		debugLogger.Printf(prefix+format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	_, filepath, line, _ := runtime.Caller(1)
	file := path.Base(filepath)
	prefix := fmt.Sprintf("WAR %s:%d ", file, line)
	warnLogger.Printf(prefix+format, args...)
}

func Err(format string, args ...interface{}) {
	_, filepath, line, _ := runtime.Caller(1)
	file := path.Base(filepath)
	prefix := fmt.Sprintf("ERR %s:%d ", file, line)
	warnLogger.Printf(prefix+format, args...)
}

func Info(format string, args ...interface{}) {
	_, filepath, line, _ := runtime.Caller(1)
	file := path.Base(filepath)
	prefix := fmt.Sprintf("INF %s:%d ", file, line)
	infoLogger.Printf(prefix+format, args...)
}

func DebugAudio(format string, args ...interface{}) {
	Debug(LOG_AUDIO, format, args...)
}

func DebugPidp(format string, args ...interface{}) {
	Debug(LOG_PIDP, format, args...)
}
