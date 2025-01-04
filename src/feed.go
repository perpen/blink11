package main

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/perpen/blink11/config"
	"github.com/perpen/blink11/console"
	"github.com/perpen/blink11/lib"
	"github.com/perpen/blink11/logging"
	"github.com/perpen/pidp11"
)

type startStopper interface {
	start()
	stop()
}

type feedManager struct {
	feeds map[string]startStopper
}

func newFeedManager(cfg config.ConfigServer, bus Unibus) *feedManager {
	mgr := feedManager{
		feeds: map[string]startStopper{
			"network":   newNetworkFeed(cfg, bus),
			"time":      newTimeFeed(bus),
			"idle":      newIdleFeed(bus),
			"animation": newAnimationFeed(),
		},
	}
	return &mgr
}

func (mgr *feedManager) control(name string, b bool) {
	feed, ok := mgr.feeds[name]
	lib.Assert(ok, "control on unknown feed", "feed", name)
	if b {
		Debug(logging.LOG_FEED, "starting feed", "feed", name)
		feed.start()
	} else {
		Debug(logging.LOG_FEED, "stopping feed", "feed", name)
		feed.stop()
	}
}

// timeFeed generates metrics such as hours/minutes/etc.
type timeFeed struct {
	bus  Unibus
	done chan bool
}

func newTimeFeed(bus Unibus) startStopper {
	feed := timeFeed{
		bus:  bus,
		done: make(chan bool),
	}
	return &feed
}

func (feed *timeFeed) start() {
	go func() {
		ticker := time.NewTicker(40 * time.Millisecond)
		for {
			select {
			case <-feed.done:
				return
			case <-ticker.C:
				t := time.Now()
				am_pm := 2 // will be displayed with a binary meter
				if t.Hour() >= 12 {
					am_pm = 1
				}
				feed.bus <- Metric{name: "time.am_pm", val: am_pm, max: 2}
				feed.bus <- Metric{name: "time.hours", val: t.Hour(), max: 23}
				feed.bus <- Metric{name: "time.hours12", val: t.Hour() % 12, max: 11}
				feed.bus <- Metric{name: "time.minutes", val: t.Minute(), max: 59}
				feed.bus <- Metric{name: "time.seconds", val: t.Second(), max: 59}
				feed.bus <- Metric{
					name: "time.tenthseconds",
					val:  t.Nanosecond(),
					max:  1e9 - 1,
				}

				// cylon - back and forth between 0 and 9
				var cylonNs int
				if t.Second()%2 == 0 {
					cylonNs = t.Nanosecond()
				} else {
					cylonNs = 1e9 - t.Nanosecond()
				}
				feed.bus <- Metric{
					name: "time.tenthseconds_cylon",
					val:  cylonNs,
					max:  1e9,
				}
			}
		}
	}()
}

func (feed *timeFeed) stop() {
	feed.done <- true
}

// The networkFeed receives metrics on a tcp socket
type networkFeed struct {
	bus     Unibus
	addr    string
	started bool
}

func newNetworkFeed(cfg config.ConfigServer, bus Unibus) startStopper {
	feed := networkFeed{
		addr: cfg.Addr,
		bus:  bus,
	}
	return &feed
}

func (feed *networkFeed) start() {
	if feed.started {
		return
	}
	Debug(logging.LOG_FEED|logging.LOG_NET, "listening", "addr", feed.addr)
	l, err := net.Listen("tcp4", feed.addr)
	if err != nil {
		slog.Error("listen", "addr", feed.addr, "err", err)
		return
	}
	feed.started = true
	go func() {
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				slog.Error("accept", "err", err)
				return
			}
			go feed.handleConnection(c)
		}
	}()
}

// We collect network metrics always
func (feed *networkFeed) stop() {}

func (feed *networkFeed) handleConnection(c net.Conn) {
	defer c.Close()
	Debug(logging.LOG_FEED|logging.LOG_NET, "serving", "client", c.RemoteAddr().String())
	if true {
		// send message to agent
		go c.Write([]byte("hello\n"))
	}
	r := bufio.NewReader(c)
	line, err := r.ReadString('\n')
	if err == io.EOF {
		slog.Warn("eof from client on init")
		return
	}
	client := ""
	if strings.HasPrefix(line, "client ") {
		client = strings.TrimPrefix(line, "client ")
		client = strings.TrimSpace(client)
	} else {
		slog.Warn("networkFeed: invalid first line from client", "line", line)
		return
	}
	slog.Info("network client", "client", client)
	<-readMessages(client, r, feed.bus, nil)
	slog.Info("network client disconnected", "client", client)
}

// idleFeed approximates idle patterns for various systems
type idleFeed struct {
	bus  Unibus
	done chan bool
}

func newIdleFeed(bus Unibus) startStopper {
	feed := idleFeed{
		bus:  bus,
		done: make(chan bool),
	}
	return &feed
}

