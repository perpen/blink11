package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"hfdom.org/blink11/v2/pidp11"
)

// The max value may be used or not, depending on the Meter type, but it
// should be >= val. Else bad will be set to true before the metric is passed
// to its meter.
type Metric struct {
	name     string
	val, max int
	bad      bool
	sound    string
}

type StartStopper interface {
	Start()
	Stop()
}

// The SuperFeed controls all feeds, collects their metrics.
// Incoming metrics are stored, and published to the output channel.
type SuperFeed struct {
	in, out chan Metric
	store   *MetricStore
	feeds   map[string]StartStopper
	done    chan bool
}

func NewSuperFeed(in, out chan Metric, store *MetricStore) *SuperFeed {
	feed := SuperFeed{
		in:    in,
		out:   out,
		store: store,
		feeds: make(map[string]StartStopper, 0),
		done:  make(chan bool, 0),
	}
	return &feed
}

func (super *SuperFeed) Start() {
	Debug(LOG_FEED, "SuperFeed.Start()\n")
	statsChan := make(chan Metric, 100)
	go func() {
		for met := range super.in {
			Debug(LOG_FEED, "SuperFeed: met=%v\n", met)
			super.store.Set(met.name, met)
			assert(met.name != "metrics_rx", "oops")
			select {
			case statsChan <- met:
			default:
			}
			super.out <- met
		}
	}()

	// Emit "metrics_rx" every slotMs, indicating count of metrics received
	// over last slots*slotMs milliseconds
	slots := 5
	slotMs := 100
	curSlot := 0
	timeSlots := make([]int, slots)
	var lock sync.Mutex
	max := 600 // arbitrary

	go func() { // Collect metrics count
		for range statsChan {
			Debug(LOG_FEED, "SuperFeed: masterChan\n")
			lock.Lock()
			timeSlots[curSlot]++
			lock.Unlock()
		}
	}()

	go func() { // Periodically summarise, emit metric
		ticker := time.NewTicker(time.Duration(slotMs) * time.Millisecond)
		for {
			<-ticker.C
			sum := 0
			lock.Lock()
			for _, count := range timeSlots {
				sum += count
			}
			curSlot = (curSlot + 1) % slots
			timeSlots[curSlot] = 0
			lock.Unlock()

			if sum > max {
				Warn("SuperFeed/metrics_rx: while computing metrics_rx, %d > %d",
					sum, max)
				sum = max
			}
			Debug(LOG_FEED, "SuperFeed/metrics_rx: %d/%d", sum, max)
			met := Metric{
				name: "metrics_rx",
				val:  sum,
				max:  max,
			}
			super.out <- met
		}
	}()
}

func (super *SuperFeed) Stop() {}

func (super *SuperFeed) Register(name string, feed StartStopper) {
	if _, ok := super.feeds[name]; ok {
		log.Fatalf("feed already registered: %s", name)
	}
	super.feeds[name] = feed
}

func (super *SuperFeed) Control(name string, b bool) {
	feed, ok := super.feeds[name]
	assert(ok, "unknown feed: %s", name)
	if b {
		Debug(LOG_FEED, "starting feed %s\n", name)
		feed.Start()
	} else {
		Debug(LOG_FEED, "stopping feed %s\n", name)
		feed.Stop()
	}
}

// Invoked when pressing the TEST switch.
type TestFeed struct {
	out  chan Metric
	done chan bool
}

func NewTestFeed(out chan Metric) StartStopper {
	feed := TestFeed{
		out:  out,
		done: make(chan bool, 0),
	}
	return &feed
}

func (feed *TestFeed) Start() {
	Debug(LOG_FEED, "TestFeed.Start()\n")
	feed.out <- Metric{name: "test.bar", val: 1, max: 1}
	feed.out <- Metric{name: "test.flash_slow", val: 1, max: 10}
	feed.out <- Metric{name: "test.flash_fast", val: 100, max: 100}
	feed.out <- Metric{name: "test.strobe_slow", val: 1, max: 10}
	feed.out <- Metric{name: "test.strobe_fast", val: 100, max: 100}
	feed.out <- Metric{name: "test.on_ms", val: 10, max: 100}
	feed.out <- Metric{name: "test.off_ms", val: 10, max: 100}
	feed.out <- Metric{name: "test.error", bad: true}
}

