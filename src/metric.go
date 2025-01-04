package main

import (
	"fmt"
	"sync"
)

// The max value may be used or not, depending on the Meter type, but it
// should be >= val. Else the error message will be set before the metric
// is passed to its meter.
// Special case: if max is -1, the max value will be guessed using Automax.
type Metric struct {
	name     string
	val, max int
	err      string
}

func (met Metric) String() string {
	if met.err != "" {
		return fmt.Sprintf("Metric[name:%s error:%s]", met.name, met.err)
	}
	return fmt.Sprintf("Metric[name:%s val:%d max:%d]",
		met.name, met.val, met.max)
}

// If percentage value is less than the gate, set the value to 0
func (m *Metric) Gate(gate float64) {
	valP := float64(m.val) / float64(m.max)
	if valP < gate {
		m.val = 0
	}
}

// The metricsStore is used to restore metrics when switching modes
type metricsStore struct {
	sync.Mutex
	metrics map[string]Metric
}

func newMetricsStore() *metricsStore {
	return &metricsStore{
		metrics: make(map[string]Metric, 0),
	}
}

func (store *metricsStore) retrieve(name string) Metric {
	store.Lock()
	defer store.Unlock()
	return store.metrics[name]
}

func (store *metricsStore) store(met Metric) {
	store.Lock()
	defer store.Unlock()
	store.metrics[met.name] = met
}
