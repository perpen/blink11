This doc is wip

# TODO
- compute pulse latency by testing a few and checking for underflow
- re-implement timer in a script
- auto-login for sound: https://linuxconfig.org/how-to-set-user-autologin-on-raspberry-pi

# Blink11

A controller for the PiDP-11, meant for home automation tasks.

https://obsolescence.wixsite.com/obsolescence/pidp-11

Features:
- Hackable
- Selection of different modes using the knobs
- Light effects: flashing, strobing, attack/release
- Integration with the world using:
  - Internal or external commands
  - TCP endpoint for pushing metrics
- Multi-streams sound effects, including speech

## Terminology

- Metric: has a name, a value, a max value, an error message. Numbers are positive.
- Feed: emits metrics
- Meter: a method for representing a metric using a set of lights (eg
  lights bar, dot, flashing, strobing, binary, luminosity)
- Mode: defines the mapping of Meters to lights. It is
  selected using the rotary knobs.

## Modes

Each mode has:
- A unique name
- Feeds, which will be started when switching to the mode (eg time or
  network feeds)
- A list of other modes to inherit meters from. Inherited meters
  may be overriden by re-defining them in the mode.

Modes are usually selected using the rotary knobs, but some switches
activate special modes:
- TEST: Test mode, switching on all lights and exercising some effects.
- S. BUS CYCLE: Levels mode, to adjust audio volume and led brightness
  using the address/data knobs. Pressing the address knob enables/disables
  speaking the mode name when selected.
- HALT: Data entry mode, for editing the mode's memory space
  using the LOAD/EXAM/DEP switches.

## Agents

Blink11 provides a TCP endpoint to which metrics can be pushed
remotely.

For an example see `./agent11`, which is a linux program which submits
performance metrics such as cpu usage or load.
```
agent11 -hostname myhost -server pi:3333 -cpu -net
```

Agent11 has an `-stdin` option, which can be used to send arbitrary
metrics from another program, with format `METRIC VALUE MAX [ERROR]`
```
for n in $(seq 0 10); do
	echo "%s.counter $1 10"
done | agent11 -hostname myhost -server pi:3333 -stdin
```
If the string "%s" is present in the metric name, agent11 will replace it
with the string provided by the `-hostname` option.

xxx provide example script.

## Commands

A mode can have an associated command, specified as an argv-style list,
which has access to the mode's memory space.
```
sections:
- section: demo
  modes:
   - name: timer demo
     command: [timer]
   - name: external command demo
     command: [exec, /my/script.sh, param1, param2]
```

The command for the current mode is started or stopped by pressing the START switch.
The RUN light is bright if the command for the current mode is running,
dimmed if another mode's command is running,
off if no command is running in any mode.

The only implemented commands are `timer` and `exec`.

Command `timer` does a countdown from the number of seconds stored in memory at address 0.

With `exec`, the command executed can emit metrics by writing to stdout lines with format `metric: METRIC VALUE MAX [ERROR]`, eg:
```
metric: temperature 35 70
metric: temperature 0 0 oops
```
Sounds can be played with `sound: FILENAME`:
```
sound: alarm.raw
sound: tts:hello
```
Lines not matching these formats are ignored.

Exec should be sufficient for most tasks. It is only necessary to implement a built-in command if for example it requires access to the blink11 state while it is running.

### Command parameters

The command run with `exec` is passed the content of the mode memory space using environment variables: if address 2 contains value 3, variable `BLINK11_ADDR_2=3` will be passed.

### Data entry

Each mode has its own memory space, which can be edited by
pressing the HALT switch and using the register switches as would be done on a PDP-11:
- View memory address content:
  - Configure address using the register switches
  - Press LOAD ADRS
  - Press EXAM - to view the following addresses press again
- Write to address:
  - Enter address using the register switches
  - Press LOAD ADRS
  - Enter data using the register switches
  - Press DEP - xxx repeat??

## Sounds

xxx

## Provided

examples:
- alarm time
- timer duration, optionally looping
- volume
- brightness
- mode

# Resources/thanks

- Neil Higgins' excellent PiDP-11 reference information pdf: https://groups.google.com/g/pidp-11/c/v2ncPq_5Qxk/m/GVsrb5DnAwAJ
- David Jones' alternative to the original GPIO C code, a nice read: https://groups.google.com/g/pidp-11/c/LzHi8gI2S5E/m/GR1SLaatAgAJ
- Blink11 uses Stian Eikeland's GPIO library. My GPIO bugs magically disappeared when I moved from my dodgy C→Go translation to this: https://github.com/stianeikeland/go-rpio
