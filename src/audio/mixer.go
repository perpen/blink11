package audio

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/lib"
)

type Mixer struct {
	sync.Mutex
	backend         Backend
	latencyMs       int
	dir             string
	tmpDir          string
	ttsLanguage     string
	ttsVolumeFactor float64
	ttsSpeedFactor  float64
	sounds          map[int]*Sound
	soundByName     map[string]*Sound
	streams         map[int]*Stream
	nextStreamID    int
	muted           bool
	out             io.Writer
	logger          *slog.Logger
}

type Backend interface {
	Output() io.Writer
	BufSize() int
	GetVolume() float64
	SetVolume(float64)
}

const SPEECH_STREAM_ID = 0

type Sound struct {
	name     string // for logging
	data     []int16
	isSpeech bool
}

func (snd Sound) String() string {
	return fmt.Sprintf("Sound[name:%s isSpeech:%v]", snd.name, snd.isSpeech)
}

type Stream struct {
	sound  *Sound
	offset int
	done   chan bool
}

func NewMixer(cfg config.ConfigAudio, logger *slog.Logger) (*Mixer, error) {
	var backend Backend
	var err error
	switch cfg.Impl {
	case "none":
	case "pipewire":
		backend, err = newPipewireBackend(cfg, logger)
	default:
		log.Fatalf("NewMixer: invalid backend: '%s'", cfg.Impl)
	}
	if err != nil {
		log.Fatalf("NewMixer: cannot create backend %s: %v", cfg.Impl, err)
	}

	if cfg.Rate <= 0 {
		log.Fatalf("NewMixer: invalid rate %d", cfg.Rate)
	}
	if cfg.Latency_ms <= 0 {
		log.Fatalf("NewMixer: invalid latencyMs %d", cfg.Latency_ms)
	}
	if cfg.Cache_dir == "" {
		log.Fatal("NewMixer: missing cache_dir config")
	}
	if cfg.Tts_language == "" {
		log.Fatal("NewMixer: missing tts_language config")
	}
	ttsVolumeFactor := cfg.Tts_volume_factor
	lib.Check(ttsVolumeFactor >= 0 && ttsVolumeFactor <= 1,
		"NewMixer: tts_volume_factor should be in range [0, 1]")
	if ttsVolumeFactor == 0 {
		ttsVolumeFactor = .5
	}
	ttsSpeedFactor := cfg.Tts_speed_factor
	if ttsSpeedFactor == 0 {
		ttsSpeedFactor = 1
	}
	lib.Check(ttsSpeedFactor > 0 && ttsSpeedFactor <= 2,
		"NewMixer: tts_speed_factor should be in range ]0, 2]")

	err = os.MkdirAll(cfg.Cache_dir, 0744)
	if err != nil {
		return nil, err
	}

	a := Mixer{
		backend:         backend,
		latencyMs:       cfg.Latency_ms,
		dir:             cfg.Sounds_dir,
		tmpDir:          cfg.Cache_dir,
		ttsLanguage:     cfg.Tts_language,
		ttsVolumeFactor: ttsVolumeFactor,
		ttsSpeedFactor:  ttsSpeedFactor,
		sounds:          make(map[int]*Sound, 0),
		soundByName:     make(map[string]*Sound, 0),
		streams:         make(map[int]*Stream, 0),
		nextStreamID:    1,
		logger:          logger,
	}
	return &a, nil
}

func (a *Mixer) GetVolume() float64 {
	if a.backend == nil {
		return 1
	}
	return a.backend.GetVolume()
}

func (a *Mixer) SetVolume(vol float64) {
	if a.backend == nil {
		return
	}
	a.backend.SetVolume(vol)
}

// Does not affect currently playing sounds, just prevents future play.
func (a *Mixer) Mute(b bool) {
	a.muted = b
}

