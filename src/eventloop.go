package main

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/console"
	"github.com/perpen/blink11/logging"
	"github.com/perpen/pidp11"
)

type Unibus chan<- Message

type Message interface{}

type Control struct {
	msg string
}

func (ctrl Control) String() string {
	return fmt.Sprintf("Control[msg:%s]", ctrl.msg)
}

type Sound struct {
	name string
}

func (snd Sound) String() string {
	return fmt.Sprintf("Sound[name:%s]", snd.name)
}

func eventLoop(cfg config.Config) {
	bus := make(chan Message, 50)
	feedMgr := newFeedManager(cfg.Server, bus)
	modesByName := makeModes(cfg.Sections, bus)
	store := newMetricsStore()

	// We track knob positions by counting rotation events
	knoba := 0 // position of address knob, range [0, 7]
	knobd := 0 // position of data knob, range [0, 3]
	// To remember knobd as we change knoba
	knobaToKnobd := make(map[int]int, 0)
	knobaToKnobd[knoba] = knobd

	testMode := newTestMode(bus)
	levelsMode := newLevelsMode(bus)
	entryMode := newEntryMode(bus)

	var curMode, prevMode *Mode

	updateRUNLed := func() {
		val := 0
		if curMode.running {
			val = 1
		}
		bus <- Metric{
			name: "system.running",
			val:  val,
			max:  1,
		}
	}

	// Update the leds using the meter corresponding to the metric, if any
	visualise := func(met Metric) {
		// Update the meter if currently showing
		meter, ok := curMode.meters[met.name]
		if ok {
			meter.visualise(met)
		}
	}

	selectMode := func(newMode *Mode) {
		oldMode := curMode
		Debug(logging.LOG_LOOP, "select mode", "mode", newMode.name)
		curMode = newMode

		// Update meters with last know values
		console.ClearLeds()
		console.Mute(true) // do not play meters sounds
		for metricName := range newMode.meters {
			metric := store.retrieve(metricName)
			visualise(metric)
		}
		console.Mute(false)
		console.SpeakMode(strings.ReplaceAll(newMode.speech, "_", " "))

		// Update knob leds
		if !newMode.isSystem {
			bus <- Metric{name: "system.knoba", val: newMode.knoba + 1, max: 8}
			bus <- Metric{name: "system.knobd", val: newMode.knobd + 1, max: 4}
		}

		go newMode.start()

		// Stop/start feeds for old/new mode
		oldFeeds := make([]string, 0)
		if oldMode != nil {
			oldFeeds = oldMode.feeds
		}
		for _, feed := range oldFeeds {
			if !slices.Contains(newMode.feeds, feed) {
				feedMgr.control(feed, false)
			}
		}
		for _, feed := range newMode.feeds {
			if !slices.Contains(oldFeeds, feed) {
				feedMgr.control(feed, true)
			}
		}

		if newMode.handler != nil {
			// Sending a zero-event to the mode as init msg
			newMode.handler(pidp11.Event{})
		}
		updateRUNLed()
	}

	// Move to new mode and remember previous
	to := func(newMode *Mode) {
		Debug(logging.LOG_LOOP, "to", "mode", newMode.name)
		prev := curMode
		if prev != nil && !prev.isSystem { // xxx needed?
			prevMode = prev
		}
		selectMode(newMode)
	}

	// Back to previous mode, w/o changing navigation history
	back := func() {
		selectMode(prevMode)
	}

	modeForKnobs := func(knoba, knobd int) *Mode {
		for _, mode := range modesByName {
			if !mode.isSystem && mode.knoba == knoba && mode.knobd == knobd {
				return mode
			}
		}
		return nil
	}

	// Messages
	go func() {
		autoscaler := newAutoScaler()
		counter := newMessageCounter(bus)
		for msg := range bus {
			Debug(logging.LOG_LOOP, "eventLoop", "msg", msg)
			counter.account()
			if met, ok := msg.(Metric); ok {
				// Debug(logging.LOG_LOOP, "eventLoop", "metric", met)
				met = autoscaler.adjust(met)
				visualise(met)
				store.store(met)
			} else if ctrl, ok := msg.(Control); ok {
				Debug(logging.LOG_LOOP, "eventLoop", "control", ctrl)
				if ctrl.msg == "updateRUNLed" {
					updateRUNLed()
				}
			} else if snd, ok := msg.(Sound); ok {
				Debug(logging.LOG_LOOP, "eventLoop", "sound", snd)
				go console.LoadAndPlaySound(snd.name)
			} else {
				slog.Error("event loop got invalid message", "msg", msg)
			}
		}
	}()

	// Init
	slog.Info("Press Data knob to exit")
	initialMode := modeForKnobs(knoba, knobd)
	to(initialMode)

	// Events
	for curMode == nil {
		time.Sleep(10 * time.Millisecond)
	}
	for {
		evt := <-console.Events
		Debug(logging.LOG_LOOP, "eventLoop", "event", evt)

		// Filter event with current mode's handler
		if curMode.handler != nil && (curMode.isSystem || !isSystemEvent(evt)) {
			var exit bool
			evt, exit = curMode.handler(evt)
			if exit {
				back()
				continue
			}
		}

		switch evt.ID {
		case pidp11.SS_TEST: // test mode
			if evt.On {
				to(testMode)
			}
		case pidp11.SS_HALT: // entry mode
			if curMode != levelsMode {
				entryMode.modeParam = curMode
				to(entryMode)
			}
		case pidp11.SS_S_BUS_CYCLE: // levels mode
			if curMode != entryMode {
				to(levelsMode)
			}
		case pidp11.SS_KNOBA, pidp11.SS_KNOBD: // change mode
			var newMode *Mode
			ka := knoba
			kd := knobd
			boolAsInt := -1
			if evt.On {
				boolAsInt = 1
			}
			switch evt.ID {
			case pidp11.SS_KNOBA:
				ka = knoba + boolAsInt
				if ka >= 0 && ka <= 7 {
					kd = knobaToKnobd[ka]
					newMode = modeForKnobs(ka, kd)
				}
			case pidp11.SS_KNOBD:
				kd = knobd + boolAsInt
				if kd >= 0 && kd <= 3 {
					newMode = modeForKnobs(ka, kd)
				}
			}
			if newMode == nil || newMode == curMode {
				console.KnobSound(false)
			} else {
				console.KnobSound(true)
				knoba = ka
				knobd = kd
				knobaToKnobd[ka] = kd
				to(newMode)
			}
		}
	}
}

// Some events are reserved for the system
func isSystemEvent(evt pidp11.Event) bool {
	reserved := []pidp11.SwitchID{
		pidp11.SS_KNOBA,
		pidp11.SS_KNOBD,
		pidp11.SS_HALT,
		pidp11.SS_S_BUS_CYCLE,
	}
	return slices.Contains(reserved, evt.ID)
}
