package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type DiskStater struct {
	firstTime   bool
	re          *regexp.Regexp
	prevReading DiskReading
}

func (ds *DiskStater) Init() {
	ds.firstTime = true
	ds.re = regexp.MustCompile(`^ *([0-9]+) +([0-9]+) +([a-z0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+)`)
}

type DiskReading struct {
	reads, writes int
}

func (ds *DiskStater) Message() string {
	stats := ds.stats()
	if ds.firstTime {
		ds.firstTime = false
		return ""
	}
	buf := strings.Builder{}
	gen := func(name string, val int) {
		buf.WriteString(fmt.Sprintf("metric %%h.%s %d -1\n", name, val))
	}
	gen("disk.reads", stats.reads)
	gen("disk.writes", stats.writes)
	return buf.String()
}

func (ds *DiskStater) stats() DiskReading {
	// See Documentation/admin-guide/iostats.rst for format
	buf, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanLines)

	reading := DiskReading{}
	for scanner.Scan() {
		line := scanner.Text()
		tokens := ds.re.FindStringSubmatch(line)
		if tokens == nil {
			logger.Error("no match")
		}
		if tokens != nil {
			// # of ops completed
			ri := 4
			rw := 8
			// # of sectors
			// ri := 6
			// rw := 10
			// # of milliseconds spent
			// ri := 11
			// rw := 15
			atoi := func(i int) int {
				n, err := strconv.Atoi(tokens[i])
				if err != nil {
					panic(err)
				}
				return n
			}
			// logger.Info("", "reads", atoi(ri), "writes", atoi(rw))
			reading.reads += atoi(ri)
			reading.writes += atoi(rw)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read error:", err)
	}
	stats := DiskReading{
		reads:  reading.reads - ds.prevReading.reads,
		writes: reading.writes - ds.prevReading.writes,
	}
	// logger.Debug("disk", "reading", reading, "in", string(buf))
	// logger.Debug("disk", "stats", stats)
	ds.prevReading = reading
	return stats
}
