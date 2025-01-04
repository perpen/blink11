package main

import (
	"fmt"
	"log/slog"
	"math"
	"time"
)

const autoScalerSlotDuration = 1 * time.Second // reactivity
const autoScalerRange = 1 * time.Hour          // period to consider
// We ignore low/high outliers using factors of the standard deviation
const autoScalerLowFactor = 3
const autoScalerHighFactor = 2

// Metric transformer: for metrics with a negative max, tries to guess
// a plausible max value by looking at recent metric history.
type autoScaler struct {
	histories  map[string]*metricHistory
	slotsCount int
}

type metricHistory struct {
	name     string
	slots    []float64
	counts   []float64
	curSlot  int
	computed bool
	mean, sd float64
}

func newAutoScaler() *autoScaler {
	a := autoScaler{
		histories:  make(map[string]*metricHistory, 0),
		slotsCount: int(autoScalerRange.Milliseconds() / autoScalerSlotDuration.Milliseconds()),
	}
	return &a
}

func (g *autoScaler) adjust(met Metric) Metric {
	if met.max >= 0 {
		return met
	}
	oldMet := met
	ma, found := g.histories[met.name]
	if !found {
		ma = newMetricHistory(met.name, g.slotsCount)
		g.histories[met.name] = ma
	}
	ma.add(met)
	if !ma.computed {
		met.max = met.val
		return met
	}
	low := int(ma.mean - autoScalerLowFactor*ma.sd)
	low = max(low, 0)
	high := int(ma.mean + autoScalerHighFactor*ma.sd)
	met.max = high - low
	met.val -= low
	met.val = max(met.val, 0)
	met.val = min(met.val, met.max)
	if false && met.name == "system.messages" {
		slog.Info(fmt.Sprintf("mean %d, sd %d, low %d, high %d   %d -> %d/%d",
			int(ma.mean), int(ma.sd),
			low, high,
			oldMet.val, met.val, met.max))
	}
	return met
}

func newMetricHistory(name string, slotsCount int) *metricHistory {
	ma := metricHistory{
		name:   name,
		slots:  make([]float64, slotsCount),
		counts: make([]float64, slotsCount),
	}
	go func() {
		ticker := time.NewTicker(autoScalerSlotDuration)
		for {
			<-ticker.C
			ma.shift()
		}
	}()
	return &ma
}

func (h *metricHistory) add(met Metric) {
	if met.val > 0 {
		h.slots[h.curSlot] += float64(met.val)
		h.counts[h.curSlot]++
	}
}

func (h *metricHistory) shift() {
	h.curSlot = (h.curSlot + 1) % len(h.slots)
	h.analyse()
	h.slots[h.curSlot] = 0
	h.counts[h.curSlot] = 0
}

// max = mean+sd for non-0 values
func (h *metricHistory) analyse() {
	sum := .0
	count := .0
	for i := range len(h.slots) {
		slotSum := h.slots[i]
		if slotSum > 0 {
			slotCount := h.counts[i]
			sum += slotSum
			count += slotCount
		}
	}
	mean := sum / count
	sumSqDiff := .0
	for i := range len(h.slots) {
		slotSum := h.slots[i]
		if slotSum == 0 {
			continue
		}
		slotCount := h.counts[i]
		slotMean := slotSum / slotCount
		diff := slotMean - mean
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / float64(len(h.slots))
	sd := math.Sqrt(variance)
	if mean != h.mean {
		// Debug(logging.LOG_FEED, "MetricHistory", "name", ma.name, "mean", ma.mean, "sd", sd)
		h.mean = mean
		h.sd = sd
	}
	h.computed = true
}
