package main

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/perpen/blink11/audio"
	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/console"
	"github.com/perpen/blink11/lib"
	"github.com/perpen/blink11/logging"
	"github.com/perpen/pidp11"
)

// A Meter defines the visual representation of a metric
type Meter struct {
	updateFn func(*Meter, Metric)
	gate     float64
	ledIDs   []pidp11.LedID
	fx       pidp11.Effect
	sound    *audio.Sound
}

func (m Meter) String() string {
	snd := ""
	if m.sound != nil {
		snd = fmt.Sprintf(" sound:%v", m.sound)
	}
	gate := ""
	if m.gate != 0 {
		gate = fmt.Sprintf(" gate:%s", strconv.FormatFloat(m.gate, 'f', -1, 64))
	}
	ledNames := strings.Join(pidp11.LedIDsToNames(m.ledIDs), " ")
	return fmt.Sprintf("Meter[leds:[%s]%s%s]", ledNames, snd, gate)
}

func newMeterFromConfig(cfg config.ConfigMeter) *Meter {
	ledIDs := pidp11.LedNamesToIDs(cfg.Leds)
	onMs := cfg.On_ms
	offMs := cfg.Off_ms
	gate := 0.0
	if cfg.Gate != 0 {
		gate = cfg.Gate
	}
	var meter *Meter
	switch cfg.Type {
	case "lumen":
		meter = newLumenMeter(gate, ledIDs, cfg.Sound, cfg.On_ms, cfg.Off_ms)
	case "bar":
		meter = newBarMeter(gate, ledIDs, cfg.Sound, cfg.On_ms, cfg.Off_ms, cfg.Floor)
	case "dot":
		meter = newDotMeter(gate, ledIDs, cfg.Sound, cfg.Floor, cfg.On_ms, cfg.Off_ms)
	case "binary":
		meter = newBinaryMeter(gate, ledIDs, cfg.Sound, onMs, offMs)
	case "flash":
		meter = newFlashMeter(gate, ledIDs, cfg.Sound, onMs, offMs)
	case "strobe":
		meter = newStrobeMeter(gate, ledIDs, cfg.Sound, onMs, offMs)
	default:
		lib.Assert(false, "unknown meter type", "type", cfg.Type)
	}
	return meter
}

func newLumenMeter(gate float64, ledIDs []pidp11.LedID,
	soundName string, onMs, offMs int) *Meter {

	fx := console.NewSimpleEffect(onMs, offMs)
	update := func(m *Meter, met Metric) {
		val := met.val
		max := met.max
		bright := scaleFloat(val, max, 0, 1)
		for _, ledID := range m.ledIDs {
			console.Led(ledID, m.fx, bright)
		}
	}
	return newMeter(update, gate, ledIDs, fx, soundName)
}

func newBarMeter(gate float64, ledIDs []pidp11.LedID,
	soundName string, onMs, offMs int, floor bool) *Meter {

	fx := console.NewSimpleEffect(onMs, offMs)
	scaler := scaleRound
	if floor {
		scaler = scaleFloor
	}
	update := func(m *Meter, met Metric) {
		val := met.val
		max := met.max
		ledsCount := len(m.ledIDs)
		highest := scaler(val, max, 0, float64(ledsCount))
		for i, ledID := range m.ledIDs {
			bright := .0
			if i < highest {
				bright = 1
			}
			console.Led(ledID, m.fx, bright)
		}
	}
	return newMeter(update, gate, ledIDs, fx, soundName)
}

func newBinaryMeter(gate float64, ledIDs []pidp11.LedID,
	soundName string, onMs, offMs int) *Meter {

	fx := console.NewSimpleEffect(onMs, offMs)
	update := func(m *Meter, met Metric) {
		for bit := range len(m.ledIDs) {
			ledID := m.ledIDs[bit]
			bright := .0
			if met.val&(1<<bit) != 0 {
				bright = 1
			}
			console.Led(ledID, m.fx, bright)
		}
	}
	return newMeter(update, gate, ledIDs, fx, soundName)
}

