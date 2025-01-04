package console

import (
	"log/slog"
	"sync"

	"github.com/perpen/blink11/audio"
	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/lib"
	"github.com/perpen/blink11/memory"
	"github.com/perpen/pidp11"
)

// The Console interacts with the PiDP-11 and audio.
var Events <-chan pidp11.Event
var mixer *audio.Mixer
var knobSoundOk, knobSoundKo, volumeSound *audio.Sound
var brightnessMin, brightnessMax float64
var brightnessAdjust float64
var modeSpeechEnabled bool
var lock sync.Mutex

func Start(cfgAudio config.ConfigAudio, cfgPidp config.ConfigPidp,
	pidpLogger, audioLogger *slog.Logger) error {

	var err error
	mixer, err = audio.NewMixer(cfgAudio, audioLogger)
	lib.Check(err == nil, "audio init failed", "err", err)
	mixer.Init()
	knobSoundOk, _ = mixer.Load(cfgAudio.Knob_ok_sound)
	knobSoundKo, _ = mixer.Load(cfgAudio.Knob_ko_sound)
	volumeSound, _ = mixer.Load(cfgAudio.Volume_sound)
	volume, found := memory.Volume()
	if !found {
		volume = .5
	}
	mixer.SetVolume(volume)
	speechEnabled, found := memory.SpeechEnabled()
	if !found {
		speechEnabled = true
	}
	EnableModeSpeech(speechEnabled)
	LoadAndPlaySound(cfgAudio.Startup_sound)

	brightnessMin = cfgPidp.Brightness.Min
	lib.Check(brightnessMin >= .03 && brightnessMin <= 1,
		"bad config: pidp.brightness.min must be in range [.03, 1]",
		"pidp.brightness.min", brightnessMin)
	brightnessMax = cfgPidp.Brightness.Max
	lib.Check(brightnessMax >= brightnessMin &&
		brightnessMax <= 1,
		"bad config: pidp.brightness.max must be in range ]pidp.brightness.min, 1]",
		"pidp.brightness.max", brightnessMax)

	oneHzAt := cfgPidp.Frequencies.One_hz_at
	lib.Check(oneHzAt > 0 && oneHzAt < 1, //xxx
		"bad config: pidp.frequencies.one_hz_at must be in range ]0, 1[",
		"pidp.frequences.one_hz_at", oneHzAt)
	minHz := cfgPidp.Frequencies.Min_hz
	lib.Check(minHz >= 0.5 && minHz <= 2,
		"bad config: pidp.frequencies.min_hz must be in range [.5, 2]",
		"pid.frequencies.min_hz", minHz)
	maxHz := cfgPidp.Frequencies.Max_hz
	lib.Check(maxHz > minHz && maxHz <= 20,
		"bad config: pidp.frequencies.max_hz must be in range ]pidp.frequencies.min_hz, 30]",
		"pid.frequencies.max_hz", maxHz)

	brightnessAdjust, found = memory.Brightness()
	if !found {
		brightnessAdjust = .5
	}
	pidp11.SetBrightnessAdjust(brightnessAdjust)
	pidp11.SetBrightnessScaler(pidp11.NewLinearBrightnessScaler(brightnessMin, brightnessMax))
	pidp11.SetFrequencyScaler(pidp11.NewLinearFrequencyScaler(minHz, maxHz, oneHzAt))

	err = pidp11.Start(pidpLogger)
	if err != nil {
		return err
	}
	Events = pidp11.Events()
	return nil
}

func Stop() {
	err := pidp11.Stop()
	if err != nil {
		slog.Error("cannot close rpio", "err", err)
	}
}

// Returns the integer indicated by the register switches.
func ReadRegSwitches() uint {
	return pidp11.ReadRegSwitches()
}

func GetBrightnessScaling() float64 {
	return pidp11.GetBrightnessAdjust()
}

func SetBrightnessScaling(val float64) {
	lib.Assert(val >= .1 && val <= 1, "invalid brightness factor", "factor", val)
	pidp11.SetBrightnessAdjust(val)
}

// Loads a sound in memory, returns a reference that can be used to play it.
// Calling multiple times with the same param returns the same ID.
// Can be slow, eg if the sound is tts.
func LoadSound(name string) (*audio.Sound, error) {
	return mixer.Load(name)
}

// Plays sound asynchronously
func PlaySound(sound *audio.Sound) {
	mixer.Play(sound)
}

func LoadAndPlaySound(name string) error {
	id, err := LoadSound(name)
	if err != nil {
		slog.Error("cannot load sound", "sound", name, "err", err)
	} else {
		PlaySound(id)
	}
	return err
}

func KnobSound(good bool) {
	sound := knobSoundOk
	if !good {
		sound = knobSoundKo
	}
	PlaySound(sound)
}

func SpeakMode(text string) {
	if ModeSpeechEnabled() {
		Speak(text)
	}
}

func Speak(text string) {
	go mixer.Speak(text)
}

func SpeakSync(text string) {
	if ModeSpeechEnabled() {
		mixer.SpeakSync(text)
	}
}

func EnableModeSpeech(b bool) {
	lock.Lock()
	defer lock.Unlock()
	modeSpeechEnabled = b
}

func ModeSpeechEnabled() bool {
	lock.Lock()
	defer lock.Unlock()
	return modeSpeechEnabled
}

// Dis/allow new sounds from being played
func Mute(b bool) {
	mixer.Mute(b)
}

func GetVolume() float64 {
	return mixer.GetVolume()
}

func SetVolume(vol float64) {
	mixer.SetVolume(vol)
	PlaySound(volumeSound)
}

func LedIDByName(ledName string) pidp11.LedID {
	return pidp11.LedIDByName(ledName)
}

func ClearLeds() {
	pidp11.ClearLeds(100)
}

// Switches a led on (to provided [0, 1] brightness) or off, using the effect.
// The parameters are effect-specific, and must be in range [0, 1].
func Led(id pidp11.LedID, fx pidp11.Effect, bright float64, fxParams ...float64) {
	lib.Assert(0 <= bright && bright <= 1, "invalid brightness",
		"brightness", bright)
	for i, param := range fxParams {
		lib.Assert(0 <= param && param <= 1, "invalid effect param",
			"param", i, "value", param)
	}
	pidp11.Led(id, bright, fx, fxParams...)
}

func LedError(id pidp11.LedID) {
	pidp11.Led(id, 1, pidp11.NewErrorEffect())
}

func NewSimpleEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewSimpleEffect(onMs, offMs)
}

func NewFlashEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewFlashEffect(onMs, offMs)
}

func NewStrobeEffect(onMs, offMs int) pidp11.Effect {
	return pidp11.NewStrobeEffect(onMs, offMs)
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
