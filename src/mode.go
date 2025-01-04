package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/console"
	"github.com/perpen/blink11/lib"
	"github.com/perpen/blink11/logging"
	"github.com/perpen/blink11/memory"
	"github.com/perpen/pidp11"
)

// A Mode defines the presentation of its metrics
// It has its own memory space, which can be edited in HALT mode, and
// used by its command.
type Mode struct {
	sync.Mutex
	name         string
	speech       string
	feeds        []string
	meters       map[string]*Meter
	isSystem     bool // system modes are not selected via knobs
	knoba, knobd int  // knob positions for selecting this mode
	bus          Unibus
	// The event handler for the mode get passed an event and returns:
	// - the same event, or another
	// - a boolean indicating if we should leave the mode and go back to the previous one
	// When switching to the mode, an initial zero-event is sent, which can
	// be intercepted and used for mode initialisation.
	handler   func(evt pidp11.Event) (pidp11.Event, bool)
	start     func()
	running   bool
	modeParam *Mode // xxx only for entry mode
}

func (mode *Mode) String() string {
	return fmt.Sprintf("Mode[name=%s feeds=%v meters=%v)",
		mode.name, mode.feeds, mode.meters)
}

func (mode *Mode) isRunning() bool {
	mode.Lock()
	defer mode.Unlock()
	return mode.running
}

func (mode *Mode) setRunning(b bool) {
	mode.Lock()
	mode.running = b
	mode.Unlock()
	mode.bus <- Control{msg: "updateRUNLed"}
}

func (mode *Mode) read(addr uint) (uint, bool) {
	return memory.Read(mode.name, addr)
}

func (mode *Mode) write(addr, data uint) {
	memory.Write(mode.name, addr, data)
}

func (mode *Mode) usedAddresses() []uint {
	return memory.UsedAddresses(mode.name)
}

func (mode *Mode) usesLed(led pidp11.LedID) bool {
	for _, meter := range mode.meters {
		if slices.Contains(meter.ledIDs, led) {
			return true
		}
	}
	return false
}

