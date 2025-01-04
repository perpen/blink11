package main

import (
	"math"

	"hfdom.org/blink11/v2/pidp11"
)

// A Meter specifies the visual representation of a metric
type Meter struct {
	updateFn func(*Meter, Metric)
	cons     *Console
	lightIDs []pidp11.LedID
	fx       pidp11.Effect
	sound    string
	soundID  int
}

func NewMeter(update func(*Meter, Metric), cons *Console,
	lightIDs []pidp11.LedID, fx pidp11.Effect, soundName string) *Meter {

	m := Meter{
		updateFn: update,
		cons:     cons,
		lightIDs: lightIDs,
		fx:       fx,
	}
	if soundName != "" {
		soundID, err := cons.LoadSound(soundName)
		assert(err == nil, "cannot load %s", soundName)
		m.soundID = soundID
	}
	return &m
}

func (m *Meter) Update(met Metric) {
	if met.name != "metrics_rx" {
		Debug(LOG_METER, "Update: %s val/max=%d/%d", met.name, met.val, met.max)
	}
	if met.val < 0 || met.val > met.max {
		Warn("updateDot: %s val=%d/%d\n", met.name, met.val, met.max)
		met.bad = true
	}
	if met.bad {
		for _, lightID := range m.lightIDs {
			m.cons.SetLedError(lightID)
		}
		return
	}
	if m.soundID != 0 {
		m.cons.PlaySound(m.soundID)
	}
	m.updateFn(m, met)
}

func updateLumen(m *Meter, met Metric) {
	val := met.val
	max := met.max
	bright := scaleFloat(val, max, 0, 1)
	for _, lightID := range m.lightIDs {
		m.cons.SetLed(lightID, m.fx, bright)
	}
}

func updateBar(m *Meter, met Metric) {
	_updateBar(m, met, scaleRound)
}

// Useful eg to represent the hours for the current time
func updateBarFloor(m *Meter, met Metric) {
	_updateBar(m, met, scaleFloor)
}

func _updateBar(m *Meter, met Metric,
	scaler func(inVal, inMax int, outMin, outMax float64) int) {

	val := met.val
	max := met.max
	lightsCount := len(m.lightIDs)
	highest := scaler(val, max, 0, float64(lightsCount))
	for i, lightID := range m.lightIDs {
		bright := .0
		if i < highest {
			bright = 1
		}
		m.cons.SetLed(lightID, m.fx, bright)
	}
}

func updateDot(m *Meter, met Metric) {
	_updateDot(m, met, scaleRound)
}

// Useful eg to represent the hours for the current time
func updateDotFloor(m *Meter, met Metric) {
	_updateDot(m, met, scaleFloor)
}

func _updateDot(m *Meter, met Metric,
	scaler func(inVal, inMax int, outMin, outMax float64) int) {

	val := met.val
	max := met.max
	lightsCount := len(m.lightIDs)
	highest := scaler(val, max, 0, float64(lightsCount))
	for i, lightID := range m.lightIDs {
		bright := .0
		if met.val >= 0 && i == highest-1 {
			bright = 1
		}
		m.cons.SetLed(lightID, m.fx, bright)
	}
}

func updateFlashOrStrobe(m *Meter, met Metric) {
	val := met.val
	max := met.max
	pct := .0
	if val != 0 {
		pct = scaleFloat(val, max, 0, 1)
	}
	for _, lightID := range m.lightIDs {
		m.cons.SetLed(lightID, m.fx, 1, pct)
	}
}

func updateBinary(m *Meter, met Metric) {
	for bit := 0; bit < len(m.lightIDs); bit++ {
		ledID := m.lightIDs[bit]
		bright := .0
		if met.val&(1<<bit) != 0 {
			bright = 1
		}
		m.cons.SetLed(ledID, m.fx, bright)
	}
}

func scaleFloat(inVal, inMax int, outMin, outMax float64) float64 {
	assert((inVal == 0 || inMax != 0) && outMax != 0,
		"invalid args: inVal/inMax=%d/%d, outMin/outMax=%f/%f",
		inVal, inMax, outMin, outMax)
	inFrac := float64(inVal) / float64(inMax)
	return outMin + inFrac*(outMax-outMin)
}

func scaleRound(inVal, inMax int, outMin, outMax float64) int {
	return int(math.Round(scaleFloat(inVal, inMax, outMin, outMax)))
}

func scaleFloor(inVal, inMax int, outMin, outMax float64) int {
	return int(math.Floor(scaleFloat(inVal, inMax, outMin, outMax)))
}
