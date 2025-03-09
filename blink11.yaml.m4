# PI_PERSISTENT_DIR, PI_RUN_DIR are m4 macros

pidp:
  brightness:
    min: .03
    max: 1
  frequencies:
    min_hz: .5
    max_hz: 10
    one_hz_at: .1
  speak: true
server:
  addr: :3333
audio:
  impl: pipewire
  rate: 24000
  latency_ms: 10
  sounds_dir: PI_PERSISTENT_DIR/sounds

  tts_language: en
  tts_volume_factor: .5
  tts_speed_factor: 1.2
  cache_dir: PI_PERSISTENT_DIR/tmp

  startup_sound: sensor_sweep.raw
  volume_sound: click5.raw
  # knob_ok_sound: click5.raw
  # knob_ko_sound: click4.raw
memory_path: PI_PERSISTENT_DIR/memory.yaml
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
 # - reader


sections:
  - section: modes only meant to be imported, by convention names start with "_"
    hidden: true
    modes:
    - name: _idle_bsd
      meters:
        idle.bsd:
          type: dot
          on_ms: 0
          off_ms: 650
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

    - name: _idle_pdp11
      meters:
        idle.pdp11:
          type: binary
          on_ms: 0
          off_ms: 700
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

    - name: _idle_rt11
      meters:
        idle.rt11:
          type: binary
          on_ms: 200
          off_ms: 200
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]

