package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Globals
type Specials struct {
	inLevels, inDataEntry       bool
	justExamined, justDeposited bool
	entryAddr, entryData        uint
}

var specials Specials

var nonEvent = Event{ID: SS_NIL}

// Main event loop for the program, handles Console events.
// It may trigger a mode switch, and delegate event handling
// to the new mode's event handler, if present.
func (mgr *Manager) eventLoop() {
	// The mode event handlers get passed an event and return:
	// - the same event generally
	// - a boolean: false if we should stay in the mode, true if we should
	//   go back to the previous mode.
	var modeHandler func(evt Event, mode *Mode) (Event, bool)

	modeHandlers := make(map[string]func(Event, *Mode) (Event, bool), 0)
	modeHandlers["_entry"] = mgr.entryHandler
	modeHandlers["_levels"] = mgr.levelsHandler
	modeHandlers["_test"] = mgr.testHandler

	// We track knob positions by counting rotation events
	knoba := 0 // position of address knob, range [0, 7]
	knobd := 0 // position of data knob, range [0, 3]
	// To remember knobd as we change knoba
	knobaToKnobd := make(map[int]int, 0)
	knobaToKnobd[knoba] = knobd

	prevMode := mgr.ModeForKnobs(knoba, knobd)
	mgr.SelectMode(prevMode)

	// Switch to named mode and use its event handler
	to := func(name string) {
		prev := mgr.mode
		if !strings.HasPrefix(prev.name, "_") {
			prevMode = prev
		}
		newMode := mgr.modesByName[name]
		mgr.SelectMode(newMode)
		h, ok := modeHandlers[name]
		if ok {
			modeHandler = h
			modeHandler(nonEvent, prevMode) // serves as mode init
		} else {
			modeHandler = nil
		}
	}

	// Back to previous mode, but give priority to levels or data entry
	back := func() {
		mode := prevMode
		switch {
		case specials.inLevels:
			mode = mgr.modesByName["_levels"]
		case specials.inDataEntry:
			mode = mgr.modesByName["_entry"]
		}
		prevMode = mode
		mgr.SelectMode(mode)
		h, ok := modeHandlers[mode.name]
		if ok {
			modeHandler = h
		} else {
			modeHandler = nil
		}
	}

	for evt := range mgr.cons.Events {
		Debug(LOG_HANDLER, "event %v\n", evt)
		if modeHandler != nil {
			var exit bool
			evt, exit = modeHandler(evt, prevMode)
			if exit {
				back()
				continue
			}
		}
		switch evt.ID {
		case SS_TEST_ON:
			to("_test")
		case SS_HALT:
			specials.inDataEntry = true
			if !specials.inLevels {
				specials.entryAddr = 0
				specials.entryData = 0
				to("_entry")
			}
		case SS_S_BUS_CYCLE:
			specials.inLevels = true
			to("_levels")
		case SS_START:
			mgr.commandCenter.StartStop(mgr)
		case SS_KNOBA_PUSH:
			mgr.cons.ShouldSpeak(true)
			mgr.cons.Speak("power off")
			time.Sleep(time.Second)
			err := exec.Command("sudo", "poweroff").Run()
			if err != nil {
				Err("poweroff failed: %v", err)
			}
		case SS_KNOBD_PUSH:
			mgr.cons.ShouldSpeak(true)
			mgr.cons.Speak("blink11 exit")
			time.Sleep(2 * time.Second)
			mgr.mainDone <- true
		case SS_KNOBA, SS_KNOBD:
			// Change to the mode specified by the knobs, if any.
			var newMode *Mode
			ka := knoba
			kd := knobd
			switch evt.ID {
			case SS_KNOBA:
				ka = knoba + evt.Val
				if ka >= 0 && ka <= 7 {
					kd = knobaToKnobd[ka]
					newMode = mgr.ModeForKnobs(ka, kd)
				}
			case SS_KNOBD:
				kd = knobd + evt.Val
				if kd >= 0 && kd <= 3 {
					newMode = mgr.ModeForKnobs(ka, kd)
				}
			}
			if newMode == nil || newMode == mgr.mode {
				mgr.cons.KnobSound(false)
			} else {
				Debug(LOG_HANDLER, "new mode ka: %d  kd: %d → %s\n",
					ka, kd, newMode.name)
				mgr.cons.KnobSound(true)
				knoba = ka
				knobd = kd
				knobaToKnobd[ka] = kd
				mgr.SelectMode(newMode)
			}
		}
	}
}

