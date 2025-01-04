manager:
  memory: /var/tmp/blink11-mem.yaml
pidp:
  #negate_knoba: true # reverse rotation
  #negate_knobd: true
  brightness: # between 0 and 1
    min: 0.05
    max: 1
    initial_scaling: .75 # 0→min, 1→max
  frequencies:
    # For periodic effects (eg flash/strobe), the frequency is
    # linear to the metric value (which is a percentage).
    one_hz_at: .1 # value corresponding to 1Hz frequency
    min_hz: .5 # if frequency is less, just keep the led off
    max_hz: 10 # frequency obtained for input value 1
server:
  addr: :3333
audio:
  impl: pipewire
  rate: 48000
  latency_ms: 5
  volume: .5
  dir: /run/user/1000/blink11/sounds
  tmp_dir: /run/user/1000/blink11/tmp
  tts_lang: en
  knob_ok: click5.raw
  # knob_ko: click5.raw
  # startup_sound: sci-fi-high-tech-sounds-860.raw
  # startup_sound: "tts:console connected"
logging:
 - main
 # - pidp
 # - manager
 # - mode
 # - feed
 # - net
 # - handler
 # - meter
 # - cons
 # - audio
 # - event
 # - command
 # - memory


# The set of meters displayed on the panel at a given time
# is called a "mode".
# The config below describe the modes, and the structure and order
# of the section and mode entries imply how to select each mode using
# the address and data knobs:
# - The address knob selects a "section"
# - The data knob selects a mode in this section.
#
# sections:
#   - section: <section description, free text>
#     hidden: true|false  # if true, the modes in the section will not
#                         # be selectable via knobs. They can be reused..
#     modes:
#     - name: <string> # used as a description, or for reuse
#       feeds: <feeds required to generate the metrics>
#       inherits: <names of modes to inherit meters/feeds from>
#       meters:
#         <metric name>:
#           type: bar|dot|binary|flash|strobe
#           on_ms|off_ms: <integer>
#           lights: <lights>
#     - name: ...
#  - section: ...
#
sections:
  - section: reusable fragments, by convention names start with "_"
    hidden: true
    modes:
    - name: _common
      meters:
        metrics_rx:
          type: lumen
          lights: [MASTER]
        running:
          type: lumen
          lights: [RUN]

    - name: _idle_bsd
      feeds: [time]
      meters:
        idle.bsd:
          type: binary
          on_ms: 300
          off_ms: 300
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

    - name: _idle_pdp11
      feeds: [time]
      meters:
        idle.pdp11:
          type: binary
          on_ms: 0
          off_ms: 200
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

    - name: _idle_rt11
      feeds: [time]
      meters:
        idle.rt11:
          type: binary
          on_ms: 200
          off_ms: 300
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

    - name: _time_octal
      feeds: [time]
      meters:
        time.tenthseconds_cylon:
          type: dot
          off_ms: 500
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        time.seconds:
          type: binary
          lights: [A0, A1, A2, A3, A4, A5]
        time.minutes:
          type: binary
          lights: [A6, A7, A8, A9, A10, A11]
        time.hours12:
          type: binary
          lights: [A12, A13, A14, A15, A16, A17]
        time.am_pm:
          type: binary
          lights: [PAR_HI, PAR_LO]

    - name: _time_bars
      feeds: [time]
      meters:
        time.minutes:
          type: bar
          lights: [A5, A4, A3, A2, A1, A0, D5, D4, D3, D2, D1, D0]
        time.hours12:
          type: bar
          floor: true
          lights: [A14, A13, A12, A11, A10, A9, D14, D13, D12, D11, D10, D9]
        time.am_pm:
          type: binary
          lights: [PAR_HI, PAR_LO]

  - section: time, alarms, etc
    modes:
    - name: time_octal
      speech: time
      inherits: [_common, _time_octal]

    - name: time_easy
      speech: easy time
      inherits: [_common, _time_bars]

    - name: timer
      inherits: [_common]
      command: [timer]
      meters:
        timer.remaining_hours:
          type: binary
          lights: [D12, D13, D14, D15]
        timer.remaining_minutes:
          type: binary
          lights: [D6, D7, D8, D9, D10, D11]
        timer.remaining_seconds:
          type: binary
          lights: [D0, D1, D2, D3, D4, D5]
        timer.remaining_bar:
          type: bar
          off_ms: 300
          lights: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]