func (feed *TestFeed) Stop() {}

// TimeFeed generates metrics such as hours/minutes/etc.
type TimeFeed struct {
	out  chan Metric
	done chan bool
}

func NewTimeFeed(out chan Metric) StartStopper {
	feed := TimeFeed{
		out:  out,
		done: make(chan bool, 0),
	}
	return &feed
}

func (feed *TimeFeed) Start() {
	Debug(LOG_FEED, "TimeFeed.Start()\n")
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
				feed.out <- Metric{name: "time.am_pm", val: am_pm, max: 2}
				feed.out <- Metric{name: "time.hours", val: t.Hour(), max: 23}
				feed.out <- Metric{name: "time.hours12", val: t.Hour() % 12, max: 11}
				feed.out <- Metric{name: "time.minutes", val: t.Minute(), max: 59}
				feed.out <- Metric{name: "time.seconds", val: t.Second(), max: 59}
				feed.out <- Metric{
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
				feed.out <- Metric{
					name: "time.tenthseconds_cylon",
					val:  cylonNs,
					max:  1e9,
				}
			}
		}
	}()
}

func (feed *TimeFeed) Stop() {
	Debug(LOG_FEED, "TimeFeed.Stop()\n")
	feed.done <- true
}

// The NetworkFeed receives metrics on a tcp socket, see ./comm/
type NetworkFeed struct {
	out     chan Metric
	addr    string
	started bool
}

func NewNetworkFeed(out chan Metric, cfg Config) StartStopper {
	feed := NetworkFeed{
		addr: cfg.Server.Addr,
		out:  out,
	}
	return &feed
}

func (feed *NetworkFeed) Start() {
	Debug(LOG_FEED|LOG_NET, "NetworkFeed.Start()\n")
	if feed.started {
		return
	}
	Debug(LOG_FEED|LOG_NET, "listening on %v\n", feed.addr)
	l, err := net.Listen("tcp4", feed.addr)
	if err != nil {
		Err("%v", err)
		return
	}
	feed.started = true
	go func() {
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				Err("%v", err)
				return
			}
			go feed.handleConnection(c)
		}
	}()
}

func (feed *NetworkFeed) Stop() {}

func (feed *NetworkFeed) handleConnection(c net.Conn) {
	defer c.Close()
	Debug(LOG_FEED|LOG_NET, "Serving %s\n", c.RemoteAddr().String())
	if false {
		// send messages to agent
		go func() {
			for i := 0; i < 5; i++ {
				c.Write([]byte("S\n"))
				time.Sleep(1000 * time.Millisecond)
			}
		}()
	}
	r := bufio.NewReader(c)
	line, err := r.ReadString('\n')
	if err == io.EOF {
		Warn("eof")
		return
	}
	client := ""
	if strings.HasPrefix(line, "client ") {
		client = line[len("client ") : len(line)-1]
	} else {
		Warn("NetworkFeed: invalid first line from client: %s", line)
		return
	}
	Info("network client: %s", client)
	seen := make(map[string]bool, 0) // to show error envelopes on disconnection
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			Warn("eof from %s", client)
			for metricName := range seen {
				feed.out <- Metric{name: metricName, bad: true}
			}
			return
		}
		if err != nil {
			Err("%s: %v\n", client, err)
			return
		}
		met, ok := parseMessage(line)
		if !ok {
			Warn("network: invalid metric line: '%s'", line)
			continue
		}
		seen[met.name] = true
		Debug(LOG_FEED|LOG_NET, "%s: received: %v\n", client, met)
		feed.out <- met
	}
}

var metricLineRe = regexp.MustCompile(`^([-_.A-Za-z0-9]+) +([0-9]+) +([0-9]+)( +(false|true))?$`)
var metricSoundLineRe = regexp.MustCompile(`^sound: *(.+)$`)

