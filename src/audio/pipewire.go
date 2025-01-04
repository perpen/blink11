package audio

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/lib"
)

type pipewireBackend struct {
	out     io.Writer
	logger  *slog.Logger
	bufSize int
}

func newPipewireBackend(cfg config.ConfigAudio, logger *slog.Logger) (Backend, error) {
	p := pipewireBackend{
		logger: logger,
		// bufSize: rate / 1000 * latencyMs,
		bufSize: upperPowerOf2(cfg.Rate / 1000 * cfg.Latency_ms),
	}
	latencyMs := cfg.Latency_ms
	rate := cfg.Rate

	logger.Debug("newPipewireBackend", "rate", rate, "latencyMs", latencyMs, "buffer", p.bufSize)

	cmd := exec.Command(
		"pw-play",
		fmt.Sprintf("--latency=%dms", latencyMs),
		fmt.Sprintf("--rate=%d", rate),
		"--format=s16",
		"--channels=1",
		"-")
	out, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf(`pw-play StdinPipe: %+v`, err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err = cmd.Start()
	if err != nil {
		log.Fatalf(`failed to start pw-play: %+v`, err)
	}

	p.out = out
	return p, nil
}

func (p pipewireBackend) Output() io.Writer {
	return p.out
}

func (p pipewireBackend) BufSize() int {
	return p.bufSize
}

var WPCTL_VOLUME_RE = regexp.MustCompile(`Volume: ([0-9.]+)`)

func (p pipewireBackend) GetVolume() float64 {
	out := run("wpctl", "get-volume", "@DEFAULT_AUDIO_SINK@")
	p.logger.Debug("get volume", "output", string(out))
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

func (p pipewireBackend) SetVolume(level float64) {
	lib.Assert(level >= 0 && level <= 1, "invalid volume", "level", level)
	intLevel := int(level * 100)
	run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", fmt.Sprintf("%d%%", intLevel))
}

func upperPowerOf2(n int) int {
	for i := 1; ; i++ {
		p := math.Pow(2, float64(i))
		if p > float64(n) {
			return int(p)
		}
	}
}
