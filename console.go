package main

import (
	"fmt"
	"time"

	"hfdom.org/blink11/v2/audio"
	"hfdom.org/blink11/v2/pidp11"
)

// The Console manages all interactions with the PiDP-11, and audio output.
// LEDs are referenced using type pidp11.LedID, but for switches we use our own
// type `Event`, which provides higher-level events than `pidp11.Event`.
type Console struct {
	pidp                         *pidp11.Pidp
	audio                        audio.Audio
	switches                     map[pidp11.SwitchID]bool
	Events                       chan Event
	knobSoundOk, knobSoundKo     int
	startupSound                 int
	brightnessMin, brightnessMax float64
	brightnessScaling            float64
	percentToHz                  func(v float64) float64
	speak                        bool
}

type Event struct {
	ID  SwitchID
	Val int
}

func (evt Event) String() string {
	return fmt.Sprintf("%s (%d)", switchNames[evt.ID], evt.Val)
}

func NewConsole(cfg Config) *Console {
	pidp := pidp11.NewPidp(cfg.Pidp.Negate_knoba, cfg.Pidp.Negate_knobd, DebugPidp)

	var audioImpl audio.Audio
	var err error
	switch cfg.Audio.Impl {
	case "null":
		audioImpl, err = audio.NewNull()
	case "pipewire":
		audioImpl, err = audio.NewPipewire(
			cfg.Audio.Rate,
			cfg.Audio.Latency_ms,
			cfg.Audio.Dir,
			cfg.Audio.Tmp_dir,
			cfg.Audio.Tts_lang,
			DebugAudio)
	default:
		assert(false, "bad config: invalid audio.impl")
	}
	if err != nil {
		panic(err)
	}

	volume := cfg.Audio.Volume
	assert(0 <= volume && volume <= 1,
		"bad config: audio.volume must be in range [0, 1]: %f", volume)
	audioImpl.SetVolume(cfg.Audio.Volume)
	knobSoundOk, _ := audioImpl.Load(cfg.Audio.Knob_ok)
	knobSoundKo, _ := audioImpl.Load(cfg.Audio.Knob_ko)
	startupSound, _ := audioImpl.Load(cfg.Audio.Startup_sound)

	brightnessMin := cfg.Pidp.Brightness.Min
	assert(brightnessMin >= .05 && brightnessMin <= 1,
		"bad config: pidp.brightness.min must be in range [.05, 1]: %f",
		brightnessMin)
	brightnessMax := cfg.Pidp.Brightness.Max
	assert(brightnessMax >= brightnessMin &&
		brightnessMax <= 1,
		"bad config: pidp.brightness.max must be in range ]pidp.brightness.min, 1]: %f",
		brightnessMax)
	brightnessFactor := cfg.Pidp.Brightness.Initial_scaling
	assert(brightnessFactor >= .1 && brightnessFactor <= 1,
		"bad config: pidp.brightness.max must be in range [.1, 1]: %f",
		brightnessFactor)

	oneHzAt := cfg.Pidp.Frequencies.One_hz_at
	assert(oneHzAt > 0 && oneHzAt < 1, //xxx
		"bad config: pidp.frequencies.one_hz_at must be in range ]0, 1[: %f",
		oneHzAt)
	minHz := cfg.Pidp.Frequencies.Min_hz
	assert(minHz >= 0.5 && minHz <= 2,
		"bad config: pidp.frequencies.min_hz must be in range [.5, 2]: %f",
		minHz)
	maxHz := cfg.Pidp.Frequencies.Max_hz
	assert(maxHz > minHz && maxHz <= 20,
		"bad config: pidp.frequencies.max_hz must be in range ]pidp.frequencies.min_hz, 30]: %f",
		maxHz)

	// Linear scaling of metric value to frequency
	percentToHz := func(pct float64) float64 {
		// we want: a + b*oneHzAt = 1 and a + b = maxHz
		a := (1 - maxHz*oneHzAt) / (1 - oneHzAt)
		b := maxHz - a
		hz := a + b*pct
		if hz < minHz {
			return 0
		}
		return hz
	}

	return &Console{
		pidp:              pidp,
		audio:             audioImpl,
		switches:          make(map[pidp11.SwitchID]bool, 0),
		Events:            make(chan Event, 0),
		knobSoundOk:       knobSoundOk,
		knobSoundKo:       knobSoundKo,
		startupSound:      startupSound,
		brightnessMin:     brightnessMin,
		brightnessMax:     brightnessMax,
		brightnessScaling: brightnessFactor,
		percentToHz:       percentToHz,
		speak:             true,
	}
}

