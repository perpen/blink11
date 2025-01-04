package audio

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// Simple audio mixer piping into pipewire command pw-play
type Pipewire struct {
	sync.Mutex
	rate         int
	latencyMs    int
	dir          string
	tmpDir       string
	ttsLanguage  string
	vol          float64
	sounds       map[int][]int
	soundByName  map[string]int
	streams      map[int]*Stream
	nextStreamID int
	muted        bool
	debug        func(format string, args ...interface{})
}

const SPEECH_STREAM_ID = 0

type Stream struct {
	soundID int
	offset  int
}

func NewPipewire(rate, latencyMs int, dir, tmpDir, ttsLanguage string,
	debug func(format string, args ...interface{})) (Audio, error) {

	if rate <= 0 {
		log.Fatalf("NewPipewire: invalid rate %d", rate)
	}
	if latencyMs <= 0 {
		log.Fatalf("NewPipewire: invalid latencyMs %d", latencyMs)
	}
	if tmpDir == "" {
		log.Fatal("NewPipewire: missing tmpDir config")
	}
	if ttsLanguage == "" {
		log.Fatal("NewPipewire: missing ttsLanguage config")
	}
	err := os.MkdirAll(tmpDir, 0744)
	if err != nil {
		return nil, err
	}
	a := Pipewire{
		rate:         rate,
		latencyMs:    latencyMs,
		dir:          dir,
		tmpDir:       tmpDir,
		ttsLanguage:  ttsLanguage,
		vol:          1,
		sounds:       make(map[int][]int, 0),
		soundByName:  make(map[string]int, 0),
		streams:      make(map[int]*Stream, 0),
		nextStreamID: 1,
		debug:        debug,
	}
	return &a, nil
}

func (a *Pipewire) Mute(b bool) {
	a.muted = b
}

func run(argv []string) []byte {
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	if err != nil {
		log.Fatalf(`failed to execute "%v" (%+v)`, argv, err)
	}
	return out
}

var WPCTL_VOLUME_RE = regexp.MustCompile(`Volume: ([0-9.]+)`)

func (a *Pipewire) GetVolume() float64 {
	out := run([]string{"wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@"})
	a.debug("get: %s", string(out))
	found := WPCTL_VOLUME_RE.FindStringSubmatch(string(out))
	if found == nil {
		log.Fatalf("cannot find volume regexp in '%s'", string(out))
	}
	vol, err := strconv.ParseFloat(found[1], 32)
	if err != nil {
		log.Fatalf("cannot parse float '%s'", found[1])
	}
	return vol
}

func (a *Pipewire) SetVolume(level float64) {
	intLevel := int(level * 100)
	argv := []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", fmt.Sprintf("%d%%", intLevel)}
	run(argv)
}

func (a *Pipewire) Start() {
	a.debug("Pipewire.Start()\n")
	// bufSize := a.rate / 1000 * a.latencyMs * 1
	bufSize := upperPowerOf2(a.rate / 1000 * a.latencyMs)
	a.debug("rate=%d latencyMs=%d bufSize=%d", a.rate, a.latencyMs, bufSize)
	cmd := exec.Command(
		"pw-play",
		fmt.Sprintf("--latency=%dms", a.latencyMs),
		fmt.Sprintf("--rate=%d", a.rate),
		"-")
	out, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf(`pw-play StdinPipe: %+v`, err)
	}
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stdout
	err = cmd.Start()
	if err != nil {
		log.Fatalf(`failed to start pw-play: %+v`, err)
	}

	go func() {
		concurrentStreams := 0
		ticker := time.NewTicker(time.Duration(a.latencyMs) * time.Millisecond)
		zeroBlock := make([]int, bufSize)
		block := make([]int, bufSize)
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
				a.debug("%d concurrent streams", streamsCount)
				concurrentStreams = streamsCount
			}
			deadStreams := make([]int, 0)
			copy(block, zeroBlock)
			// Mix all streams
			for sid, stream := range a.streams {
				sound := a.sounds[stream.soundID]
				soundLen := len(sound)
				off := stream.offset
				end := off + bufSize
				if end > soundLen-1 {
					end = soundLen - 1
				}
				// a.debug("stream %d sound %d off %d end %d progress %v%%\n", sid, stream.soundID, off, end, math.Round(100.0*float64(off)/float64(soundLen)))
				for i := off; i < end-1; i++ {
					block[i-off] += sound[i]
				}
				if end >= soundLen-1 {
					deadStreams = append(deadStreams, sid)
				} else {
					stream.offset = end
				}
			}
			for _, sid := range deadStreams {
				// a.debug("stream end: sound %d, stream %d\n", a.streams[sid].soundID, sid)
				delete(a.streams, sid)
			}
			out.Write(intsToBytes(block[:]))
			a.Unlock()
			loopDur := time.Now().Sub(t0)
			if int(loopDur.Milliseconds()) > a.latencyMs {
				a.debug("slow loop: %v", loopDur)
			}
		}
	}()
}

