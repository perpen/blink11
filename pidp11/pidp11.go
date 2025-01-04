package pidp11

import (
	"fmt"
	"sync"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

type LedID int
type SwitchID int

type Event struct {
	ID    SwitchID
	State bool
}

var loopμs int // approx. duration of a loop, for converting durations to loops
var debug func(format string, args ...interface{})

type Pidp struct {
	running                  bool
	ledSpecs                 [72]ledSpec
	switches                 [38]bool // current state of switches
	Events                   chan Event
	negateKnoba, negateKnobd bool
}

// Specifies the current brightness of the led, and its envelope
type ledSpec struct {
	sync.Mutex
	bright int // brightness, 0-31
	env    envelope
	name   string // for debug messages
}

func NewPidp(negateKnoba, negateKnobd bool,
	debugLogger func(format string, args ...interface{})) *Pidp {

	debug = debugLogger
	pidp := Pidp{
		Events:      make(chan Event, 50),
		negateKnoba: negateKnoba,
		negateKnobd: negateKnobd,
	}
	for id := LedID(0); id < LEDS_COUNT; id++ {
		pidp.ledSpecs[id].name = LedName(id)
	}
	return &pidp
}

func (pidp *Pidp) Start() error {
	if err := rpio.Open(); err != nil {
		return err
	}
	pidp.running = true

	// Time the loop
	timingChan := make(chan int, 1)
	timingLoops := 3000
	go pidp.loop(timingChan, timingLoops)
	loopμs = <-timingChan / timingLoops
	close(timingChan)
	fmt.Printf("Pidp loop duration: %vμs\n", loopμs)

	return nil
}

func (pidp *Pidp) Stop() error {
	pidp.running = false
	time.Sleep(20 * time.Millisecond) // time for the loop to notice
	pidp.ClearLeds()
	return rpio.Close()
}

func (pidp *Pidp) ClearLeds() {
	fx := NewSimpleEffect(0, 0)
	for id := LedID(0); id < LEDS_COUNT; id++ {
		pidp.SetLed(LedID(id), fx, 0)
	}
}

func (pidp *Pidp) SetLed(id LedID, fx Effect, brightP float64, params ...float64) {
	spec := &pidp.ledSpecs[id]
	debug("SetLed(%s, %v, %f, %v)", spec.name, fx, brightP, params)
	spec.Lock()
	defer spec.Unlock()
	progress := spec.getProgress()
	spec.makeEnvelope(fx, brightP, params...)
	spec.setProgress(progress)
}

func (spec *ledSpec) isOn(counter int) bool {
	spec.step()
	return BRIGHTNESS_PHASES[spec.bright][counter%(BRIGHTNESS_STEPS-1)]
}

func (pidp *Pidp) loop(timingChan chan int, timingLoops int) {
	// All pins as inputs, pull-ups on columns, pull-offs on rows
	for _, ledrow := range LED_ROWS {
		pin := rpio.Pin(ledrow)
		pin.Input()
		pin.Low()
	}
	for _, col := range COLS {
		rpio.Pin(col).Input()
	}
	for _, row := range ROWS {
		rpio.Pin(row).Input()
	}
	for _, col := range COLS {
		rpio.Pin(col).PullUp()
	}
	for _, ledrow := range LED_ROWS {
		rpio.Pin(ledrow).PullOff()
	}
	for _, row := range ROWS {
		rpio.Pin(row).PullOff()
	}

	// Main loop, exits when .running is false
	counter := 1
	start := time.Now()
	for {
		if counter == timingLoops {
			μs := int(time.Now().Sub(start).Microseconds())
			timingChan <- μs
		}

		// LEDs
		for _, col := range COLS {
			rpio.Pin(col).Output()
		}
		for ledrownum, ledrow := range LED_ROWS {
			for colnum, col := range COLS {
				led := ledrownum*len(COLS) + colnum
				if pidp.ledSpecs[led].isOn(counter) {
					rpio.Pin(col).Low()
				} else {
					rpio.Pin(col).High()
				}
			}
			rpio.Pin(ledrow).High()
			rpio.Pin(ledrow).Output()
			nanosleep(5e4) // led is on
			rpio.Pin(ledrow).Low()
			nanosleep(ANTI_GHOSTING_PAUSE_NS)
		}

		// Switches
		for _, col := range COLS {
			rpio.Pin(col).Input()
		}
		for rownum, row := range ROWS {
			rpio.Pin(row).Output()
			rpio.Pin(row).Low()
			nanosleep(500)
			for colnum, col := range COLS {
				reading := rpio.Pin(col).Read()
				sw := rownum*len(COLS) + colnum
				oldState := pidp.switches[sw]
				newState := reading != rpio.Low
				pidp.switches[sw] = newState
				if newState != oldState {
					ev := Event{
						ID:    SwitchID(sw),
						State: newState,
					}
					ev = filterKnobs(
						ev, pidp.negateKnoba, pidp.negateKnobd)
					if ev.ID != SW_NONE {
						pidp.Events <- ev
					}
				}
			}
			rpio.Pin(row).Input()
		}

		if !pidp.running {
			break
		}
		counter += 1
	}
}

func assert(b bool, format string, args ...interface{}) {
	if !b {
		panic(fmt.Sprintf("assertion failed: "+format, args...))
	}
}