// Create the modes from the config.
// Exits on invalid config.
func makeModes(cfg config.ConfigSections, bus Unibus) map[string]*Mode {
	knobAMeter := newDotMeter(float64(0), console.KnobAddrLeds, "", true, 0, 0)
	knobDMeter := newDotMeter(.0, console.KnobDataLeds, "", true, 0, 0)

	// Parse config
	nameToImports := make(map[string][]string, 0)
	nameToMode := make(map[string]*Mode, 0)
	sectionNum := 0
	for _, section := range cfg {
		sectionName := section.Section
		hidden := section.Hidden
		modeNum := 0
		for _, cfgMode := range section.Modes {
			Debug(logging.LOG_MODE, "cfgMode", "section", sectionName, "mode", cfgMode)
			name := cfgMode.Name
			lib.Check(name != "", "mode without a name")
			_, ok := nameToMode[name]
			lib.Check(!ok, "duplicate mode name", "mode", name)

			knoba := sectionNum
			knobd := modeNum
			if hidden {
				knoba = -1
				knobd = -1
			} else {
				lib.Check(knoba <= 7 && knobd <= 3,
					"exhausted knob positions for mode",
					"mode", name, "knoba", knoba, "knobd", knobd)
			}

			feeds := cfgMode.Feeds
			if feeds == nil {
				feeds = make([]string, 0)
			}

			meters := make(map[string]*Meter, 0)
			for meterName, cfgMeter := range cfgMode.Meters {
				meters[meterName] = newMeterFromConfig(cfgMeter)
			}
			if !hidden {
				meters["system.knoba"] = knobAMeter
				meters["system.knobd"] = knobDMeter
			}

			imports := cfgMode.Imports
			if imports != nil && len(imports) > 0 {
				nameToImports[name] = imports
			}

			speech := cfgMode.Speech
			if speech == "" {
				speech = name
			}
			mode := &Mode{
				name:   name,
				speech: speech,
				feeds:  feeds,
				meters: meters,
				knoba:  knoba,
				knobd:  knobd,
				bus:    bus,
				start:  func() {},
			}
			if cfgMode.Control.Argv != nil {
				argv := cfgMode.Control.Argv
				tickerPeriodMs := cfgMode.Control.Ticker_period_ms
				if tickerPeriodMs == 0 {
					tickerPeriodMs = 1000
				}
				autostart := cfgMode.Control.Autostart
				mode = newControllerMode(mode, argv, tickerPeriodMs, autostart)
			}

			// Add system meters, if their leds are not used by the config
			if !mode.usesLed(pidp11.LED_MASTER) {
				mode.meters[counterMetric] = newLumenMeter(0,
					[]pidp11.LedID{pidp11.LED_MASTER}, "", 300, 300)
			}

			Debug(logging.LOG_MODE, "created mode", "mode", mode) //xxx
			nameToMode[cfgMode.Name] = mode
			if !hidden {
				modeNum++
			}
		}
		if !hidden {
			sectionNum++
		}
	}

	// Apply mode imports
	var walk func(name string, seen map[string]bool)
	walk = func(name string, seen map[string]bool) {
		_, ok := seen[name]
		lib.Check(!ok, "import cycle for mode", "mode", name)
		seen[name] = true
		mode := nameToMode[name]
		for _, imported := range nameToImports[name] {
			walk(imported, seen)
			used, ok := nameToMode[imported]
			lib.Check(ok, "mode imports unknown mode", "mode", name, "imports", imported)
			mode.feeds = append(mode.feeds, used.feeds...)
			for meter_name, meter := range used.meters {
				if _, ok := mode.meters[meter_name]; !ok {
					mode.meters[meter_name] = meter
				}
			}
		}
	}
	for name := range nameToMode {
		seen := make(map[string]bool, 0)
		walk(name, seen)
	}

	// Check no led is used by more than one meter
	for modeName, mode := range nameToMode {
		ledToMeters := make(map[string][]string, 0)
		for meterName, meter := range mode.meters {
			for _, ledID := range meter.ledIDs {
				ledName := pidp11.LedName(ledID)
				if _, ok := ledToMeters[ledName]; !ok {
					ledToMeters[ledName] = make([]string, 0)
				}
				ledToMeters[ledName] = append(ledToMeters[ledName], meterName)
			}
		}
		for ledName, meters := range ledToMeters {
			lib.Check(len(meters) == 1,
				"led used by multiple meters",
				"mode", modeName, "led", ledName, "meters", strings.Join(meters, ", "))
		}
	}

	Debug(logging.LOG_MODE, "modes:")
	for _, mode := range nameToMode {
		Debug(logging.LOG_MODE, "-", "mode", mode)
	}
	Debug(logging.LOG_MODE, "knobs to mode:")
	for _, mode := range nameToMode {
		if !mode.isSystem {
			Debug(logging.LOG_MODE, "-", "knoba", mode.knoba, "knobd", mode.knobd,
				"mode", mode.name)
		}
	}

	return nameToMode
}