func (cons *Console) Start() {
	cons.audio.Start()
	if cons.startupSound > 0 {
		cons.PlaySound(cons.startupSound)
		time.Sleep(time.Second)
	}
	go cons.eventLoop()
	if err := cons.pidp.Start(); err != nil {
		panic(err)
	}
}

func (cons *Console) Stop() {
	// xxx stop event loop
	cons.pidp.Stop()
	cons.audio.Stop()
}

func (cons *Console) GetBrightnessScaling() float64 {
	return cons.brightnessScaling
}

func (cons *Console) SetBrightnessScaling(val float64) {
	assert(val >= .1 && val <= 1, "invalid brightness factor: %f", val)
	cons.brightnessScaling = val
}

// Loads a sound in memory, returns an ID that can be used to play it.
// Calling multiple times with the same param returns the same ID.
func (cons *Console) LoadSound(name string) (int, error) {
	return cons.audio.Load(name)
}

// Plays sound asynchronously. See cons.NoSound()
func (cons *Console) PlaySound(id int) {
	cons.audio.Play(id)
}

func (cons *Console) KnobSound(good bool) {
	sound := cons.knobSoundOk
	if !good {
		sound = cons.knobSoundKo
	}
	cons.PlaySound(sound)
}

func (cons *Console) Speak(text string) {
	if cons.speak {
		cons.audio.Speak(text)
	}
}

// Dis/allow new sounds from being played
func (cons *Console) ShouldSpeak(b bool) {
	cons.speak = b
}

// Dis/allow new sounds from being played
func (cons *Console) Mute(b bool) {
	cons.audio.Mute(b)
}

func (cons *Console) GetVolume() float64 {
	return cons.audio.GetVolume()
}

func (cons *Console) SetVolume(vol float64) {
	cons.audio.SetVolume(vol)
}

func (cons *Console) LedIDByName(ledName string) (pidp11.LedID, bool) {
	return pidp11.LedIDByName(ledName)
}

func (cons *Console) ClearLeds() {
	cons.pidp.ClearLeds()
}

// Switches a led on (to provided [0, 1] brightness) or off, using given effect.
// The `params` are effect-specific, and must be in range [0, 1].
func (cons *Console) SetLed(id pidp11.LedID, fx pidp11.Effect, bright float64,
	params ...float64) {

	assert(0 <= bright && bright <= 1, "brightness: %f", bright)
	for i, param := range params {
		assert(0 <= param && param <= 1, "param #%d: %f", i, param)
	}
	bright = cons.scaleBrightness(bright)
	cons.pidp.SetLed(id, fx, bright, params...)
}

func (cons *Console) SetLedError(id pidp11.LedID) {
	fx := pidp11.NewErrorEffect()
	cons.pidp.SetLed(id, fx, cons.scaleBrightness(1))
}

// Computes the brightness to be passed to pidp.SetLed(), from the min and max
// levels defined in the config file, and the runtime-adjustable brightness factor.
func (cons *Console) scaleBrightness(bright float64) float64 {
	if bright == 0 {
		return 0
	}
	return cons.brightnessMin + bright*float64(cons.brightnessScaling*(cons.brightnessMax-cons.brightnessMin))
}

func (cons *Console) NewSimpleEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewSimpleEffect(onMs, offMs)
}

func (cons *Console) NewFlashEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewFlashEffect(onMs, offMs, cons.percentToHz)
}

func (cons *Console) NewStrobeEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewStrobeEffect(onMs, offMs, cons.percentToHz)
}

