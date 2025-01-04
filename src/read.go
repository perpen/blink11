package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/perpen/blink11/logging"
)

var metricLineRe = regexp.MustCompile(`^metric *([-_.A-Za-z0-9]+) +([0-9]+) +(-?[0-9]+)( +.*)?$`)
var errorLineRe = regexp.MustCompile(`^error *([^ ]+) +(.*)$`)
var controlLineRe = regexp.MustCompile(`^control +(.*)$`)
var soundLineRe = regexp.MustCompile(`^sound +(.*)$`)

// Reads messages and pushes them to the bus.
// Returns a channel that will be used to notify of exit.
// If intercept is not nil, it will be used to transform the parsed
// message, or do something. If it returns nil then the message will
// not be pushed to the channel.
// The name param is used in logging messages.
func readMessages(source string, r io.Reader, bus Unibus,
	intercept func(Message) Message) <-chan bool {

	seen := make(map[string]bool, 0) // to show error effect on disconnection

	type parserFunc func(string) (Message, bool)
	parsers := []parserFunc{
		func(line string) (Message, bool) {
			tokens := metricLineRe.FindStringSubmatch(line)
			if tokens == nil {
				return nil, false
			}
			atoi := func(i int) int {
				n, _ := strconv.Atoi(tokens[i])
				return n
			}
			name := tokens[1]
			seen[name] = true
			met := Metric{
				name: tokens[1],
				val:  atoi(2),
				max:  atoi(3),
			}
			return met, true
		},
		func(line string) (Message, bool) {
			tokens := errorLineRe.FindStringSubmatch(line)
			if tokens == nil {
				return nil, false
			}
			met := Metric{
				name: tokens[1],
				err:  tokens[2],
			}
			return met, true
		},
		func(line string) (Message, bool) {
			tokens := soundLineRe.FindStringSubmatch(line)
			if tokens == nil {
				return nil, false
			}
			sound := strings.TrimSpace(tokens[1])
			snd := Sound{
				name: sound,
			}
			return snd, true
		},
		func(line string) (Message, bool) {
			tokens := controlLineRe.FindStringSubmatch(line)
			if tokens == nil {
				return nil, false
			}
			ctrl := Control{
				msg: tokens[1],
			}
			return ctrl, true
		},
	}

	done := make(chan bool)
	go func() {
		logPrefix := "readMessages"
		Debug(logging.LOG_READER, logPrefix, "source", source)
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if source == "home" {
				Debug(logging.LOG_READER, logPrefix, "line", line)
			}
			var ok bool
			var msg Message
			for _, parser := range parsers {
				msg, ok = parser(line)
				if ok {
					break
				}
			}
			if ok {
				if intercept != nil {
					intercepted := intercept(msg)
					if intercepted != nil {
						bus <- intercepted
					}
				} else {
					bus <- msg
				}
			} else {
				slog.Warn(logPrefix+" invalid message", "line", line)
				continue
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Error(logPrefix+" read", "err", err)
		}

		for metricName := range seen {
			Debug(logging.LOG_READER, logPrefix+" emitting error'", "metric", metricName)
			bus <- Metric{
				name: metricName,
				err:  fmt.Sprintf("EOF from %s", source),
			}
		}
		done <- true
	}()
	return done
}
