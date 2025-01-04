package main

import (
	"strings"
	"sync"
)

// The program's main control object.
type Manager struct {
	cons              *Console
	store             *MetricStore
	super             *SuperFeed
	superIn, superOut chan Metric
	commandCenter     *CommandCenter
	modesByName       map[string]*Mode
	mode              *Mode
	done, mainDone    chan bool
}

func NewManager(cfg Config, mainDone chan bool) *Manager {
	cons := NewConsole(cfg)
	cons.Start()
	mem := NewMemory(cfg)
	cc := NewCommandCenter()
	modesByName := makeModes(cfg, cons, mem, cc)
	store := NewMetricStore()
	superIn := make(chan Metric, 10)
	superOut := make(chan Metric, 10)
	super := NewSuperFeed(superIn, superOut, store)

	mgr := Manager{
		cons:          cons,
		store:         store,
		super:         super,
		superIn:       superIn,
		superOut:      superOut,
		commandCenter: cc,
		mainDone:      mainDone,
		modesByName:   modesByName,
	}

	cc.Start(&mgr)
	super.Start()
	super.Register("test", NewTestFeed(superIn))
	super.Register("network", NewNetworkFeed(superIn, cfg))
	super.Register("time", NewTimeFeed(superIn))
	super.Register("idle", NewIdleFeed(superIn))
	super.Register("effects", NewEffectsFeed(cons))
	super.Register("animation", NewAnimationFeed(cons))

	return &mgr
}

func (mgr *Manager) Start() {
	go mgr.eventLoop()
	go func() {
		for met := range mgr.superOut {
			mgr.consume(met)
		}
	}()
}

func (mgr *Manager) Stop() {
	mgr.super.Stop()
	mgr.cons.Stop()
}

// Switch to the mode, stopping/starting feeds as needed.
// Restore metrics from the store.
func (mgr *Manager) SelectMode(mode *Mode) {
	oldMode := mgr.mode
	Debug(LOG_MANAGER, "showMode: %s\n", mode.name)
	mgr.mode = mode

	mgr.commandCenter.UpdateRunningMeter(mgr)
	mgr.cons.ClearLeds()
	mgr.cons.Mute(true) // do not play sounds when restoring metrics
	for metricName := range mode.meters {
		metric := mgr.store.Get(metricName)
		mgr.consume(metric)
	}
	mgr.cons.Mute(false)
	mgr.cons.Speak(strings.ReplaceAll(mode.speech, "_", " "))

	if mode.Knobbable() {
		// Update knobs lights
		mgr.EmitKnobMetrics(mode.knoba, mode.knobd)
	}
	mgr.setupFeeds(oldMode, mode)
}

// Stop/start relevant feeds when switching mode
func (mgr *Manager) setupFeeds(oldMode, newMode *Mode) {
	if oldMode == nil {
		blankMode := Mode{}
		oldMode = &blankMode
	}
	Debug(LOG_FEED, "setupFeeds: %s → %s\n", oldMode.name, newMode.name)
	oldFeeds := oldMode.feeds
	newFeeds := newMode.feeds
	toStop := arrayToMap(oldFeeds)
	toStart := make([]string, 0)
	for _, feed := range newFeeds {
		if _, ok := toStop[feed]; ok {
			delete(toStop, feed)
		} else {
			toStart = append(toStart, feed)
		}
	}
	Debug(LOG_FEED, "setupFeeds: toStop: %v  toStart: %v\n", toStop, toStart)
	for feed := range toStop {
		mgr.super.Control(feed, false)
	}
	for _, feed := range toStart {
		mgr.super.Control(feed, true)
	}
}

// xxx use simple arrays
func arrayToMap(a []string) map[string]int {
	m := make(map[string]int, len(a))
	for _, x := range a {
		m[x] = 0
	}
	return m
}

func (mgr *Manager) Emit(name string, val, max int, bad ...bool) {
	mgr.superIn <- Metric{
		name: name,
		val:  val,
		max:  max,
		bad:  len(bad) > 0 && bad[0],
	}
}

// Update knob lights
func (mgr *Manager) EmitKnobMetrics(knoba, knobd int) {
	mgr.Emit("knoba", knoba+1, 8)
	mgr.Emit("knobd", knobd+1, 4)
}

func (mgr *Manager) ModeForKnobs(knoba, knobd int) *Mode {
	for _, mode := range mgr.modesByName {
		if mode.Knobbable() && mode.knoba == knoba && mode.knobd == knobd {
			return mode
		}
	}
	return nil
}

// Update the lights using the meter corresponding to the metric, if any
func (mgr *Manager) consume(met Metric) {
	if met.sound != "" {
		// mgr.cons.Speak(met.sound)
		id, err := mgr.cons.LoadSound(met.sound)
		if err != nil {
			Err("cannot load sound %s: %v", met.sound, err)
		} else {
			mgr.cons.PlaySound(id)
		}
	}
	if met.name != "" {
		if meter, ok := mgr.mode.meters[met.name]; ok {
			meter.Update(met)
		}
	}
}

// The MetricStore is used to restore metrics when switching modes
type MetricStore struct {
	sync.Mutex
	metrics map[string]Metric
}

func NewMetricStore() *MetricStore {
	return &MetricStore{
		metrics: make(map[string]Metric, 0),
	}
}

func (store *MetricStore) Get(name string) Metric {
	store.Lock()
	defer store.Unlock()
	return store.metrics[name]
}

func (store *MetricStore) Set(name string, metric Metric) {
	store.Lock()
	defer store.Unlock()
	store.metrics[name] = metric
}
