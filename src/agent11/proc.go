package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Stater interface {
	Init()
	Message() string
}

type ProcStater struct {
	cpuCount     int
	prevReadings []ProcReading
	metrics      []ProcMetric
}

func (p *ProcStater) Init() {
	buf, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		panic(err)
	}
	siblingsRe := regexp.MustCompile(`processor\s*:\s*([0-9]+)`)
	allGroups := siblingsRe.FindAllStringSubmatch(string(buf), -1)
	if allGroups == nil {
		logger.Error("cannot find processor lines in cpuinfo",
			"re", siblingsRe, "buf", string(buf))
		os.Exit(1)
	}
	groups := allGroups[len(allGroups)-1]
	lastCpu, err := strconv.Atoi(groups[1])
	if err != nil {
		panic(err)
	}
	logger.Info("ProcStater", "cpuCount", lastCpu)
	p.cpuCount = lastCpu+1
	p.prevReadings = make([]ProcReading, p.cpuCount+1)
	p.metrics = make([]ProcMetric, p.cpuCount+1)
}

type ProcReading struct {
	cpuNum    int
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

type ProcMetric struct {
	user, system, idle, iowait, interrupt, steal, nonIdle int
}

// We may have loadavg greater than 1, give some headroom
const loadAvgFactor = .5

// The ints are [0, 100], and 100 represents loadAvgFactor * loadavg / cpus >= 1
type Loadavg struct {
	one, five, fifteen int
}

func (p *ProcStater) Message() string {
	p.compute()
	load := procLoadavg()
	buf := strings.Builder{}
	gen := func(name string, val int) {
		buf.WriteString(fmt.Sprintf("metric %%h.%s %d 100\n", name, val))
	}
	for cpu, metric := range p.metrics {
		var prefix string
		if cpu == p.cpuCount {
			prefix = "cpu."
		} else {
			prefix = fmt.Sprintf("cpu%d.", cpu)
		}
		gen(prefix+"user", metric.user)
		gen(prefix+"system", metric.system)
		gen(prefix+"idle", metric.idle)
		gen(prefix+"iowait", metric.iowait)
		gen(prefix+"interrupt", metric.interrupt)
		gen(prefix+"steal", metric.steal)
		gen(prefix+"cpu", metric.nonIdle)
	}
	gen("load.one", load.one)
	gen("load.five", load.five)
	gen("load.fifteen", load.fifteen)
	return buf.String()
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
		i := int(100 * loadAvgFactor * float64(f) / float64(numCPU))
		i = min(i, 100)
		return i
	}
	return Loadavg{
		one:     next(),
		five:    next(),
		fifteen: next(),
	}
}

// See proc(5)
func (p ProcStater) compute() {
	logger.Debug("compute")
	buf, err := os.ReadFile("/proc/stat")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanWords)
out:
	for {
		scanner.Scan()
		tok := scanner.Text()
		logger.Debug("scanner", "tok", tok)

		var index int
		switch {
		case tok == "cpu":
			index = p.cpuCount
		case strings.HasPrefix(tok, "cpu"):
			cpuNumString := strings.TrimPrefix(tok, "cpu")
			cpuNum, err := strconv.Atoi(cpuNumString)
			if err != nil {
				panic(err)
			}
			index = cpuNum
		default:
			// We don't care about the rest
			break out
		}

		nextCount := 1
		next := func() int {
			nextCount++
			if !scanner.Scan() {
				logger.Error("next() ran out of tokens, bug")
				os.Exit(1)
			}
			txt := scanner.Text()
			n, err := strconv.Atoi(txt)
			if err != nil {
				logger.Error("next: bad int", "txt", txt)
				os.Exit(1)
			}
			return n
		}
		reading := ProcReading{
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
		if nextCount != 11 {
			logger.Error("nextCount != 11", "nextCount", nextCount)
			os.Exit(1)
		}
		reading.total = reading.user + reading.nice + reading.system +
			reading.idle + reading.iowait + reading.irq + reading.softirq +
			reading.steal + reading.guest + reading.guestNice
		logger.Debug("", "reading", reading)

		prevReading := p.prevReadings[index]
		total := float64(reading.total-prevReading.total) / 100.0
		v := func(name string, old, new int) int {
			delta := new - old
			if delta < 0 {
				logger.Error("negative number, bug", "name", name,
					"old", old, "new", new, "delta", delta)
				logger.Error("", "old", prevReading, "new", reading)
				return 0
			}
			return int(float64(delta) / total)
		}
		metric := ProcMetric{
			user:      v("user", prevReading.user+prevReading.nice, reading.user+reading.nice),
			system:    v("system", prevReading.system, reading.system),
			idle:      v("idle", prevReading.idle, reading.idle),
			iowait:    v("iowait", prevReading.iowait, reading.iowait),
			interrupt: v("interrupt", prevReading.irq+prevReading.softirq, reading.irq+reading.softirq),
			steal:     v("steal", prevReading.steal, reading.steal),
			// nonIdle:   v("nonIdle", 100-idle, 0),
		}
		metric.nonIdle = metric.user + metric.system
		p.metrics[index] = metric
		p.prevReadings[index] = reading
	}
	if err := scanner.Err(); err != nil {
		logger.Error("scanner", "err", err)
	}
}
