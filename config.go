package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Logging []string
	Manager struct {
		Memory string
	}
	Pidp struct {
		Negate_knoba, Negate_knobd bool
		Brightness                 struct {
			Min, Max, Initial_scaling float64
		}
		Frequencies struct {
			One_hz_at, Min_hz, Max_hz float64
		}
	}
	Server struct {
		Addr string
	}
	Audio struct {
		Impl             string
		Rate             int
		Latency_ms       int
		Volume           float64
		Dir, Tmp_dir     string
		Tts_lang         string
		Knob_ok, Knob_ko string
		Startup_sound    string
	}
	Sections []struct {
		Section string
		Hidden  bool
		Modes   []struct {
			Knobs    []int
			Name     string
			Speech   string
			Feeds    []string
			Inherits []string
			Command  []string
			Meters   map[string]struct {
				Lights        []string
				Type          string
				On_ms, Off_ms int
				Floor         bool
				Sound         string
			}
		}
	}
}

func Parse() Config {
	yml, err := os.ReadFile("blink11.yaml")
	if err != nil {
		panic(err)
	}
	//yml = []byte(strings.ReplaceAll(string(yml), "	", "  "))
	cfg := Config{}
	err = yaml.Unmarshal([]byte(yml), &cfg)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return cfg
}