// Event handler for the levels mode, for adjusting volume/brightness
// using the knobs.
func (mgr *Manager) levelsHandler(evt Event, _ *Mode) (Event, bool) {
	Debug(LOG_HANDLER, "levelsHandler %v\n", evt)

	// Volume between 0% and 100%
	vol := int(mgr.cons.GetVolume() * 100)
	vol7 := scaleRound(vol, 100, 0, 7)

	// Brightness between 25% and 100%
	bright := mgr.cons.GetBrightnessScaling()
	if bright < .25 {
		bright = .25
		mgr.cons.SetBrightnessScaling(bright)
	}
	bright3 := scaleRound(int(bright*100)-25, 75, 0, 3)

	emit := func() {
		mgr.Emit("volume", vol7, 7)
		mgr.Emit("brightness_range", 10, 10)
		speech := 0
		if mgr.cons.speak {
			speech = 1
		}
		mgr.Emit("speech_enabled", speech, 10)
		mgr.EmitKnobMetrics(vol7, bright3)
	}

	switch evt.ID {
	case SS_S_INST: // get out
		specials.inLevels = false
		return nonEvent, true
	case SS_KNOBA:
		vol7 += evt.Val
		if vol7 < 0 || vol7 > 7 {
			mgr.cons.KnobSound(false)
		} else {
			mgr.cons.KnobSound(true)
			mgr.cons.SetVolume(float64(scaleRound(vol7, 7, 0, 100)) / 100.0)
			emit()
		}
	case SS_KNOBD:
		bright3 += evt.Val
		if bright3 < 0 || bright3 > 3 {
			mgr.cons.KnobSound(false)
		} else {
			mgr.cons.KnobSound(true)
			mgr.cons.SetBrightnessScaling(.25 + scaleFloat(bright3, 3, 0, .75))
			emit()
		}
	case SS_KNOBA_PUSH:
		if mgr.cons.speak {
			mgr.cons.ShouldSpeak(false)
		} else {
			mgr.cons.ShouldSpeak(true)
			mgr.cons.Speak("speech enabled")
		}
		emit()
	default:
		emit()
	}
	return nonEvent, false
}

// Event handler for TEST mode
func (mgr *Manager) testHandler(evt Event, _ *Mode) (Event, bool) {
	Debug(LOG_HANDLER, "testHandler %v\n", evt)
	switch evt.ID {
	case SS_TEST_OFF:
		return nonEvent, true
	}
	return nonEvent, false
}

// Event handler for the HALT mode, aka data entry.
func (mgr *Manager) entryHandler(evt Event, mode *Mode) (Event, bool) {
	Debug(LOG_HANDLER, "entryHandler %v\n", evt)
	showAddr := func() {
		mgr.Emit("entry.address", int(specials.entryAddr), 1<<21)
	}
	showData := func() {
		specials.entryData = mode.Read(specials.entryAddr)
		mgr.Emit("entry.data", int(specials.entryData), 1<<16)
	}

	if evt == nonEvent { //init
		specials.entryAddr = 0
		showData()
		return nonEvent, true
	}

	cons := mgr.cons
	switch evt.ID {
	case SS_LOAD:
		specials.entryAddr = cons.ReadRegSwitches()
		showAddr()
	case SS_EXAM:
		if specials.justExamined {
			// Repeated EXAM shows content of next address
			specials.entryAddr++
			showAddr()
		}
		showData()
	case SS_DEP:
		// xxx how does repeat work on the real thing?
		data := cons.ReadRegSwitches() &^ (0xFF << 16)
		Debug(LOG_HANDLER, "dataEntryEventHandler data=%v\n", data)
		mode.Write(specials.entryAddr, data)
		showData()
	case SS_KNOBA_PUSH:
		mgr.cons.Speak(fmt.Sprintf("%d", specials.entryAddr))
	case SS_KNOBD_PUSH:
		mgr.cons.Speak(fmt.Sprintf("%d", specials.entryData))
	case SS_ENABLE:
		specials.inDataEntry = false
		return nonEvent, true
	}
	specials.justExamined = evt.ID == SS_EXAM
	specials.justDeposited = evt.ID == SS_DEP
	return nonEvent, false
}
