package pidp11

// The mechanism for detecting knob rotations used in the original C version
// can be simplified to only check for a sequence of 2 non-consecutive events,
// instead of a sequence of 4 consecutive events.

type knobEvent struct {
	isCw, state bool
}

// First and last events for clockwise/anticlockwise rotation
var cwStart = knobEvent{true, false}
var cwEnd = knobEvent{false, true}
var acwStart = knobEvent{false, false}
var acwEnd = knobEvent{true, true}
var end knobEvent

// For non-knobs, simply returns the event.
// For knobs returns either:
//   - an event with ID SW_NONE, meaning the event should be ignored by the
//     caller;
//   - or a synthetic event with ID SW_KNOBA or SW_KNOBD, and a state
//     signifying the direction of the rotation.
func filterKnobs(ev Event, negateKnoba, negateKnobd bool) Event {
	knobID := SW_NONE
	var isCw bool
	switch ev.ID {
	case SW_KNOBA_CW:
		knobID = SW_KNOBA
		isCw = true
	case SW_KNOBA_ACW:
		knobID = SW_KNOBA
		isCw = false
	case SW_KNOBD_CW:
		knobID = SW_KNOBD
		isCw = true
	case SW_KNOBD_ACW:
		knobID = SW_KNOBD
		isCw = false
	}
	if knobID == SW_NONE {
		// Not a knob, just forward the event
		return ev
	}
	nonEvent := Event{ID: SW_NONE}
	bp := knobEvent{isCw, ev.State}
	switch bp {
	case cwStart:
		end = cwEnd
		return nonEvent
	case acwStart:
		end = acwEnd
		return nonEvent
	case end:
		// Emit synthetic event
		state := end == cwEnd
		if knobID == SW_KNOBA && negateKnoba {
			state = !state
		} else if negateKnobd {
			state = !state
		}
		return Event{knobID, state}
	default:
		return nonEvent
	}
}