func (feed *idleFeed) start() {
	go func() {
		pdp11Ticker := time.NewTicker(150 * time.Millisecond)
		pdp11Counter := 0

		// https://www.youtube.com/watch?v=ZPabDl50VFI
		bsdTicker := time.NewTicker(75 * time.Millisecond)
		bsdCounter := 0

		// https://www.youtube.com/watch?v=ycADKwgnLpE
		rt11Ticker := time.NewTicker(50 * time.Millisecond)
		rt11Width := 15
		rt11Offset := 0
		rt11Step := 1

		for {
			select {
			case <-feed.done:
				return
			case <-pdp11Ticker.C:
				if false {
					// authentic: leds range of leds
					// use with binary meter
					pdp11Counter++
					n := pdp11Counter % 8
					four := uint(017)
					idle := (0377 & ((four<<(n+4) | four<<(n+12)) >> 8)) |
						(0377&((four<<(16-n)|four<<(8-n))>>8))<<8
					feed.bus <- Metric{
						name: "idle.pdp11",
						val:  int(idle),
						max:  int(idle),
					}
				} else {
					// fake: only leds up one led and relies on off_ms to
					// make it look like range of leds
					// use with binary meter
					pdp11Counter++
					n := pdp11Counter % 8
					one := uint(1)
					idle := (one << n) | (one << (8 - n) << 7)
					feed.bus <- Metric{
						name: "idle.pdp11",
						val:  int(idle),
						max:  int(idle),
					}
				}
			case <-bsdTicker.C:
				if false {
					// authentic: leds up range of leds
					// use with binary meter
					bsdCounter++
					n := bsdCounter % 16
					eight := uint(0377)
					idle := (eight<<n | eight<<(n+16)) >> 8
					feed.bus <- Metric{
						name: "idle.bsd",
						val:  int(idle),
						max:  int(idle),
					}
				} else {
					// fake: relies on off_ms to make it look like range of leds
					// use with dot meter
					bsdCounter++
					n := 1 + bsdCounter%16
					idle := n
					feed.bus <- Metric{
						name: "idle.bsd",
						val:  int(idle),
						max:  16,
					}
				}
			case <-rt11Ticker.C:
				rt11Offset += rt11Step
				if rt11Offset == 0 || rt11Offset+rt11Width == 16 {
					rt11Width--
					if rt11Width == 0 {
						rt11Width = 15
						rt11Offset = 0
						rt11Step = 1
					} else {
						rt11Step = -rt11Step
					}
				}
				idle := uint(0177777) >> (16 - rt11Width) << rt11Offset
				feed.bus <- Metric{
					name: "idle.rt11",
					val:  int(idle),
					max:  int(idle),
				}
			}
		}
	}()
}

func (feed *idleFeed) stop() {
	feed.done <- true
}

// This feed controls the leds directly instead of generating metrics
type animationFeed struct {
	done chan bool
}

func newAnimationFeed() startStopper {
	feed := animationFeed{
		done: make(chan bool),
	}
	return &feed
}

func (feed *animationFeed) start() {
	w := 22
	h := 3
	screen := [][]pidp11.LedID{
		{pidp11.LED_PAR_HI, pidp11.LED_PAR_LO, pidp11.LED_UNUSED1, pidp11.LED_D15, pidp11.LED_D14, pidp11.LED_D13, pidp11.LED_D12, pidp11.LED_D11, pidp11.LED_D10, pidp11.LED_D9, pidp11.LED_D8, pidp11.LED_D7, pidp11.LED_D6, pidp11.LED_D5, pidp11.LED_D4, pidp11.LED_D3, pidp11.LED_D2, pidp11.LED_D1, pidp11.LED_D0},
		{pidp11.LED_A21, pidp11.LED_A20, pidp11.LED_A19, pidp11.LED_A18, pidp11.LED_A17, pidp11.LED_A16, pidp11.LED_A15, pidp11.LED_A14, pidp11.LED_A13, pidp11.LED_A12, pidp11.LED_A11, pidp11.LED_A10, pidp11.LED_A9, pidp11.LED_A8, pidp11.LED_A7, pidp11.LED_A6, pidp11.LED_A5, pidp11.LED_A4, pidp11.LED_A3, pidp11.LED_A2, pidp11.LED_A1, pidp11.LED_A0},
		{pidp11.LED_PAR_ERR, pidp11.LED_ADRS_ERR, pidp11.LED_RUN, pidp11.LED_PAUSE, pidp11.LED_MASTER, pidp11.LED_USER, pidp11.LED_SUPER, pidp11.LED_KERNEL, pidp11.LED_DATA, pidp11.LED_ADDR_16, pidp11.LED_ADDR_18, pidp11.LED_ADDR_22},
	}

	//fx := pidp11.NewSimpleEffect(0, int(200*8*(1-console.GetBrightnessScaling())))
	pixel := func(x, y int, bright float64, fx pidp11.Effect) {
		switch y {
		case 0:
			x -= 3
		case 1:
		case 2:
			x -= 10
		}
		x = x % w
		y = y % h
		if x < 0 || x >= len(screen[y]) {
			return
		}
		led := screen[y][x]
		console.Led(led, fx, bright)
	}

	go func() {
		fx := pidp11.NewSimpleEffect(0, int(400))
		n := 0
		period := 10
		slant := 2
		ticker := time.NewTicker(40 * time.Millisecond)
		for {
			for y := range h {
				for x := range w {
					if (x*slant+y+n)%period == 0 {
						pixel(x+y, y, 1, fx)
						pixel(x+y, y, 0, fx)
					}
				}
			}
			select {
			case <-feed.done:
				return
			case <-ticker.C:
			}
			n++
		}
	}()
}

func (feed *animationFeed) stop() {
	feed.done <- true
}