// Flash or strobe depending on the effect param
func newFlashOrStrobeMeter(gate float64, ledIDs []pidp11.LedID,
	fx pidp11.Effect, soundName string) *Meter {

	update := func(m *Meter, met Metric) {
		val := met.val
		max := met.max
		pct := .0
		if val != 0 {
			pct = scaleFloat(val, max, 0, 1)
		}
		for _, ledID := range m.ledIDs {
			console.Led(ledID, m.fx, 1, pct)
		}
	}

	return newMeter(update, gate, ledIDs, fx, soundName)
}

func newStrobeMeter(gate float64, ledIDs []pidp11.LedID, soundName string,
	onMs, offMs int) *Meter {

	fx := console.NewStrobeEffect(onMs, offMs)
	return newFlashOrStrobeMeter(gate, ledIDs, fx, soundName)
}

func newFlashMeter(gate float64, ledIDs []pidp11.LedID, soundName string,
	onMs, offMs int) *Meter {

	fx := console.NewFlashEffect(onMs, offMs)
	return newFlashOrStrobeMeter(gate, ledIDs, fx, soundName)
}

func newDotMeter(gate float64, ledIDs []pidp11.LedID, soundName string,
	doFloor bool, onMs, offMs int) *Meter {

	fx := console.NewSimpleEffect(onMs, offMs)
	scaler := scaleRound
	if doFloor {
		scaler = scaleFloor
	}
	update := func(m *Meter, met Metric) {
		val := met.val
		max := met.max
		ledsCount := len(m.ledIDs)
		highest := scaler(val, max, 0, float64(ledsCount))
		for i, ledID := range m.ledIDs {
			bright := .0
			if met.val >= 0 && i == highest-1 {
				bright = 1
			}
			console.Led(ledID, m.fx, bright)
		}
	}
	meter := newMeter(update, gate, ledIDs, fx, soundName)
	return meter
}

func newMeter(update func(*Meter, Metric), gate float64,
	ledIDs []pidp11.LedID, fx pidp11.Effect, soundName string) *Meter {

	lib.Check(0 <= gate && gate <= 1.0, "meter gate must be in [0, 1]",
		"gate", gate)
	m := Meter{
		updateFn: update,
		gate:     gate,
		ledIDs:   ledIDs,
		fx:       fx,
	}
	if soundName != "" {
		soundID, err := console.LoadSound(soundName)
		lib.Assert(err == nil, "cannot load sound", "name", soundName)
		m.sound = soundID
	}
	return &m
}

func (m *Meter) visualise(met Metric) {
	if met.name != counterMetric {
		Debug(logging.LOG_METER, "Meter.Visualise", "metric", met.name, "val", met.val, "max", met.max)
	}
	if met.val < 0 || met.val > met.max {
		met.err = fmt.Sprintf("updateDot: %s val=%d/%d", met.name, met.val, met.max)
	}
	if met.err != "" {
		slog.Error("Meter.Visualise", "metric", met.name, "err", met.err)
		for _, ledID := range m.ledIDs {
			console.LedError(ledID)
		}
		return
	}
	met.Gate(m.gate)
	if met.val > 0 {
		console.PlaySound(m.sound)
	}
	m.updateFn(m, met)
}

func scaleFloat(inVal, inMax int, outMin, outMax float64) float64 {
	lib.Assert((inVal == 0 || inMax != 0) && outMax != 0,
		"scaleFloat given invalid args",
		"inVal", inVal, "inMax", inMax, "outMin", outMin, "outMax", outMax)
	if inVal == 0 {
		return outMin
	}
	inFrac := float64(inVal) / float64(inMax)
	return outMin + inFrac*(outMax-outMin)
}

func scaleRound(inVal, inMax int, outMin, outMax float64) int {
	return int(math.Round(scaleFloat(inVal, inMax, outMin, outMax)))
}

func scaleFloor(inVal, inMax int, outMin, outMax float64) int {
	return int(math.Floor(scaleFloat(inVal, inMax, outMin, outMax)))
}
