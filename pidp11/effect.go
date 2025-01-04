package pidp11

import (
	"math"
)

type Effect interface {
	makeEnvelope(spec *ledSpec, bright int, params ...float64)
}

// One-shot attack or release, onMs used when switching on, offMs when off
type SimpleEffect struct {
	onMs, offMs int
}

func NewSimpleEffect(onMs, offMs int) SimpleEffect {
	return SimpleEffect{
		onMs:  onMs,
		offMs: offMs,
	}
}

func (fx SimpleEffect) makeEnvelope(spec *ledSpec, bright int, params ...float64) {
	assertParams(0, params)
	env := &spec.env
	delta := abs(bright - spec.bright)
	var fxMs int
	if bright == 0 {
		fxMs = fx.offMs
	} else {
		fxMs = fx.onMs
	}
	ms := scale(delta, 31, 0, fxMs)
	env.addStage(spec.bright, bright, ms, true)
}

type StrobeEffect struct {
	strobeOnLoops, onMs, offMs int
	// xxx expose param, or remove:
	lowDivider int // used to compute the low value from the requested brightness
	scaler     func(float64) float64
}

func NewStrobeEffect(onMs, offMs int, scaler func(float64) float64) StrobeEffect {
	return StrobeEffect{
		strobeOnLoops: 60, // how long the led should stay on
		onMs:          onMs,
		offMs:         offMs,
		lowDivider:    999,
		scaler:        scaler,
	}
}

func (fx StrobeEffect) makeEnvelope(spec *ledSpec, bright int, params ...float64) {
	assertParams(1, params)
	hz := fx.scaler(params[0])
	onMs := fx.onMs
	offMs := fx.offMs
	if hz == 0 {
		spec.env.addStage(spec.bright, 0, offMs, true)
		return
	}
	periodMs := int(math.Round(1e3 / hz))
	strobeOnMs := loopμs * fx.strobeOnLoops / 1e3
	restMs := periodMs - strobeOnMs
	assert(restMs >= 0, "restMs=%d", restMs)
	if onMs+offMs > restMs {
		shrinkage := float64(restMs) / float64(onMs+offMs)
		onMs = int(math.Floor(float64(onMs) * shrinkage))
		offMs = int(math.Floor(float64(offMs) * shrinkage))
		assert(onMs+offMs <= restMs, "onMs=%d, offMs=%d, restMs=%d",
			onMs, offMs, restMs)
	}
	upMs := onMs + strobeOnMs
	downMs := periodMs - upMs
	spec.setupASRS(bright, bright/fx.lowDivider, onMs, offMs, upMs, downMs)
}

type FlashEffect struct {
	onMs, offMs int
	lowDivider  int // used to compute the low value from the requested brightness
	scaler      func(float64) float64
}

func NewFlashEffect(onMs, offMs int, scaler func(float64) float64) FlashEffect {
	return FlashEffect{
		onMs:       onMs,
		offMs:      offMs,
		lowDivider: 999,
		scaler:     scaler,
	}
}

func (fx FlashEffect) makeEnvelope(spec *ledSpec, bright int, params ...float64) {
	assertParams(1, params)
	hz := fx.scaler(params[0])
	onMs := fx.onMs
	offMs := fx.offMs
	if hz == 0 {
		spec.env.addStage(spec.bright, 0, offMs, true)
		return
	}
	periodMs := int(math.Round(1e3 / hz))
	assert(periodMs >= 2, "periodMs=%d", periodMs)
	downMs := periodMs / 2
	upMs := downMs
	if offMs > downMs {
		offMs = downMs
	}
	if onMs > upMs {
		onMs = upMs
	}
	spec.setupASRS(bright, bright/fx.lowDivider, onMs, offMs, upMs, downMs)
}

// Create an attack-sustain-release-sustain envelope
func (spec *ledSpec) setupASRS(hi, lo, onMs, offMs, upMs, downMs int) {
	debug("setupASRS: led=%s, onMs=%d, offMs=%d, upMs=%d, downMs=%d", spec.name, onMs, offMs, upMs, downMs)
	env := &spec.env
	if onMs > 0 {
		env.addStage(lo, hi, onMs, false) // attack
	}
	env.addStage(hi, hi, upMs-onMs, false) // sustain high
	if offMs > 0 {
		env.addStage(hi, lo, offMs, false) // release
	}
	env.addStage(lo, lo, downMs-offMs, false) //sustain low
}

// Blinks the leds using a hopefully recognisable envelope
type ErrorEffect struct{}

func NewErrorEffect() ErrorEffect {
	return ErrorEffect{}
}

func (fx ErrorEffect) makeEnvelope(spec *ledSpec, bright int, params ...float64) {
	assertParams(0, params)
	hi := bright
	ms := 200
	lo := hi / 4
	spec.env.addStage(hi, 0, ms, false)
	spec.env.addStage(0, lo, ms, false)
	spec.env.addStage(lo, lo, ms, false)
}

func assertParams(count int, params []float64) {
	assert(len(params) == count,
		"expected %d params, got %d", count, len(params))
}

func scale(inVal, inMax, outMin, outMax int) int {
	if inVal == 0 && inMax == 0 || outMin == outMax {
		return outMin
	}
	inFrac := float64(inVal) / float64(inMax)
	return int(math.Round(float64(outMin) + inFrac*float64(outMax-outMin)))
}