func (a *Mixer) Init() {
	a.logger.Debug("init")
	if a.backend == nil {
		return
	}
	bufSize := a.backend.BufSize()
	out := a.backend.Output()

	// Mixing
	go func() {
		concurrentStreams := 0
		ticker := time.NewTicker(time.Duration(a.latencyMs) * time.Millisecond)
		zeroBlock := make([]int16, bufSize)
		block := make([]int16, bufSize)
		for {
			<-ticker.C
			t0 := time.Now()
			a.Lock()
			streamsCount := len(a.streams)
			if streamsCount == 0 {
				a.Unlock()
				continue
			}
			if streamsCount > 10 && streamsCount != concurrentStreams {
				a.logger.Warn("many concurrent streams", "count", streamsCount)
				concurrentStreams = streamsCount
			}
			deadStreams := make([]int, 0)
			copy(block, zeroBlock)
			for sid, stream := range a.streams {
				sound := stream.sound
				data := sound.data
				soundLen := len(data)
				off := stream.offset
				end := min(off+bufSize, soundLen-1)
				for i := off; i < end-1; i++ {
					block[i-off] += data[i]
				}
				if end >= soundLen-1 {
					deadStreams = append(deadStreams, sid)
				} else {
					stream.offset = end
				}
			}
			for _, sid := range deadStreams {
				streamDone := a.streams[sid].done
				if streamDone != nil {
					a.streams[sid].done <- true
				}
				delete(a.streams, sid)
			}
			out.Write(castIntsToBytes(block[:]))
			a.Unlock()
			loopDur := time.Now().Sub(t0)
			if int(loopDur.Milliseconds()) > a.latencyMs {
				a.logger.Warn("slow mixer loop", "duration", loopDur)
			}
		}
	}()
}

// Loads the audio from the signed 16 bits/mono file with the given name,
// located under the sounds directory.
func (a *Mixer) Load(name string) (*Sound, error) {
	if name == "" {
		return nil, nil
	}
	if sound, ok := a.soundByName[name]; ok {
		return sound, nil
	}
	if strings.HasPrefix(name, "tts:") {
		text := strings.TrimPrefix(name, "tts:")
		sound := a.speechID(text)
		return sound, nil
	}
	path := a.dir + "/" + name
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("sound file not found: %s", path)
	}
	return a.loadFile(name, path)
}

func (a *Mixer) loadFile(name, path string) (*Sound, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	a.Lock()
	defer a.Unlock()
	id := len(a.sounds) + 1
	ints := castBytesToInts(b)
	sound := &Sound{
		name: name,
		data: ints,
	}
	a.sounds[id] = sound
	a.soundByName[name] = sound
	return sound, nil
}

func (a *Mixer) Play(sound *Sound) {
	if sound == nil || a.muted {
		return
	}
	var streamID int
	if sound.isSpeech {
		streamID = SPEECH_STREAM_ID
	} else {
		a.Lock()
		streamID = a.nextStreamID
		a.nextStreamID++
		a.Unlock()
	}
	a.playOnStream(sound, streamID, false)
	a.logger.Debug("Play", "sound", sound, "streamID", streamID)
}

func (a *Mixer) playOnStream(sound *Sound, streamID int, sync bool) {
	a.Lock()
	stream := Stream{
		sound:  sound,
		offset: 0,
	}
	if sync {
		stream.done = make(chan bool)
	}
	a.streams[streamID] = &stream
	a.Unlock()
	if sync {
		<-stream.done
	}
}

func (a *Mixer) Speak(text string) {
	a.speak(text, false)
}

func (a *Mixer) SpeakSync(text string) {
	a.speak(text, true)
}

func (a *Mixer) speak(text string, sync bool) {
	id := a.speechID(text)
	a.playOnStream(id, SPEECH_STREAM_ID, sync)
}

func (a *Mixer) speechID(text string) *Sound {
	name := url.QueryEscape(text)
	cached := fmt.Sprintf("%s/%s-%s.raw", a.tmpDir, a.ttsLanguage, name)
	err := ttsApi(a.ttsLanguage, text, cached, a.ttsVolumeFactor, a.ttsSpeedFactor)
	if err != nil {
		a.logger.Debug("ttsApi error", "err", err)
		return nil
	}
	sound, err := a.loadFile(name, cached)
	if err != nil {
		log.Fatalf("loadFile error: %v", err)
	}
	sound.isSpeech = true
	return sound
}

func castBytesToInts(b []byte) []int16 {
	p := unsafe.Pointer(unsafe.SliceData(b))
	return unsafe.Slice((*int16)(p), len(b)/2)
}

func castIntsToBytes(i []int16) []byte {
	p := unsafe.Pointer(unsafe.SliceData(i))
	return unsafe.Slice((*byte)(p), len(i)*2)
}

func run(argv ...string) []byte {
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	if err != nil {
		log.Fatalf(`failed to execute "%v" (%+v)`, argv, err)
	}
	return out
}
