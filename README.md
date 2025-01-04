Blink11 is a controller for the
[PiDP-11](https://obsolescence.wixsite.com/obsolescence/pidp-11),
written in Go.

It can be used for home automation, systems monitoring and any other
application requiring an unreasonable amount of blink.

Features:
- Mostly configured via a config file
- Selection of different modes using the knobs
- Modes can be implemented using simple scripts
- Mode parameters can be entered in the traditional manner using the
  LOAD/EXAM/DEP switches
- TCP endpoint
- Light effects: flashing, strobing, attack/release, ...
- Multi-streams sound effects, speech

This uses the [pidp11 module](https://github.com/perpen/pidp11) for
interacting with the PiDP-11.

# Requirements

This has been tested on a Raspberry Pi 3 running debian 12.8 (32 and
64 bits).

Sound requires pipewire, but adding support for a different platform
is trivial, see interface `audio.Backend`.

# Bugs

I have no idea if the code is going to run on other hardware or linux
distributions.

[Create an issue](https://github.com/perpen/blink11/issues) if you have
problems.

# Terminology

- Metric: has a name, a (positive) value, a max value.
  It is _mostly_ used as a percentage.
- Feed: emits metrics
- Meter: a method for representing a metric using a set of leds (eg
  lights bar, single dot over a range of lights, flashing, strobing,
  binary, brightness)
- Mode: defines the mapping of Meters to lights. It is
  selected using the rotary knobs.
- Agent: external program which sends metrics to blink11 over the network.

# Config file

`blink11` takes the path to its configuration file as a parameter.

The file is in yaml format. Indent with spaces, not tabs.

See `blink11.yaml.m4` for an example. Note that this file is an m4
template and cannot be used as is. See the function `_config` in script
`hack`.

```yaml
pidp:
  # sensible brightness and frequencies settings
  brightness: # between 0 and 1
    min: .05
    max: 1
  frequencies: # for flash/strobe effects
    min_hz: .5 # if frequency is less, just keep the led off
    max_hz: 10 # frequency for metric value 1
    one_hz_at: .1 # we want 1Hz for this metric value
  speak: true
server:
  addr: :3333
audio:
  impl: pipewire # pipewire|none
  rate: 24000 # keep in sync with the rate in ./scripts/audio-convert
  latency_ms: 10 # tune for hardware
  sounds_dir: /home/pi/blink11/sounds

  tts_language: en
  tts_volume_factor: .5
  tts_speed_factor: 1.2
  cache_dir: /home/pi/blink11/tmp # where to cache tts files

  startup_sound: sensor_sweep.raw
  volume_sound: click5.raw # played when changing volume
  knob_ok_sound: click5.raw # when rotating a knob
  knob_ko_sound: click4.raw # when rotating a knob out of range
memory_path: /home/pi/blink11/memory.yaml # stores memory spaces, volume, brightness
debug:
 # - main
 # - audio
 # - console
 # - controller
 # - event
 # - feed
 # - handler
 # - loop
 # - memory
 # - meter
 # - mode
 # - net
 # - pidp


# The set of meters displayed on the panel at a given time is called a
# "mode".
# The config below describe the modes, and the structure and order
# of the sections and modes imply how to select each mode using the
# address and data knobs:
# - The address knob selects a "section"
# - The data knob selects a mode in this section.
# Thus we can have 8 sections, with a maximum of 4 modes each.
#
# sections:
#   - section: <section description, free text>
#     hidden: true|false  # if true, the modes in the section will not
#                         # be selectable via knobs. They can however be
#                         # imported by other modes.
#     modes:
#     - name: <string>
#       imports: [<mode>, ...] # modes to import meters and feeds from, default []
#       feeds: [<feed>, ...] # feeds generating the metrics needed by the mode, default []
#       control: # optional
#         argv: [<program>, <param>, ...]
#         ticker_period_ms: <ms> # default 1000
#         autostart: true|false # default false
#       meters:
#         <metric name>:
#           type: lumen|bar|dot|binary|flash|strobe
#           on_ms: <integer> # default 0
#           off_ms: <integer> # default 0
#           leds: [<led>, ...]
#           gate: <float> # reduce noise by ignoring small values, default 0
#     - name: ...
#  - section: ...
```

# Modes

Modes are defined by the config file.

Each mode has:
- A unique name
- Optional feeds, which will be started when switching to the mode and
  emit metrics
- A list of other modes to import meters from. Imported meters may be
  overriden by re-defining them in the mode.

Modes are usually selected using the rotary knobs, but some "system"
modes are activated by switches:
- TEST: Test mode, switching on all lamps and exercising some effects.
  From this mode pressing the Data knob will cause blink11 to exit, and
  pressing the Address knob will shutdown the rpi (`sudo poweroff`).
- S. BUS CYCLE: Levels mode, to adjust audio volume and led brightness
  using the address/data knobs. Pressing the address knob enables/disables
  speaking the mode name when selected.
- HALT: Data entry mode, for editing the mode's memory space
  using the LOAD/EXAM/DEP switches. See section Data Entry below.

# Feeds

Feeds are implemented with Go code:
- `time` emits time-related metrics like `time.hours`, `time.seconds`, etc
- `network` emits the metrics received from agents over the network.
- `idle` emits metrics used for displaying a few idle patterns.
- `animation` is a hack: it does not emit metrics, but instead
  controls the leds directly.

Metrics may also be emitted by a mode's control.

# Meters

A meter associate a metric with a set of leds.

A same metric can be associated with multiple meters, on different modes.

It can be of different types:
- lumen: metric value translates to led brightness
- bar: a la progress bar, eg with 6 leds, percentage value .5 will give
  'xxx...'
- dot: cursor-style, only 1 led is on. Eg with 3 leds, percentage value .5
  will give '.x.'
- binary: will represent the integer value as binary, eg with 3 leds,
  value 6 will be 'xx.'
  It is only when using a binary meter that the metric will be handled as
  an absolute value and not as a percentage.
- flash or strobe: the higher the value, the higher the flashing/strobing
  frequency.
  Tuned via config `pidp.frequencies`. With `flash`, the leds stay on
  and off for the same duration.  With `strobe`, the leds stay on for
  a fixed short time only.

The speed at which the leds switch on or off can be tuned by the meter
properties `on_ms` and `off_ms`. Eg with `on_ms: 500` the leds will
reach full brightness after half a second.

By default the bar and dot meters round the value before assigning leds.
This behaviour can be changed using boolean attribute `floor`.

## Auto-scaling

If the max value for a metric is negative, it will be ignored and a max
value will be calculated based on the history of the values. This is
useful for example for i/o stats, for which it is not practical to use
an arbitrary maximum value. This calculation can be tuned by changing
constants in `autoscale.go`.

## System meters

Some meters are created by default for all modes:
- The MASTER led exposes metric `system.messages`, and indicates the
  number of messages received. It is auto-scaled.
- The RUN led is added to control modes, is on when in running state.

## Effects

Effects define the evolution of led brightness over time. Some are
periodic, like flashing or strobing. Brightness can ramp up/down when
the led is switched on/off.

See [pidp11](https://pkg.go.dev/github.com/perpen/pidp11) for details.

# Controls

A mode can have an associated control, which is a program specified
using an argv-style list:
```
sections:
- section: demo
  modes:
   - name: control demo
     control:
        argv: [/my/script.sh, param1, param2]
        ticker_period_ms: 1000
        autostart: true
```

The process is started when first switching to a mode, and is supposed
to run continuously. Thus when we talk below about the control being
started or stopped, it does not refer to the process being running or not.

If the process exits, presumably because of a serious condition, it can
be respawned by switching to another mode then back (not optimal). To
make it easier to work on the program, the process will be killed if
its file is modified (that is, the first element of argv which points
to a file with a shebang).

The control for the current mode is started or stopped by pressing the
START switch. The RUN led is on if the command for the current mode
is running.

When the control has been STARTed, it receives `tick` messages at
intervals defined by the `ticker_period_ms` parameter.

See examples under `script/`, written in a few different languages.

## Outgoing messages

The messages written to the program's stdin are:
- `start` or `stop` -
  Sent when START is pressed. Blink11 tracks the started/stopped state
  and sends the suitable message.  If the control config has `autostart`
  on, the `start` message is sent the first time the mode is selected,
  without having to press START.
- `tick EPOCH_MS` -
  If the control is started, this message will be sent every `ticker_period_ms`.
- `memory ADDR DATA` -
  The `start` message is always preceded by `memory` messages providing
  the current memory space to the control.
- `event SWITCH on|off` -
  These are sent when switches/buttons are actioned. Some switches are not
  available to controls, in particular the ones used for mode navigation.

## Incoming messages

Messages are read from the control program's stdout.
 For example a shell script may do `echo metric temperature 20 80`. This
value will be reflected by meters for this `temperature` metric.

Valid formats are:
- `metric METRIC VALUE MAX`
- `error METRIC MSG ...`
- `sound FILENAME`
- `control stop`

```
metric battery 35 100
metric eth0.packets 3185 -1
error battery Something went wrong
sound alarm.raw
sound tts: this will be spoken
```

The text at the end of the error message will simply be logged by blink11.

## Data entry

Each mode has its own memory space, which can be edited by pressing the
HALT switch and using the register switches as would be done on a PDP-11:
- View memory address content:
  - Configure address using the register switches
  - Press LOAD ADRS
  - Press EXAM - to view the following addresses press again
- Write to address:
  - Enter address using the register switches
  - Press LOAD ADRS
  - Enter data using the register switches
  - Press DEP

Pressing the address or data knob will voice the corresponding decimal
number.

# Agents

Blink11 provides a TCP endpoint which can receive metrics in the format
descripted in the "Incoming messages" section above.

See example `./agent11`, which is a linux program emitting every 100ms
performance metrics related to cpu usage, loadavg, network load, disk ops:
```
agent11 -hostname myhost -server pidp11:3333 -cpu -net -disk
```

If the string "%h" is present in the metric name, agent11 will replace
it with the string provided by the `-hostname` option before sending it
to blink11.

Agent11 has an `-stdin` option, which can be used to send messages from
another program, with formats listed in the Messages section below.

```
$ for i in $(seq 10); do echo metric %h.n $i 10; done | ./bin/agent11-amd64 -hostname myhost -server pidp11:3333 -stdin
... will send lines like "metric myhost.n 1 10" etc ...
```

The similar option `-emitter PROGRAM` can be used multiple times:
```
$ ./bin/agent11-amd64 -hostname myhost -server pidp11:3333 -emitter some-program -emitter 'another-program param1 param2'
```

Options `-stdin`, `-emitter`, `-cpu` etc can be used simultaneously.

# Sounds

I use a small USB speaker which fits inside the PiDP-11 case.

Volume can be adjusted using the address knob in "levels" mode.

Set config `audio.impl`to `none` to disable all audio.

Blink11 implements a simple audio mixer which can play multiple streams
in parallel.

With pipewire the simplest way to get sound to work when running blink11
as a service is to enable auto-login:
- Run `sudo raspi-config`
- Navigate to "System Options", "Boot / Auto Login", "Console autologin"

Use `scripts/audio-convert` to convert your wav/mp3/etc files to
the required raw format.  This script can as well adjust volume and
speed. Decreasing the volume may be needed to prevent distortion when
playing several loud sounds at the same time.

## Sound names, speech

Sounds are referenced by file name.

If the name starts with `tts:` the rest of the text will be spoken
(this uses the Google Translate api to generate the speech, there will
be a small lag when speaking a text for the first time).

For intelligibility, speech outputs are not mixed: starting a speech
will interrupt the previous one.

Sound files must be in s16/mono format, immediately playable without
conversion.

Refer to:
- function `sounds` in `hack`
- script `scripts/audio-convert`

# Hacking and installing

I have been developing from my linux laptop, and rsyncing files to the
rpi to test.

For this I use the script `hack`, which I suggest you read if you intend
to use blink11. No doubt the script will fail to work in environments
other than mine. If you intend on using blink11, you will probably need
a version of this script, tailored to your setup.

Useful functions:
- `hack run` to compile the code locally then run the programs on the pi.
  For speed it copies the files to a tmpfs.
- `hack services` to install on the pi systemd services for Blink11 and Agent11.
- `hack sounds` to convert audio files stored under `sounds/`

If your pi is 32 bits, change variable `arm_flags` in `hack`.

# Resources/thanks

- Neil Higgins' [PiDP-11 reference information pdf](https://groups.google.com/g/pidp-11/c/v2ncPq_5Qxk/m/GVsrb5DnAwAJ)
- Blink11 uses [Stian Eikeland's GPIO library](https://github.com/stianeikeland/go-rpio). My GPIO bugs magically disappeared when I moved from my dodgy C→Go translation to this.
- [The original pdp11/70 processor handbook](https://bitsavers.org/pdf/dec/pdp11/1170/PDP-11_70_Handbook_1977-78.pdf)