// Processes events of type pidp11.Event, emits events of type Event
func (cons *Console) eventLoop() {
	for evt := range cons.pidp.Events {
		Debug(LOG_EVENT, "Console got: %s %v\n", pidp11.SwitchName(evt.ID), evt.State)
		// From a pidp11.Event we will create a synthetic Event
		synEvt := Event{ID: SS_NIL}

		doKnob := func(knobNum int) {
			switchID := SS_KNOBA
			if knobNum == 1 {
				switchID = SS_KNOBD
			}
			val := 1
			if !evt.State {
				val = -1
			}
			synEvt = Event{
				ID:  switchID,
				Val: val,
			}
		}
		doMomentary := func(id SwitchID) {
			if !evt.State {
				synEvt = Event{ID: id}
			}
		}

		switch evt.ID {
		case pidp11.SW_KNOBA:
			doKnob(0)
		case pidp11.SW_KNOBD:
			doKnob(1)
		case pidp11.SW_KNOBA_PUSH:
			doMomentary(SS_KNOBA_PUSH)
		case pidp11.SW_KNOBD_PUSH:
			doMomentary(SS_KNOBD_PUSH)
		case pidp11.SW_TEST:
			id := SS_TEST_OFF
			if evt.State {
				id = SS_TEST_ON
			}
			synEvt = Event{ID: id}
		case pidp11.SW_LOAD:
			doMomentary(SS_LOAD)
		case pidp11.SW_EXAM:
			doMomentary(SS_EXAM)
		case pidp11.SW_DEP:
			doMomentary(SS_DEP)
		case pidp11.SW_CONT:
			doMomentary(SS_CONT)
		case pidp11.SW_ENABLE:
			id := SS_ENABLE
			if !evt.State {
				id = SS_HALT
			}
			synEvt = Event{ID: id}
		case pidp11.SW_SINST:
			id := SS_S_INST
			if !evt.State {
				id = SS_S_BUS_CYCLE
			}
			synEvt = Event{ID: id}
		case pidp11.SW_START:
			doMomentary(SS_START)
		default:
			cons.switches[evt.ID] = evt.State
		}
		if synEvt.ID != SS_NIL {
			cons.Events <- synEvt
		}
	}
}

// Returns the integer indicated by the register switches.
func (cons *Console) ReadRegSwitches() uint {
	val := uint(0)
	for i := 0; i < 22; i++ {
		if !cons.switches[pidp11.SwitchID(i)] {
			val ^= 1 << i
		}
	}
	Debug(LOG_CONS, "ReadRegSwitches: final %d\n", val)
	return val
}

type SwitchID int

const (
	SS_NIL SwitchID = iota
	SS_KNOBA_PUSH
	SS_KNOBD_PUSH
	SS_TEST_ON
	SS_TEST_OFF
	SS_LOAD
	SS_EXAM
	SS_DEP
	SS_CONT
	SS_ENABLE
	SS_HALT
	SS_S_INST
	SS_S_BUS_CYCLE
	SS_START
	SS_KNOBA
	SS_KNOBD
)

var switchNames = []string{
	"NIL",
	"KNOBA_PUSH",
	"KNOBD_PUSH",
	"TEST_ON",
	"TEST_OFF",
	"LOAD",
	"EXAM",
	"DEP",
	"CONT",
	"ENABLE",
	"HALT",
	"S_INST",
	"S_BUS_CYCLE",
	"START",
	"KNOBA",
	"KNOBD",
	"KNOBS",
}

var KnobAddrLeds = []pidp11.LedID{
	pidp11.LED_CONS_PHY,
	pidp11.LED_KERNEL_D,
	pidp11.LED_SUPER_D,
	pidp11.LED_USER_D,
	pidp11.LED_USER_I,
	pidp11.LED_SUPER_I,
	pidp11.LED_KERNEL_I,
	pidp11.LED_PROG_PHY,
}
var KnobDataLeds = []pidp11.LedID{
	pidp11.LED_BUS_REG,
	pidp11.LED_DATA_PATHS,
	pidp11.LED_μADR_FPP_CPU,
	pidp11.LED_DISPLAY_REGISTER,
}
