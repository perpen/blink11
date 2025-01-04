package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type NetReading struct {
	packetsRx, packetsTx int
}

var prevNetReading NetReading
var netStarted = false
var maxRx = 1000
var maxTx = 1000

func netStatsMsg() string {
	if !netStarted {
		netStats() // to init prevNetReading
		netStarted = true
	}
	stats := netStats()
	//fmt.Printf("stats=%v\n", stats)
	if stats.packetsRx > maxRx {
		maxRx = stats.packetsRx
	}
	if stats.packetsTx > maxTx {
		maxTx = stats.packetsTx
	}
	maxTRx := maxRx + maxTx
	msg := fmt.Sprintf(
		"%%h.rx_packets %d %d\n"+
			"%%h.tx_packets %d %d\n"+
			"%%h.packets %d %d",
		stats.packetsRx, maxRx,
		stats.packetsTx, maxTx,
		stats.packetsRx+stats.packetsTx, maxTRx,
	)
	return msg
}

func netStats() NetReading {
	netStatsRe := regexp.MustCompile(`^ *([a-z0-9]+): +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+) +([0-9]+)`)

	buf, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(buf)))
	scanner.Split(bufio.ScanLines)

	reading := NetReading{}
	for scanner.Scan() {
		line := scanner.Text()
		tokens := netStatsRe.FindStringSubmatch(line)
		if tokens != nil {
			//fmt.Printf("tokens: %v\n", tokens)
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
		packetsRx: reading.packetsRx - prevNetReading.packetsRx,
		packetsTx: reading.packetsTx - prevNetReading.packetsTx,
	}
	prevNetReading = reading
	return stats
}