// Mode invoked when pressing the TEST switch.
func newTestMode(bus Unibus) *Mode {
	mode := Mode{
		isSystem: true,
		name:     "system.test",
		speech:   "lamp test",
		bus:      bus,
		meters: map[string]*Meter{
			"system.test.flash_slow":  newFlashMeter(0, []pidp11.LedID{pidp11.LED_DATA_PATHS}, "", 0, 0),
			"system.test.flash_fast":  newFlashMeter(0, []pidp11.LedID{pidp11.LED_μADR_FPP_CPU}, "", 0, 0),
			"system.test.strobe_slow": newStrobeMeter(0, []pidp11.LedID{pidp11.LED_BUS_REG}, "", 0, 0),
			"system.test.strobe_fast": newStrobeMeter(0, []pidp11.LedID{pidp11.LED_DISPLAY_REGISTER}, "", 0, 0),
			"system.test.on_ms":       newStrobeMeter(0, []pidp11.LedID{pidp11.LED_PAR_LO}, "", 1000, 0),
			"system.test.off_ms":      newStrobeMeter(0, []pidp11.LedID{pidp11.LED_PAR_HI}, "", 0, 1000),
			"system.test.error":       newBarMeter(0, []pidp11.LedID{pidp11.LED_ADRS_ERR, pidp11.LED_PAR_ERR}, "", 0, 0, false),
			"system.test.bar":         newBarMeter(0, []pidp11.LedID{pidp11.LED_ADDR_22, pidp11.LED_ADDR_18, pidp11.LED_ADDR_16, pidp11.LED_DATA, pidp11.LED_KERNEL, pidp11.LED_SUPER, pidp11.LED_USER, pidp11.LED_MASTER, pidp11.LED_PAUSE, pidp11.LED_RUN, pidp11.LED_A0, pidp11.LED_A1, pidp11.LED_A2, pidp11.LED_A3, pidp11.LED_A4, pidp11.LED_A5, pidp11.LED_A6, pidp11.LED_A7, pidp11.LED_A8, pidp11.LED_A9, pidp11.LED_A10, pidp11.LED_A11, pidp11.LED_A12, pidp11.LED_A13, pidp11.LED_A14, pidp11.LED_A15, pidp11.LED_A16, pidp11.LED_A17, pidp11.LED_A18, pidp11.LED_A19, pidp11.LED_A20, pidp11.LED_A21, pidp11.LED_CONS_PHY, pidp11.LED_KERNEL_D, pidp11.LED_SUPER_D, pidp11.LED_USER_D, pidp11.LED_USER_I, pidp11.LED_SUPER_I, pidp11.LED_KERNEL_I, pidp11.LED_PROG_PHY}, "", 0, 0, false),
			"system.test.bright1":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D0}, "", 0, 0),
			"system.test.bright2":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D1}, "", 0, 0),
			"system.test.bright3":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D2}, "", 0, 0),
			"system.test.bright4":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D3}, "", 0, 0),
			"system.test.bright5":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D4}, "", 0, 0),
			"system.test.bright6":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D5}, "", 0, 0),
			"system.test.bright7":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D6}, "", 0, 0),
			"system.test.bright8":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D7}, "", 0, 0),
			"system.test.bright9":     newLumenMeter(0, []pidp11.LedID{pidp11.LED_D8}, "", 0, 0),
			"system.test.bright10":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D9}, "", 0, 0),
			"system.test.bright11":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D10}, "", 0, 0),
			"system.test.bright12":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D11}, "", 0, 0),
			"system.test.bright13":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D12}, "", 0, 0),
			"system.test.bright14":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D13}, "", 0, 0),
			"system.test.bright15":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D14}, "", 0, 0),
			"system.test.bright16":    newLumenMeter(0, []pidp11.LedID{pidp11.LED_D15}, "", 0, 0),
		},
		start: func() {
			bus <- Metric{name: "system.test.bar", val: 1, max: 1}
			bus <- Metric{name: "system.test.flash_slow", val: 1, max: 10}
			bus <- Metric{name: "system.test.flash_fast", val: 1, max: 1}
			bus <- Metric{name: "system.test.strobe_slow", val: 1, max: 10}
			bus <- Metric{name: "system.test.strobe_fast", val: 1, max: 1}
			bus <- Metric{name: "system.test.on_ms", val: 1, max: 15}
			bus <- Metric{name: "system.test.off_ms", val: 1, max: 15}
			bus <- Metric{name: "system.test.error", err: "oops"}
			for i := 1; i <= 16; i++ {
				name := fmt.Sprintf("system.test.bright%d", i)
				met := Metric{name: name, val: i, max: 16}
				bus <- met
			}
		},
		handler: func(evt pidp11.Event) (pidp11.Event, bool) {
			switch evt.ID {
			case pidp11.SS_TEST: // exit test mode
				if !evt.On {
					return pidp11.Event{}, true
				}
			case pidp11.SS_KNOBD_PUSH: // exit blink11
				if evt.On {
					exitChan <- nil
					break
				}
			case pidp11.SS_KNOBA_PUSH: // rpi power off
				if evt.On {
					console.EnableModeSpeech(true)
					console.SpeakSync("power off")
					err := exec.Command("sudo", "poweroff").Run()
					if err != nil {
						slog.Error("poweroff failed", "err", err)
					}
				}
			}
			return pidp11.Event{}, false
		},
	}
	return &mode
}

