A Go library for the PiDP-11

## xxx
- invert the states?? resting or default position should give false.

## Light effects

A number of effects are supported:
- Simple (one-shot): optional ramping up or down of brightness when
  switching a led on or off.
- Flash (periodic): the light stays on and off for the same duration (but
  supports ramping up/down)
- Strobe (periodic): the light stays on for always the same duration, and
  off for a varying duration.
- Error (periodic): creates a hopefully recognisable brightness envelope.

### Envelopes

New effects can easily be added by making new implementations of the
`Effect` interface.

Effects are implemented using an envelope (as in audio synthesis), which
is either one-shot (eg simply switching a led on or off) or periodic (flashing etc).

An `envelope` is a sequence of `stage`s. A stage defines a linear progression
between two brightness levels, over a certain duration.

## Events

When a switch is actioned, an event is emitted on buffered channel `Pidp.Events`.
The events should be read reasonably quickly to avoid blocking the main loop.

## API

xxx provide sample command for testing?

```
pidp.Start()
defer pidp.Stop()
pidp.SetLed(LED_A0, true)
pidp.SetLedProps(LED_A1, 16, 2, 0)
pidp.SetLedProps(LED_A1, 16, 0, 2)
for ev := range pidp.Events {
	fmt.Printf("%s %v\n", pidp.SwitchName(ev.ID), ev.State)
}
```
