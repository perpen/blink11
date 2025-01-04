package main

import (
	"sync"
	"time"
)

const counterMetric = "system.messages"

type messageCounter struct {
	sync.Mutex
	timeSlots []int
	curSlot   int
}

// Pushes periodically a metric indicating the number of messages received
// recently.
func newMessageCounter(bus Unibus) *messageCounter {
	const slots = 3
	const slotDuration = 100 * time.Millisecond

	c := messageCounter{
		timeSlots: make([]int, slots),
		curSlot:   0,
	}

	go func() {
		// Periodically summarise, emit metrics counter metric
		ticker := time.NewTicker(slotDuration)
		for {
			<-ticker.C
			c.Lock()
			sum := 0
			for _, count := range c.timeSlots {
				sum += count
			}
			c.curSlot = (c.curSlot + 1) % slots
			c.timeSlots[c.curSlot] = 0
			c.Unlock()
			bus <- Metric{
				name: counterMetric,
				val:  sum,
				max:  -1,
			}
		}
	}()
	return &c
}

func (c *messageCounter) account() {
	c.Lock()
	defer c.Unlock()
	c.timeSlots[c.curSlot]++
}