// Event handler for the levels mode, for adjusting volume/brightness
// using the knobs.
func newLevelsMode(bus Unibus) *Mode {
	mode := Mode{
		isSystem: true,
		name:     "system.levels",
		speech:   "levels",
		bus:      bus,
		meters: map[string]*Meter{
			"system.volume": newBarMeter(0, []pidp11.LedID{
				pidp11.LED_A21, pidp11.LED_A20, pidp11.LED_A19, pidp11.LED_A18, pidp11.LED_A17, pidp11.LED_A16, pidp11.LED_A15, pidp11.LED_A14, pidp11.LED_A13, pidp11.LED_A12, pidp11.LED_A11, pidp11.LED_A10, pidp11.LED_A9, pidp11.LED_A8, pidp11.LED_A7, pidp11.LED_A6, pidp11.LED_A5, pidp11.LED_A4, pidp11.LED_A3, pidp11.LED_A2, pidp11.LED_A1, pidp11.LED_A0},
				"", 0, 0, false),
			"system.brightness_range": newBarMeter(0, []pidp11.LedID{
				pidp11.LED_D0, pidp11.LED_D1, pidp11.LED_D2, pidp11.LED_D3, pidp11.LED_D4, pidp11.LED_D5, pidp11.LED_D6, pidp11.LED_D7, pidp11.LED_D8, pidp11.LED_D9, pidp11.LED_D10, pidp11.LED_D11, pidp11.LED_D12, pidp11.LED_D13, pidp11.LED_D14, pidp11.LED_D15},
				"", 0, 0, false),
			"system.speech_enabled": newBinaryMeter(0, []pidp11.LedID{pidp11.LED_ADDR_22},
				"", 0, 0),
		},
		start: func() {},
		handler: func(evt pidp11.Event) (pidp11.Event, bool) {
			// Volume between 0% and 100%
			vol := int(console.GetVolume() * 100)
			vol7 := scaleRound(vol, 100, 0, 7)

			// Brightness between 25% and 100%
			bright := console.GetBrightnessScaling()
			if bright < .25 {
				bright = .25
				console.SetBrightnessScaling(bright)
			}
			bright3 := scaleRound(int(bright*100)-25, 75, 0, 3)

			emit := func() {
				bus <- Metric{name: "system.volume", val: vol7, max: 7}
				bus <- Metric{name: "system.brightness_range", val: 10, max: 10}
				bus <- Metric{name: "system.knoba", val: vol7 + 1, max: 8}
				bus <- Metric{name: "system.knobd", val: bright3 + 1, max: 4}
				speech := 0
				if console.ModeSpeechEnabled() {
					speech = 1
				}
				bus <- Metric{name: "system.speech_enabled", val: speech, max: 1}
			}

			boolAsInt := -1
			if evt.On {
				boolAsInt = 1
			}
			switch evt.ID {
			case pidp11.SS_S_INST: // exit mode
				return pidp11.Event{}, true
			case pidp11.SS_KNOBA:
				vol7 += boolAsInt
				if vol7 < 0 || vol7 > 7 {
					console.KnobSound(false)
				} else {
					console.KnobSound(true)
					volFloat := float64(scaleRound(vol7, 7, 0, 100)) / 100.0
					console.SetVolume(volFloat)
					go memory.SaveVolume(volFloat)
					emit()
				}
			case pidp11.SS_KNOBD:
				bright3 += boolAsInt
				if bright3 < 0 || bright3 > 3 {
					console.KnobSound(false)
				} else {
					console.KnobSound(true)
					bright := .25 + scaleFloat(bright3, 3, 0, .75)
					console.SetBrightnessScaling(bright)
					go memory.SaveBrightness(bright)
					emit()
				}
			case pidp11.SS_KNOBA_PUSH:
				if console.ModeSpeechEnabled() {
					console.EnableModeSpeech(false)
				} else {
					console.EnableModeSpeech(true)
					console.SpeakMode("speech enabled")
				}
				go memory.SaveSpeechEnabled(console.ModeSpeechEnabled())
				emit()
			default:
				emit()
			}
			return pidp11.Event{}, false
		},
	}
	return &mode
}