# usage: m4_machine_stats(HOST, OFF_MS, CPU_METER, LOAD_METER, IDLE_TYPE, SPEECH)
define(m4_machine_stats, `
    - name: $1_load
      speech: $6
      feeds: [network, idle]
      inherits: [_common, _idle_$5]
      meters:
        $1.packets:
          type: $3
          off_ms: $2
          lights: [DATA]
        $1.proc.user:
          type: $3
          off_ms: $2
          lights: [USER]
        $1.proc.system:
          type: $3
          off_ms: $2
          lights: [SUPER]
        $1.proc.iowait:
          type: $3
          off_ms: $2
          lights: [KERNEL]
        $1.proc.load1:
          type: $4
          off_ms: $2
          lights: [ADDR_16]
        $1.proc.load5:
          type: $4
          off_ms: $2
          lights: [ADDR_18]
        $1.proc.load15:
          type: $4
          off_ms: $2
          lights: [ADDR_22]
')

  - section: machines stats
    modes:
m4_machine_stats(ptoseis, 200, strobe, lumen, bsd, laptop)
m4_machine_stats(pdp11, 200, strobe, lumen, pdp11, pdp)
m4_machine_stats(dell, 200, strobe, lumen, rt11, proxmox)

  - section: home-assistant
    modes:
    - name: climate
      inherits: [_common]
      command: [exec, /run/user/1001/blink11-files/ha.sh, agent]
      meters:
        weather.temp:
          type: binary
          lights: [A0, A1, A2, A3, A4, A5]
        dyson.temp:
          type: binary
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

  - section: various experiments
    modes:
    - name: animation
      feeds: [animation]
    - name: effects
      feeds: [effects]
    - name: script
      inherits: [_common]
      command: [exec, /run/user/1001/blink11/fast.sh]
      meters:
        hack:
          type: binary
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

  - section: system, be careful changing these
    hidden: true
    modes:
    - name: _test
      feeds: [test]
      meters:
        test.flash_slow:
          type: flash
          lights: [DATA_PATHS]
        test.flash_fast:
          type: flash
          lights: [μADR_FPP_CPU]
        test.strobe_slow:
          type: strobe
          lights: [BUS_REG]
        test.strobe_fast:
          type: strobe
          lights: [DISPLAY_REGISTER]
        test.on_ms:
          type: strobe
          on_ms: 1000
          lights: [PAR_LO]
        test.off_ms:
          type: strobe
          off_ms: 1000
          lights: [PAR_HI]
        test.error:
          type: bar # whatever
          lights: [ADRS_ERR, PAR_ERR]
        test.bar:
          type: bar
          lights: [ADDR_22, ADDR_18, ADDR_16, DATA, KERNEL, SUPER, USER, MASTER, PAUSE, RUN, A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21, D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15, CONS_PHY, KERNEL_D, SUPER_D, USER_D, USER_I, SUPER_I, KERNEL_I, PROG_PHY]

    - name: _levels
      meters:
        volume:
          type: bar
          lights: [A21, A20, A19, A18, A17, A16, A15, A14, A13, A12, A11, A10, A9, A8, A7, A6, A5, A4, A3, A2, A1, A0]
        brightness_range:
          type: bar
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        speech_enabled:
          type: binary
          lights: [ADDR_22]

    - name: _entry
      meters:
        entry.address:
          type: binary
          lights: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]
        entry.data:
          type: binary
          lights: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]


# right to left
#[A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]
#[D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
#[ADDR_22, ADDR_18, ADDR_16, DATA, KERNEL, SUPER, USER, MASTER, PAUSE, RUN, ADRS_ERR, PAR_ERR, A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21, D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15, PAR_LO, PAR_HI, CONS_PHY, KERNEL_D, SUPER_D, USER_D, USER_I, SUPER_I, KERNEL_I, PROG_PHY, BUS_REG, DATA_PATHS, μADR_FPP_CPU, DISPLAY_REGISTER]

# left to right
#[A21, A20, A19, A18, A17, A16, A15, A14, A13, A12, A11, A10, A9, A8, A7, A6, A5, A4, A3, A2, A1, A0]
#[D15, D14, D13, D12, D11, D10, D9, D8, D7, D6, D5, D4, D3, D2, D1, D0]
#[DISPLAY_REGISTER, μADR_FPP_CPU, DATA_PATHS, BUS_REG, PROG_PHY, KERNEL_I, SUPER_I, USER_I, USER_D, SUPER_D, KERNEL_D, CONS_PHY, PAR_HI, PAR_LO, D15, D14, D13, D12, D11, D10, D9, D8, D7, D6, D5, D4, D3, D2, D1, D0, A21, A20, A19, A18, A17, A16, A15, A14, A13, A12, A11, A10, A9, A8, A7, A6, A5, A4, A3, A2, A1, A0, PAR_ERR, ADRS_ERR, RUN, PAUSE, MASTER, USER, SUPER, KERNEL, DATA, ADDR_16, ADDR_18, ADDR_22]