# usage: m4_machine_stats(HOST, OFF_MS, CPU_METER, LOAD_METER, IDLE_TYPE, SPEECH, CPU_GATE)
define(m4_machine_stats, `
    - name: $1_load
      speech: $6
      imports: [_idle_$5]
      feeds: [network, idle]
      meters:
        $1.disk.reads:
          type: $3
          on_ms: 200
          off_ms: $2
          leds: [PAR_ERR]
        $1.disk.writes:
          type: $3
          on_ms: 200
          off_ms: $2
          leds: [ADRS_ERR]
        $1.tx_packets:
          type: $3
          on_ms: 200
          off_ms: $2
          leds: [PAR_HI]
        $1.rx_packets:
          type: $3
          on_ms: 200
          off_ms: $2
          leds: [PAR_LO]
        $1.cpu.user:
          type: $3
          off_ms: $2
          leds: [USER]
        $1.cpu.system:
          type: $3
          off_ms: $2
          leds: [SUPER]
        $1.cpu.iowait:
          type: $3
          off_ms: $2
          leds: [KERNEL]
        $1.load.one:
          type: $4
          on_ms: 200
          off_ms: $2
          leds: [ADDR_16]
        $1.load.five:
          type: $4
          on_ms: 200
          off_ms: $2
          leds: [ADDR_18]
        $1.load.fifteen:
          type: $4
          on_ms: 200
          off_ms: $2
          leds: [ADDR_22]
        # To reduce noise we gate the metrics
        $1.cpu0.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A0]
        $1.cpu1.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A1]
        $1.cpu2.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A2]
        $1.cpu3.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A3]
        $1.cpu4.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A4]
        $1.cpu5.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A5]
        $1.cpu6.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A6]
        $1.cpu7.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A7]
        $1.cpu8.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A8]
        $1.cpu9.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A9]
        $1.cpu10.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A10]
        $1.cpu11.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A11]
        $1.cpu12.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A12]
        $1.cpu13.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A13]
        $1.cpu14.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A14]
        $1.cpu15.cpu:
          type: $3
          gate: $7
          off_ms: $2
          leds: [A15]
')

  - section: machines stats
    modes:
m4_machine_stats(ptoseis, 200, lumen, lumen, bsd, laptop, .15)
m4_machine_stats(pdp11, 200, lumen, lumen, pdp11, pdp, .2)
m4_machine_stats(dell, 200, strobe, lumen, rt11, proxmox, .1)

  - section: home assistant
    modes:
    - name: home
      control:
        argv: [PI_RUN_DIR/scripts/homeassistant/run.sh]
        autostart: true
      meters:
        ha.plug.office:
          type: binary
          leds: [D0]
        ha.plug.corridor:
          type: binary
          leds: [D1]
        ha.plug.music_room:
          type: binary
          leds: [D2]
        ha.light.square_bulb:
          type: binary
          leds: [D3]
        ha.light.low_bulb:
          type: binary
          leds: [D4]
        ha.light.high_bulb:
          type: binary
          leds: [D5]
        ha.weather.temp:
          type: binary
          leds: [A0, A1, A2, A3, A4, A5]
        ha.office.temp:
          type: binary
          leds: [A6, A7, A8, A9, A10, A11]
        ha.car.level:
          type: binary
          leds: [D12, D13, D14, D15]
        ha.car.status:
          type: strobe
          leds: [PAR_HI]
        ha.chris.home:
          type: binary
          leds: [PAR_ERR]
        ha.ginger.home:
          type: binary
          leds: [ADRS_ERR]

  - section: time
    modes:
    - name: time_octal
      speech: time
      feeds: [time]
      meters:
        time.tenthseconds_cylon:
          type: dot
          off_ms: 500
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        time.seconds:
          type: binary
          leds: [A0, A1, A2, A3, A4, A5]
        time.minutes:
          type: binary
          leds: [A6, A7, A8, A9, A10, A11]
        time.hours12:
          type: binary
          leds: [A12, A13, A14, A15, A16, A17]
        time.am_pm:
          type: binary
          leds: [PAR_HI, PAR_LO]
    - name: shell stopwatch
      control:
        argv: [PI_RUN_DIR/scripts/stopwatch.sh]
        ticker_period_ms: 100
      meters:
        shell-stopwatch:
          type: binary
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        shell-stopwatch-tenths:
          type: bar
          leds: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10]
    - name: python timer
      control:
        argv: [PI_RUN_DIR/scripts/timer.py]
        ticker_period_ms: 1000
      meters:
        python-timer-counter:
          type: binary
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        python-timer-progress:
          type: bar
          leds: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]

  - section: experiments
    modes:
    - name: animation
      feeds: [animation]
    - name: meters demo
      feeds: [time]
      control:
        argv: [PI_RUN_DIR/scripts/meters-demo.sh]
        ticker_period_ms: 600
      meters:
        demo.binary:
          type: binary
          leds: [ADDR_22, ADDR_18, ADDR_16, DATA]
        demo.lumen:
          type: lumen
          leds: [A18, A19, A20, A21]
        demo.flash:
          type: flash
          leds: [PAR_HI]
        demo.strobe:
          type: strobe
          leds: [PAR_LO]
        demo.bar:
          type: bar
          leds: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9]
        demo.dot:
          type: dot
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9]
        demo.flash_attack:
          type: flash
          on_ms: 800
          leds: [A12]
        demo.flash_decay:
          type: flash
          off_ms: 800
          leds: [A13]
        demo.flash_attack_decay:
          type: flash
          on_ms: 800
          off_ms: 800
          leds: [A14]
        demo.error:
          type: lumen
          leds: [PAR_ERR]
    - name: time_easy
      speech: easy time
      feeds: [time]
      meters:
        time.minutes:
          type: bar
          leds: [A5, A4, A3, A2, A1, A0, D5, D4, D3, D2, D1, D0]
        time.hours12:
          type: bar
          floor: true
          leds: [A14, A13, A12, A11, A10, A9, D14, D13, D12, D11, D10]
        time.am_pm:
          type: binary
          leds: [PAR_HI, PAR_LO]
    - name: awk timer
      control:
        argv: [mawk, -W, interactive, -f, PI_RUN_DIR/scripts/timer.awk]
        ticker_period_ms: 1000
      meters:
        awk-timer:
          type: binary
          leds: [D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
        awk-timer-progress:
          type: bar
          leds: [A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]

  - section: more experiments
    modes:
    - name: alphabet
      control:
        argv: [PI_RUN_DIR/scripts/alphabet.py]
        ticker_period_ms: 1000
        autostart: true
      meters:
        alphabet.1:
          type: binary
          leds: [PAR_ERR, ADRS_ERR, RUN, A11, A10, A9, D11, D10, D9]
        alphabet.2:
          type: binary
          leds: [PAUSE, MASTER, USER, A8, A7, A6, D8, D7, D6]
        alphabet.3:
          type: binary
          leds: [SUPER, KERNEL, DATA, A5, A4, A3, D5, D4, D3]
        alphabet.4:
          type: binary
          leds: [ADDR_16, ADDR_18, ADDR_22, A2, A1, A0, D2, D1, D0]


## leds reference
# right to left
#[A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21]
#[D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15]
#[ADDR_22, ADDR_18, ADDR_16, DATA, KERNEL, SUPER, USER, MASTER, PAUSE, RUN, ADRS_ERR, PAR_ERR, A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11, A12, A13, A14, A15, A16, A17, A18, A19, A20, A21, D0, D1, D2, D3, D4, D5, D6, D7, D8, D9, D10, D11, D12, D13, D14, D15, PAR_LO, PAR_HI, CONS_PHY, KERNEL_D, SUPER_D, USER_D, USER_I, SUPER_I, KERNEL_I, PROG_PHY, BUS_REG, DATA_PATHS, μADR_FPP_CPU, DISPLAY_REGISTER]
# left to right
#[A21, A20, A19, A18, A17, A16, A15, A14, A13, A12, A11, A10, A9, A8, A7, A6, A5, A4, A3, A2, A1, A0]
#[D15, D14, D13, D12, D11, D10, D9, D8, D7, D6, D5, D4, D3, D2, D1, D0]
#[DISPLAY_REGISTER, μADR_FPP_CPU, DATA_PATHS, BUS_REG, PROG_PHY, KERNEL_I, SUPER_I, USER_I, USER_D, SUPER_D, KERNEL_D, CONS_PHY, PAR_HI, PAR_LO, D15, D14, D13, D12, D11, D10, D9, D8, D7, D6, D5, D4, D3, D2, D1, D0, A21, A20, A19, A18, A17, A16, A15, A14, A13, A12, A11, A10, A9, A8, A7, A6, A5, A4, A3, A2, A1, A0, PAR_ERR, ADRS_ERR, RUN, PAUSE, MASTER, USER, SUPER, KERNEL, DATA, ADDR_16, ADDR_18, ADDR_22]
