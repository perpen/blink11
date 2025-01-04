package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Command struct {
	sync.Mutex
	name    string
	args    []string
	running bool
	quit    chan bool
	run     func(mode *Mode, mgr *Manager, quit chan bool)
}

type CommandCenter struct {
	sync.Mutex
	modeToCommand map[string]*Command
	toggles       chan *Mode
}

func NewCommandCenter() *CommandCenter {
	return &CommandCenter{
		modeToCommand: make(map[string]*Command, 0),
		toggles:       make(chan *Mode, 0),
	}
}

func (cc *CommandCenter) Start(mgr *Manager) {
	updateMeter := func() {
		cc.UpdateRunningMeter(mgr)
	}
	exitChan := make(chan *Mode, 0)
	go func() { // xxx yuck
		for {
			select {
			case mode := <-cc.toggles:
				Debug(LOG_COMMAND, "StartStop: got signals for %s", mode.name)
				cmd := cc.GetModeCommand(mode.name)
				if cmd == nil {
					Warn("no command for mode %s", mode.name)
					continue
				}
				if !cmd.running {
					Info("StartStop: command: %s/%s",
						mode.name, cmd.name)
					cmd.quit = make(chan bool, 0)
					cmd.running = true
					updateMeter()
					go func() {
						cmd.run(mode, mgr, cmd.quit)
						Info("StartStop: command ran: %s/%s",
							mode.name, cmd.name)
						exitChan <- mode
					}()
				} else {
					Info("StartStop: stopping command: %s/%s",
						mode.name, cmd.name)
					select {
					case cmd.quit <- true:
					default:
					}
				}
			case mode := <-exitChan:
				Debug(LOG_COMMAND, "StartStop: got exitChan for %s", mode.name)
				cmd := cc.GetModeCommand(mode.name)
				cmd.running = false
				updateMeter()
			}
		}
	}()
}

func (cc *CommandCenter) GetModeCommand(mode string) *Command {
	cc.Lock()
	defer cc.Unlock()
	cmd, ok := cc.modeToCommand[mode]
	if !ok {
		return nil
	}
	return cmd
}

func (cc *CommandCenter) StartStop(mgr *Manager) {
	mode := mgr.mode
	cc.toggles <- mode
}

// The RUN led is off if no command running on any mode, bright if command
// running on current mode, dimmed if running on another mode.
func (cc *CommandCenter) UpdateRunningMeter(mgr *Manager) {
	runningCount := 0
	currentModeRunning := false
	cc.Lock()
	for _, cmd := range cc.modeToCommand {
		if cmd.running {
			runningCount++
		}
	}
	cmd, ok := cc.modeToCommand[mgr.mode.name]
	cc.Unlock()
	currentModeRunning = ok && cmd.running

	switch {
	case runningCount == 0:
		mgr.Emit("running", 0, 1)
	case currentModeRunning:
		mgr.Emit("running", 1, 1)
	default:
		mgr.Emit("running", 1, 5)
	}
}

func (cc *CommandCenter) RegisterCommand(mode string, argv []string) {
	cmd := Command{
		name:    argv[0],
		args:    argv[1:],
		running: false,
	}
	switch cmd.name {
	case "timer":
		cmd.run = cmd.startTimer
	case "exec":
		cmd.run = cmd.startExec
	default:
		assert(false, "unknown command: %s", cmd.name)
	}
	cc.Lock()
	defer cc.Unlock()
	cc.modeToCommand[mode] = &cmd
}

