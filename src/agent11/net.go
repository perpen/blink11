package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type NetStater struct {
	prevReading NetReading
	firstTime   bool
	re          *regexp.Regexp
}

func (ns *NetStater) Init() {
	ns.firstTime = true
	ns.re = regexp.MustCompile(`^ *([a-z0-9]+): +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+)`)
}

type NetReading struct {
	packetsRx, packetsTx int
}

func (ns *NetStater) Message() string {
	stats := ns.stats()
	if ns.firstTime {
		ns.firstTime = false
		return ""
	}
	buf := strings.Builder{}
	gen := func(name string, val int) {
		buf.WriteString(fmt.Sprintf("metric %%h.%s %d -1\n", name, val))
	}
	gen("rx_packets", stats.packetsRx)
	gen("tx_packets", stats.packetsTx)
	gen("packets", stats.packetsRx+stats.packetsTx)
	return buf.String()
}

func (ns *NetStater) stats() NetReading {
	buf, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanLines)

	reading := NetReading{}
	for scanner.Scan() {
		line := scanner.Text()
		tokens := ns.re.FindStringSubmatch(line)
		if tokens != nil {
			atoi := func(i int) int {
				n, _ := strconv.Atoi(tokens[i])
				return n
			}
			reading.packetsRx += atoi(3)
			reading.packetsTx += atoi(11)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read error:", err)
	}

	stats := NetReading{
		packetsRx: reading.packetsRx - ns.prevReading.packetsRx,
		packetsTx: reading.packetsTx - ns.prevReading.packetsTx,
	}
	logger.Debug("net", "reading", reading)
	ns.prevReading = reading
	return stats
}