func (a *Pipewire) Stop() {
}

func (a *Pipewire) Load(name string) (int, error) {
	if name == "" {
		return 0, nil
	}
	if id, ok := a.soundByName[name]; ok {
		return id, nil
	}
	if strings.HasPrefix(name, "tts:") {
		text := strings.TrimPrefix(name, "tts:")
		id := a.speechID(text)
		return id, nil
	}
	path := a.dir + "/" + name
	if _, err := os.Stat(path); err != nil {
		return 0, fmt.Errorf("sound file not found: %s", path)
	}
	return a.loadFile(name, path)
}

func (a *Pipewire) loadFile(name, path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	a.Lock()
	defer a.Unlock()
	id := len(a.sounds) + 1
	ints := bytesToInts(b)
	a.sounds[id] = ints
	a.soundByName[name] = id
	// a.debug("loadFile(%s): %d\n", name, len(a.sounds[id]))
	return id, nil
}

// returns stream id
func (a *Pipewire) Play(id int) int {
	if a.muted {
		a.debug("Play %d (muted)\n", id)
		return -1
	}
	a.Lock()
	streamID := a.nextStreamID
	a.nextStreamID++
	a.Unlock()
	a.playOnStream(id, streamID)
	a.debug("Play sound %d, stream %d\n", id, streamID)
	return streamID
}

// returns stream id
func (a *Pipewire) playOnStream(id, streamID int) int {
	a.Lock()
	defer a.Unlock()
	a.streams[streamID] = &Stream{
		soundID: id,
		offset:  0,
	}
	a.debug("playOnStream sound %d, stream %d\n", id, streamID)
	return streamID
}

func (a *Pipewire) Speak(text string) int {
	id := a.speechID(text)
	return a.playOnStream(id, SPEECH_STREAM_ID)
}

// returns sound id
func (a *Pipewire) speechID(text string) int {
	name := url.QueryEscape(text)
	cached := fmt.Sprintf("%s/%s-%s.raw", a.tmpDir, a.ttsLanguage, name)
	err := ttsApi(a.ttsLanguage, text, cached)
	if err != nil {
		a.debug("ttsApi error: %v", err)
		return 0
	}
	id, err := a.loadFile(name, cached)
	if err != nil {
		log.Fatalf("loadFile error: %v", err)
	}
	return id
}

// Gets an mp3 from the translate api and stores it in path
func ttsApi(lang, text, path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	mp3Path := strings.TrimSuffix(path, ".raw") + ".mp3"
	mp3Writer, err := os.Create(mp3Path)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=32&client=tw-ob&q=%s&tl=%s",
		url.QueryEscape(text), lang)
	response, err := http.Get(url)
	if err != nil {
		os.Remove(mp3Path)
		return err
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status != 200 {
		return fmt.Errorf("translate_tts gave status %d", status)
	}
	_, err = io.Copy(mp3Writer, response.Body)
	mp3Writer.Close()
	if err != nil {
		os.Remove(mp3Path)
		return err
	}

	err = exec.Command("./scripts/audio-convert.sh", mp3Path, ".5").Run()
	if err != nil {
		log.Fatalf(`audio-convert.sh error: %+v`, err)
	}
	err = os.Remove(mp3Path)
	if err != nil {
		log.Fatalf(`Remove %s: %+v`, mp3Path, err)
	}
	return nil
}

func upperPowerOf2(n int) int {
	for i := 1; ; i++ {
		p := math.Pow(2, float64(i))
		if p > float64(n) {
			return int(p)
		}
	}
}

func bytesToInts(b []byte) []int {
	p := unsafe.Pointer(unsafe.SliceData(b))
	return unsafe.Slice((*int)(p), len(b)/4)
}

func intsToBytes(i []int) []byte {
	p := unsafe.Pointer(unsafe.SliceData(i))
	return unsafe.Slice((*byte)(p), len(i)*4)
}