func newEntryMode(bus Unibus) *Mode {
	mode := Mode{
		isSystem: true,
		name:     "entry",
		speech:   "entry",
		bus:      bus,
		meters: map[string]*Meter{
			"entry.address": newBinaryMeter(0,
				[]pidp11.LedID{pidp11.LED_A0, pidp11.LED_A1, pidp11.LED_A2, pidp11.LED_A3, pidp11.LED_A4, pidp11.LED_A5, pidp11.LED_A6, pidp11.LED_A7, pidp11.LED_A8, pidp11.LED_A9, pidp11.LED_A10, pidp11.LED_A11, pidp11.LED_A12, pidp11.LED_A13, pidp11.LED_A14, pidp11.LED_A15, pidp11.LED_A16, pidp11.LED_A17, pidp11.LED_A18, pidp11.LED_A19, pidp11.LED_A20, pidp11.LED_A21},
				"", 0, 0),
			"entry.data": newBinaryMeter(0,
				[]pidp11.LedID{pidp11.LED_D0, pidp11.LED_D1, pidp11.LED_D2, pidp11.LED_D3, pidp11.LED_D4, pidp11.LED_D5, pidp11.LED_D6, pidp11.LED_D7, pidp11.LED_D8, pidp11.LED_D9, pidp11.LED_D10, pidp11.LED_D11, pidp11.LED_D12, pidp11.LED_D13, pidp11.LED_D14, pidp11.LED_D15},
				"", 0, 0),
		},
		start: func() {},
	}

	/*
		From the PDP11/70 handbook:
		11.5 STEP OPERATIONS
		Performing more than one EXAM operation in a row or more than one
		DEP operation in a row results in a STEP-operation. Depressing the
		EXAM Switch after a previous examine of a location displays the con-
		tents of the next location in memory. Raising the DEP Switch after a
		previous deposit into a memory location causes the current contents of
		the Switch Register to be deposited into the next location in memory.
	*/
	var justExamined, justDeposited bool
	var entryAddr, entryData uint // xxx

	mode.handler = func(evt pidp11.Event) (pidp11.Event, bool) {
		lib.Assert(mode.modeParam != nil, "missing modeParam")
		showAddr := func() {
			bus <- Metric{name: "entry.address", val: int(entryAddr), max: 1 << 21}
		}
		showData := func() {
			entryData, _ = mode.modeParam.read(entryAddr)
			bus <- Metric{name: "entry.data", val: int(entryData), max: 1 << 16}
		}

		if evt.IsZero() { // init message
			entryAddr = 0
			showAddr()
			showData()
			return pidp11.Event{}, true
		}

		switch evt.ID {
		case pidp11.SS_LOAD: // set address
			entryAddr = console.ReadRegSwitches()
			showAddr()
		case pidp11.SS_EXAM: // show address content
			if justExamined {
				// Repeated EXAM shows content of next address
				entryAddr++
				showAddr()
			}
			showData()
		case pidp11.SS_DEP: // store register value into address
			if justDeposited {
				entryAddr++
				showAddr()
			}
			data := console.ReadRegSwitches() &^ (0xFF << 16)
			Debug(logging.LOG_HANDLER, "EntryMode.handler", "data", data)
			mode.modeParam.write(entryAddr, data)
			showData()
		case pidp11.SS_KNOBA_PUSH:
			console.Speak(fmt.Sprintf("%d", entryAddr))
		case pidp11.SS_KNOBD_PUSH:
			console.Speak(fmt.Sprintf("%d", entryData))
		case pidp11.SS_ENABLE: // exit mode
			return pidp11.Event{}, true
		}
		justExamined = evt.ID == pidp11.SS_EXAM
		justDeposited = evt.ID == pidp11.SS_DEP
		return pidp11.Event{}, false
	}
	return &mode
}

