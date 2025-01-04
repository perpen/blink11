package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"hfdom.org/blink11/v2/pidp11"
)

// A Mode defines the presentation of its metrics
// It has its own memory space, which can be edited in HALT mode, and
// used by its command.
type Mode struct {
	name         string
	speech       string
	feeds        []string
	meters       map[string]*Meter
	mem          *Memory
	knoba, knobd int // Knob values selecting this mode
}

func (mode *Mode) String() string {
	return fmt.Sprintf("Mode[name=%s, feeds=%v, meters=%v)",
		mode.name, mode.feeds, mode.meters)
}

// Parse the config to create the modes.
// Panic on invalid config.
func makeModes(cfg Config, cons *Console, mem *Memory,
	cc *CommandCenter) map[string]*Mode {

	lightNamesToIDs := func(lightNames []string) []pidp11.LedID {
		lightIDs := make([]pidp11.LedID, len(lightNames))
		for i, lightName := range lightNames {
			id, ok := cons.LedIDByName(lightName)
			assert(ok, "bad config: invalid light name: %s", lightName)
			lightIDs[i] = id
		}
		return lightIDs
	}
	simpleFx := cons.NewSimpleEffect(0, 0)
	knobAMeter := NewMeter(updateDot, cons, KnobAddrLeds, simpleFx, "")
	knobDMeter := NewMeter(updateDot, cons, KnobDataLeds, simpleFx, "")

	nameToInherits := make(map[string][]string, 0)
	nameToMode := make(map[string]*Mode, 0)
	sectionNum := 0
	for _, section := range cfg.Sections {
		sectionName := section.Section
		hidden := section.Hidden
		modeNum := 0
		for _, cfgMode := range section.Modes {
			Debug(LOG_MODE, "cfgMode: %s/%v\n", sectionName, cfgMode)
			name := cfgMode.Name
			assert(name != "", "mode without a name")
			_, ok := nameToMode[name]
			assert(!ok, "bad config: duplicate mode name: %s", name)

			knoba := sectionNum
			knobd := modeNum
			if hidden {
				knoba = -1
				knobd = -1
			} else {
				assert(knoba <= 7 && knobd <= 3,
					"exhausted knob positions for mode %s: %d, %d",
					name, knoba, knobd)
			}

			feeds := cfgMode.Feeds
			if feeds == nil {
				feeds = make([]string, 0)
			}

			meters := make(map[string]*Meter, 0)
			for meterName, cfgMeter := range cfgMode.Meters {
				lightIDs := lightNamesToIDs(cfgMeter.Lights)
				onMs := cfgMeter.On_ms
				offMs := cfgMeter.Off_ms
				var fx pidp11.Effect
				var update func(*Meter, Metric)
				switch cfgMeter.Type {
				case "lumen":
					update = updateLumen
					fx = cons.NewSimpleEffect(onMs, offMs)
				case "bar":
					if cfgMeter.Floor {
						update = updateBarFloor
					} else {
						update = updateBar
					}
					fx = cons.NewSimpleEffect(onMs, offMs)
				case "dot":
					if cfgMeter.Floor {
						update = updateDotFloor
					} else {
						update = updateDot
					}
					fx = cons.NewSimpleEffect(onMs, offMs)
				case "binary":
					update = updateBinary
					fx = cons.NewSimpleEffect(onMs, offMs)
				case "flash":
					update = updateFlashOrStrobe
					fx = cons.NewFlashEffect(onMs, offMs)
				case "strobe":
					update = updateFlashOrStrobe
					fx = cons.NewStrobeEffect(onMs, offMs)
				default:
					assert(false, "bad config: unknown meter: %s", cfgMeter.Type)
				}
				meters[meterName] = NewMeter(update, cons, lightIDs,
					fx, cfgMeter.Sound)
			}
			if !hidden || name == "_levels" {
				meters["knoba"] = knobAMeter
				meters["knobd"] = knobDMeter
			}

			inherits := cfgMode.Inherits
			if inherits != nil && len(inherits) > 0 {
				nameToInherits[name] = inherits
			}

			speech := cfgMode.Speech
			if speech == "" {
				speech = name
			}
			mode := Mode{
				name:   name,
				speech: speech,
				feeds:  feeds,
				meters: meters,
				knoba:  knoba,
				knobd:  knobd,
				mem:    mem,
			}

			if cfgMode.Command != nil && len(cfgMode.Command) > 0 {
				cc.RegisterCommand(mode.name, cfgMode.Command)
			}

			Debug(LOG_MODE, "mode=%v", mode)
			nameToMode[cfgMode.Name] = &mode
			if !hidden {
				modeNum++
			}
		}
		if !hidden {
			sectionNum++
		}
	}

	reflectModesInheritance(nameToMode, nameToInherits)
	validateModes(nameToMode)

	Debug(LOG_MODE, "modes:\n")
	for _, mode := range nameToMode {
		Debug(LOG_MODE, "- %v\n", mode)
	}
	Debug(LOG_MODE, "knobs to mode:\n")
	for _, mode := range nameToMode {
		if mode.Knobbable() {
			Debug(LOG_MODE, "- %d,%d → %s\n",
				mode.knoba, mode.knobd, mode.name)
		}
	}

	return nameToMode
}

// xxx walking
func reflectModesInheritance(modes map[string]*Mode, nameToInherits map[string][]string) {
	var walk func(name string, seen map[string]bool)
	walk = func(name string, seen map[string]bool) {
		_, ok := seen[name]
		assert(!ok, "cycle on mode: %s", name)
		seen[name] = true
		mode := modes[name]
		for _, inherit := range nameToInherits[name] {
			walk(inherit, seen)
			used, ok := modes[inherit]
			assert(ok, "mode %s inherits unknown mode %s", name, inherit)
			mode.feeds = append(mode.feeds, used.feeds...)
			for meter_name, meter := range used.meters {
				if _, ok := mode.meters[meter_name]; !ok {
					mode.meters[meter_name] = meter
				}
			}
		}

	}

	for name := range modes {
		seen := make(map[string]bool, 0)
		walk(name, seen)
	}
}

// Panic on invalid mode
func validateModes(modes map[string]*Mode) {
	bad := false

	// Check for lights used more than once in a mode
	for modeName, mode := range modes {
		ledToMeters := make(map[string][]string, 0)
		for meterName, meter := range mode.meters {
			for _, ledID := range meter.lightIDs {
				ledName := pidp11.LedName(ledID)
				if _, ok := ledToMeters[ledName]; !ok {
					ledToMeters[ledName] = make([]string, 0)
				}
				ledToMeters[ledName] = append(ledToMeters[ledName], meterName)
			}
		}
		for ledName, meters := range ledToMeters {
			if len(meters) > 1 {
				bad = true
				Err("mode '%s': led %s used by multiple meters: %v\n",
					modeName, ledName, strings.Join(meters, ", "))
			}
		}
	}

	if bad {
		log.Fatal("bad config")
		os.Exit(1)
	}
}

func (mode *Mode) Knobbable() bool {
	return mode.knoba >= 0
}

func (mode *Mode) Read(addr uint) uint {
	return mode.mem.Read(mode.name, addr)
}

func (mode *Mode) Write(addr, data uint) {
	mode.mem.Write(mode.name, addr, data)
}

func (mode *Mode) UsedAddresses() []uint {
	return mode.mem.UsedAddresses(mode.name)
}
