package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type StatReading struct {
	user      int
	nice      int
	system    int
	idle      int
	iowait    int
	irq       int
	softirq   int
	steal     int
	guest     int
	guestNice int
	total     int
}

var prevStatReading StatReading

type Stats struct {
	user, system, iowait, interrupt, nonIdle int
}

// The ints are [0, 100], and 100 represents loadavg / cpus / 2 >= 1
type Loadavg struct {
	one, five, fifteen int
}

func procStatsMsg() string {
	stats := procStats()
	load := procLoadavg()
	msg := fmt.Sprintf("%%h.proc.user %d 100\n"+
		"%%h.proc.system %d 100\n"+
		"%%h.proc.iowait %d 100\n"+
		"%%h.proc.interrupt %d 100\n"+
		"%%h.proc.cpu %d 100\n"+
		"%%h.proc.load1 %d 100\n"+
		"%%h.proc.load5 %d 100\n"+
		"%%h.proc.load15 %d 100",
		stats.user,
		stats.system,
		stats.iowait,
		stats.interrupt,
		stats.nonIdle,
		load.one,
		load.five,
		load.fifteen,
	)
	return msg
}

func procLoadavg() Loadavg {
	buf, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanWords)
	next := func() int {
		scanner.Scan()
		s := scanner.Text()
		f, _ := strconv.ParseFloat(s, 32)
		i := int(100 * float64(f) / float64(numCPU) / 2)
		if i > 100 {
			i = 100
		}
		//fmt.Printf("%s→%f→%d\n", s, f, i)
		return i
	}
	return Loadavg{
		one:     next(),
		five:    next(),
		fifteen: next(),
	}
}

// See proc(5)
func procStats() Stats {
	buf, err := os.ReadFile("/proc/stat")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanWords)
	next := func() int {
		scanner.Scan()
		n, _ := strconv.Atoi(scanner.Text())
		return n
	}
	var stats Stats
	for scanner.Scan() {
		tok := scanner.Text()
		if tok == "cpu" {
			reading := StatReading{
				user:      next(),
				nice:      next(),
				system:    next(),
				idle:      next(),
				iowait:    next(),
				irq:       next(),
				softirq:   next(),
				steal:     next(),
				guest:     next(),
				guestNice: next(),
			}
			reading.total = reading.user + reading.nice + reading.system + reading.idle + reading.iowait + reading.irq + reading.softirq + reading.steal + reading.guest + reading.guestNice
			//fmt.Printf("reading: %v\n", reading)

			total := float64(reading.total-prevStatReading.total) / 100.0
			v := func(n int) int {
				if n < 0 {
					fmt.Printf("negative\n")
					return 0
				}
				return int(float64(n) / total)
			}
			stats = Stats{
				user:      v(reading.user + reading.nice - prevStatReading.user - prevStatReading.nice),
				system:    v(reading.system - prevStatReading.system),
				iowait:    v(reading.iowait - prevStatReading.iowait),
				interrupt: v(reading.irq + reading.softirq - prevStatReading.irq - prevStatReading.softirq),
				nonIdle:   int(100.0 - float64(reading.idle-prevStatReading.idle)/total),
			}
			prevStatReading = reading
			break
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read error:", err)
	}
	return stats
}