func (cmd *Command) startTimer(mode *Mode, mgr *Manager, quit chan bool) {
	tickSound, _ := mgr.cons.LoadSound("stdout.raw")
	endSound, _ := mgr.cons.LoadSound("main-mission-to-eagle-intercomm.raw")
	trinity := mode.Read(0)
	seconds := trinity & 077
	minutes := trinity >> 6 & 077
	hours := trinity >> 12 & 077

	text := ""
	if hours > 0 {
		text = text + fmt.Sprintf("%d hours ", hours)
	}
	if minutes > 0 {
		text = text + fmt.Sprintf("%d minutes ", minutes)
	}
	if seconds > 0 {
		text = text + fmt.Sprintf("%d seconds ", seconds)
	}
	mgr.cons.Speak(text)

	initialSeconds := hours*60*60 + minutes*60 + seconds
	tickEnabled := mode.Read(1) != 0
	secondsLeft := initialSeconds
	Debug(LOG_COMMAND, "timer starting: %ds", secondsLeft)
	defer func() {
		mgr.Emit("timer.remaining_bar", 0, 1)
		mgr.Emit("timer.remaining_sec", 0, 1)
		mgr.Emit("timer.remaining_hours", 0, 1)
		mgr.Emit("timer.remaining_minutes", 0, 1)
		mgr.Emit("timer.remaining_seconds", 0, 1)
	}()
	ticker := time.NewTicker(time.Second)
	for {
		if tickEnabled {
			mgr.cons.PlaySound(tickSound)
		}
		mgr.Emit("timer.remaining_bar", int(secondsLeft), int(initialSeconds))
		sec := uint(secondsLeft)
		remHours := uint(sec / 60 / 60)
		sec -= remHours*60*60
		remMinutes := uint(sec/60)
		sec -= remMinutes*60
		remSeconds := uint(sec)
		// Info("h:m:s %d:%d:%d", remHours, remMinutes, remSeconds)
		mgr.Emit("timer.remaining_hours", int(remHours), int(remHours))
		mgr.Emit("timer.remaining_minutes", int(remMinutes), int(remMinutes))
		mgr.Emit("timer.remaining_seconds", int(remSeconds), int(remSeconds))
		if secondsLeft == 0 {
			mgr.cons.PlaySound(endSound)
			return
		}
		select {
		case <-quit:
			return
		case <-ticker.C:
			secondsLeft--
		}
	}
}

func (cmd *Command) startExec(mode *Mode, mgr *Manager, quit chan bool) {
	argv0 := cmd.args[0]
	var argv []string
	if len(cmd.args) > 1 {
		argv = cmd.args[1:]
	} else {
		argv = make([]string, 0)
	}
	Debug(LOG_COMMAND, "startExec starting: argv0=%s, argv=%v\n", argv0, argv)
	c := exec.Command(argv0, argv...)

	// Pass memory content as env vars
	c.Env = os.Environ()
	for _, addr := range mode.UsedAddresses() {
		data := mode.Read(addr)
		s := fmt.Sprintf("BLINK11_ADDR_%d=%d", addr, data)
		c.Env = append(c.Env, s)
		Debug(LOG_COMMAND, "startExec: env %s", s)
	}

	// Handle exit or kill
	go func() {
		_, ok := <-quit
		// Either the channel was closed after process exit,
		// or we got the quit message
		if ok { // got quit message
			err := c.Process.Kill()
			if err != nil {
				Warn("startExec: %v", err)
			}
		}
	}()

	// Scan command stdout for metrics
	c.Stderr = os.Stderr
	stdout, err := c.StdoutPipe()
	if err != nil {
		Warn("startExec: %v", err)
		return
	}
	err = c.Start()
	if err != nil {
		Warn("startExec: %v", err)
		return
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		Debug(LOG_COMMAND, "stdout line: "+string(line))
		prefix := "metric: "
		prefixLen := len(prefix)
		if strings.HasPrefix(line, prefix) {
			met, ok := parseMessage(line[prefixLen:])
			if !ok {
				Warn("startExec: invalid metric line: '%s'", line)
				continue
			}
			mgr.Emit(met.name, met.val, met.max, met.bad)
		}
	}
	if err = scanner.Err(); err != nil {
		Err("startExec: read error: %v", err)
	}

	err = c.Wait()
	if err != nil {
		Warn("startExec: %v", err)
		return
	}
}
