package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Debug       []string
	Memory_path string
	Pidp        ConfigPidp
	Server      ConfigServer
	Audio       ConfigAudio
	Sections    ConfigSections
}

type ConfigPidp struct {
	Brightness struct {
		Min, Max float64
	}
	Frequencies struct {
		One_hz_at, Min_hz, Max_hz float64
	}
	Speak bool
}

type ConfigServer struct {
	Addr string
}

type ConfigAudio struct {
	Impl              string
	Rate              int
	Latency_ms        int
	Sounds_dir        string
	Cache_dir         string
	Tts_language      string
	Tts_volume_factor float64
	Tts_speed_factor  float64
	Volume_sound      string
	Knob_ok_sound     string
	Knob_ko_sound     string
	Startup_sound     string
}

type ConfigSections []struct {
	Section string
	Hidden  bool
	Modes   []ConfigMode
}

type ConfigMode struct {
	Knobs   []int
	Name    string
	Speech  string
	Feeds   []string
	Imports []string
	Command []string
	Control struct {
		Argv             []string
		Ticker_period_ms int
		Autostart        bool
	}
	Meters map[string]ConfigMeter
}

type ConfigMeter struct {
	Leds        []string
	Type          string
	On_ms, Off_ms int
	Floor         bool
	Sound         string
	Gate          float64
}

func ParseConfig(path string) Config {
	yml, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	cfg := Config{}
	err = yaml.Unmarshal([]byte(yml), &cfg)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return cfg
}