// Mode driven by an external command.
// The argv command is run when the mode is selected.
// The START switch is used to toggle the "running" state.
// When running, a `tick` message is sent to the command with the
// given periodicity.
func newControllerMode(model *Mode, argv []string, tickerPeriodMs int, autostart bool) *Mode {
	mode := Mode{
		isSystem: false,
		name:     model.name,
		speech:   model.speech,
		feeds:    model.feeds,
		meters:   model.meters,
		knoba:    model.knoba,
		knobd:    model.knobd,
		bus:      model.bus,
	}
	if !mode.usesLed(pidp11.LED_RUN) {
		mode.meters["system.running"] = newLumenMeter(0,
			[]pidp11.LedID{pidp11.LED_RUN}, "", 0, 0)
	}

	argv2 := make([]string, len(argv))
	copy(argv2, argv)
	argv = argv2

	started := false
	autostarted := false
	events := make(chan string, 1000) // messages from/to the script
	eventsQuit := make(chan bool)

	mode.start = func() {
		if started {
			return
		}

		var params []string
		if len(argv) > 1 {
			params = argv[1:]
		} else {
			params = make([]string, 0)
		}
		c := exec.Command(argv[0], params...)
		stderr, err := c.StderrPipe()
		if err != nil {
			slog.Warn("control StderrPipe", "err", err)
			return
		}
		stdin, err := c.StdinPipe()
		if err != nil {
			slog.Warn("control StdinPipe", "err", err)
			return
		}
		stdout, err := c.StdoutPipe()
		if err != nil {
			slog.Warn("control StdoutPipe", "err", err)
			return
		}
		err = c.Start()
		if err != nil {
			slog.Warn("control Start", "err", err)
			return
		}
		started = true

		// Send messages to the command's stdin
		go func() {
			period := time.Duration(tickerPeriodMs) * time.Millisecond
			clock := time.NewTicker(period)
			for {
				msg := ""
				select {
				case <-eventsQuit:
					return
				case t := <-clock.C:
					if mode.isRunning() {
						msg = fmt.Sprintf("tick %d", t.UnixMilli())
					}
				case eventStr := <-events:
					switch eventStr {
					case "stop":
						mode.setRunning(false)
					case "start-pressed":
						msg = ""
						for _, addr := range mode.usedAddresses() {
							data, _ := mode.read(addr)
							msg += fmt.Sprintf("memory %d %d\n", addr, data)
						}
						if mode.isRunning() {
							mode.setRunning(false)
							msg += fmt.Sprintf("stop %d", time.Now().UnixMilli())
						} else {
							clock.Reset(period)
							mode.setRunning(true)
							msg += fmt.Sprintf("start %d", time.Now().UnixMilli())
						}
					default:
						msg = eventStr
					}
				}
				if msg != "" {
					_, err := stdin.Write([]byte(msg + "\n"))
					if err != nil {
						slog.Error("Write error, aborting", "err", err)
						return
					}
				}
			}
		}()

		// Kill the process when the control file is modified,
		// to make it easier to test changes.
		// xxx requires leaving/returning to mode after kill
		go func() {
			// Handle command lines like [awk, ...] by trying to identify the
			// script. Only works if the script has a shebang.
			isFileWithShebang := func(p string) bool {
				f, err := os.Open(p)
				if err != nil {
					return false
				}
				var buf [2]byte
				_, err = f.Read(buf[:])
				if err != nil {
					return false
				}
				return buf[0] == '#' && buf[1] == '!'
			}
			path := argv[0]
			for _, arg := range argv {
				if isFileWithShebang(arg) {
					path = arg
					break
				}
			}
			ticker := time.NewTicker(time.Second)
			var mtime time.Time
			for range ticker.C {
				info, err := os.Stat(path)
				if err != nil {
					slog.Error("control command not found", "err", err)
					continue
				}
				newMtime := info.ModTime()
				if !mtime.IsZero() && newMtime != mtime {
					slog.Info("control command modified, killing it", "path", path)
					c.Process.Kill()
					return
				}
				mtime = newMtime
			}
		}()

		// Log command's stderr
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				slog.Info(fmt.Sprintf("%s stderr: %s", mode.name, line))
			}
			if err := scanner.Err(); err != nil {
				slog.Error("control stderr", "err", err, "mode", mode.name)
			}
		}()

		// Ingest control messages from the command's stdout, and intercept
		// our own messages.
		intercept := func(msg Message) Message {
			if ctrl, ok := msg.(Control); ok {
				// check for controller-specific messages
				if ctrl.msg == "stop" {
					events <- ctrl.msg
					return nil
				}
			}
			return msg
		}
		readMessagesDone := readMessages(mode.name, stdout, mode.bus, intercept)

		err = c.Wait()
		if err != nil {
			slog.Warn("control Wait", "err", err)
			mode.setRunning(false)
			started = false
			autostarted = false
			<-readMessagesDone
			eventsQuit <- true
		}
	}

	mode.handler = func(evt pidp11.Event) (pidp11.Event, bool) {
		Debug(logging.LOG_CONTROLLER, "control handler", "event", evt)
		if !started && evt.ID != pidp11.SS_NIL {
			Debug(logging.LOG_CONTROLLER, "control handler control exited")
			return pidp11.Event{}, false
		}
		msg := ""
		switch {
		case evt.ID == pidp11.SS_NIL: // init message
			if autostart && !autostarted {
				autostarted = true
				msg = "start-pressed"
			}
		case evt.ID == pidp11.SS_START:
			if autostart {
				slog.Info("you cannot stop an autostarted control")
			} else {
				msg = "start-pressed"
			}
		// Push other events, but not those used for mode selection
		case evt.ID == pidp11.SS_LOAD ||
			evt.ID == pidp11.SS_EXAM ||
			evt.ID == pidp11.SS_DEP ||
			evt.ID == pidp11.SS_CONT:
			msg = fmt.Sprintf("event %s %v", evt.SwitchName(), evt.On)
		case evt.ID >= pidp11.SS_SR0 && evt.ID <= pidp11.SS_SR21:
			// Rename the register switch events to just their number, eg SR_3 -> 3
			// as these are easier to use from scripts.
			regSwitchNum := evt.ID - pidp11.SS_SR0
			msg = fmt.Sprintf("event %d %v", regSwitchNum, evt.On)
		default:
			return evt, false
		}
		if msg != "" {
			go func() { //xxxx
				events <- msg
			}()
		}
		return pidp11.Event{}, false
	}
	return &mode
}