func parseMessage(line string) (Metric, bool) {
	line = strings.TrimRight(line, "\n")
	tokens := metricSoundLineRe.FindStringSubmatch(line)
	if tokens != nil {
		sound := tokens[1]
		Debug(LOG_FEED|LOG_NET, "received sound: %v\n", sound)
		return Metric{sound: sound}, true
	}
	tokens = metricLineRe.FindStringSubmatch(line)
	if tokens == nil {
		return Metric{}, false
	}
	atoi := func(i int) int {
		n, _ := strconv.Atoi(tokens[i])
		return n
	}
	return Metric{
		name: tokens[1],
		val:  atoi(2),
		max:  atoi(3),
		bad:  tokens[5] == "true",
	}, true
}

// IdleFeed approximates idle patterns for various systems
type IdleFeed struct {
	out  chan Metric
	done chan bool
}

func NewIdleFeed(out chan Metric) StartStopper {
	feed := IdleFeed{
		out:  out,
		done: make(chan bool, 0),
	}
	return &feed
}

func (feed *IdleFeed) Start() {
	Debug(LOG_FEED, "IdleFeed.Start()\n")
	go func() {
		pdp11Ticker := time.NewTicker(150 * time.Millisecond)
		pdp11Counter := 0

		// https://www.youtube.com/watch?v=ZPabDl50VFI
		bsdTicker := time.NewTicker(55 * time.Millisecond)
		bsdCounter := 0

		// https://www.youtube.com/watch?v=ycADKwgnLpE
		rt11Ticker := time.NewTicker(80 * time.Millisecond)
		rt11Width := 15
		rt11Offset := 0
		rt11Step := 1

		for {
			select {
			case <-feed.done:
				return
			case <-pdp11Ticker.C:
				pdp11Counter++
				n := pdp11Counter % 8
				four := uint(017)
				idle := (0377 & ((four<<(n+4) | four<<(n+12)) >> 8)) |
					(0377&((four<<(16-n)|four<<(8-n))>>8))<<8
				feed.out <- Metric{
					name: "idle.pdp11",
					val:  int(idle),
					max:  int(idle),
				}
			case <-bsdTicker.C:
				bsdCounter++
				n := bsdCounter % 16
				eight := uint(0377)
				idle := (eight<<n | eight<<(n+16)) >> 8
				feed.out <- Metric{
					name: "idle.bsd",
					val:  int(idle),
					max:  int(idle),
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
				feed.out <- Metric{
					name: "idle.rt11",
					val:  int(idle),
					max:  int(idle),
				}
			}
		}
	}()
}

func (feed *IdleFeed) Stop() {
	Debug(LOG_FEED, "IdleFeed.Stop()\n")
	feed.done <- true
}

// This feed controls the lights directly instead of generating metrics
type EffectsFeed struct {
	cons *Console
}

func NewEffectsFeed(cons *Console) StartStopper {
	feed := EffectsFeed{
		cons: cons,
	}
	return &feed
}

func (feed *EffectsFeed) Start() {
	Debug(LOG_FEED, "EffectsFeed.Start()\n")

	lumenLeds := []pidp11.LedID{pidp11.LED_ADDR_22, pidp11.LED_ADDR_18, pidp11.LED_ADDR_16, pidp11.LED_DATA, pidp11.LED_KERNEL, pidp11.LED_SUPER, pidp11.LED_USER, pidp11.LED_MASTER, pidp11.LED_PAUSE, pidp11.LED_RUN, pidp11.LED_ADRS_ERR}
	flashLeds := []pidp11.LedID{pidp11.LED_A0, pidp11.LED_A1, pidp11.LED_A2, pidp11.LED_A3, pidp11.LED_A4, pidp11.LED_A5, pidp11.LED_A6, pidp11.LED_A7, pidp11.LED_A8, pidp11.LED_A9, pidp11.LED_A10}
	strobeLeds := []pidp11.LedID{pidp11.LED_D0, pidp11.LED_D1, pidp11.LED_D2, pidp11.LED_D3, pidp11.LED_D4, pidp11.LED_D5, pidp11.LED_D6, pidp11.LED_D7, pidp11.LED_D8, pidp11.LED_D9, pidp11.LED_D10}

	lumenFx := pidp11.NewSimpleEffect(0, 0)
	for i, id := range lumenLeds {
		p := float64(i) / float64(len(lumenLeds)-1)
		feed.cons.SetLed(id, lumenFx, p)
	}

	flashFx := feed.cons.NewFlashEffect(0, 0)
	for i, id := range flashLeds {
		p := float64(i) / float64(len(lumenLeds)-1)
		feed.cons.SetLed(id, flashFx, 1, p)
	}

	strobeFx := feed.cons.NewStrobeEffect(0, 0)
	for i, id := range strobeLeds {
		p := float64(i) / float64(len(lumenLeds)-1)
		feed.cons.SetLed(id, strobeFx, 1, p)
	}
}

func (feed *EffectsFeed) Stop() {}

// This feed controls the lights directly instead of generating metrics
type AnimationFeed struct {
	cons *Console
	done chan bool
}

func NewAnimationFeed(cons *Console) StartStopper {
	feed := AnimationFeed{
		cons: cons,
		done: make(chan bool, 0),
	}
	return &feed
}

func (feed *AnimationFeed) Start() {
	Debug(LOG_FEED, "AnimationFeed.Start()\n")
	w := 22
	h := 3
	screen := [][]pidp11.LedID{
		{pidp11.LED_PAR_HI, pidp11.LED_PAR_LO, pidp11.LED_UNUSED1, pidp11.LED_D15, pidp11.LED_D14, pidp11.LED_D13, pidp11.LED_D12, pidp11.LED_D11, pidp11.LED_D10, pidp11.LED_D9, pidp11.LED_D8, pidp11.LED_D7, pidp11.LED_D6, pidp11.LED_D5, pidp11.LED_D4, pidp11.LED_D3, pidp11.LED_D2, pidp11.LED_D1, pidp11.LED_D0},
		{pidp11.LED_A21, pidp11.LED_A20, pidp11.LED_A19, pidp11.LED_A18, pidp11.LED_A17, pidp11.LED_A16, pidp11.LED_A15, pidp11.LED_A14, pidp11.LED_A13, pidp11.LED_A12, pidp11.LED_A11, pidp11.LED_A10, pidp11.LED_A9, pidp11.LED_A8, pidp11.LED_A7, pidp11.LED_A6, pidp11.LED_A5, pidp11.LED_A4, pidp11.LED_A3, pidp11.LED_A2, pidp11.LED_A1, pidp11.LED_A0},
		{pidp11.LED_PAR_ERR, pidp11.LED_ADRS_ERR, pidp11.LED_RUN, pidp11.LED_PAUSE, pidp11.LED_MASTER, pidp11.LED_USER, pidp11.LED_SUPER, pidp11.LED_KERNEL, pidp11.LED_DATA, pidp11.LED_ADDR_16, pidp11.LED_ADDR_18, pidp11.LED_ADDR_22},
	}

	//fx := pidp11.NewSimpleEffect(0, int(200*8*(1-feed.cons.GetBrightnessScaling())))
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
		feed.cons.SetLed(led, fx, bright)
	}

	go func() {
		switch 1 {
		case 1:
			fx := pidp11.NewSimpleEffect(0, int(1100))
			n := 0
			period := 10
			slant := 2
			ticker := time.NewTicker(100 * time.Millisecond)
			for {
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
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
		case 2:
			fx := pidp11.NewSimpleEffect(0, 1000)
			n := 0
			b := 1.0
			ticker := time.NewTicker(30 * time.Millisecond)
			for {
				for y := h - 1; y >= 0; y-- {
					for x := 0; x < w; x++ {
						b -= 1.0 / 50
						if b <= 0 {
							return
						}
						pixel(x+y, y, b, fx)
						pixel(x+y, y, 0, fx)
						select {
						case <-feed.done:
							return
						case <-ticker.C:
						}
					}
				}
				n++
			}
		case 3:
			fx := pidp11.NewSimpleEffect(0, 2000)
			for y := h - 1; y >= 0; y-- {
				for x := 0; x < w; x++ {
					pixel(x+y, y, 1, fx)
					pixel(x+y, y, 0, fx)
				}
			}
			<-feed.done
		}
	}()
}

func (feed *AnimationFeed) Stop() {
	Debug(LOG_FEED, "AnimationFeed.Stop()\n")
	feed.done <- true
}
