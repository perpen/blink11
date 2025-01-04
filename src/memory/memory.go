package memory

import (
	"log/slog"
	"os"
	"sync"

	"github.com/perpen/blink11/lib"
	"gopkg.in/yaml.v3"
)

// Memory space for each mode. Persisted to a file.
type ModeMemory map[uint]uint
type ModeMemories map[string]ModeMemory

var modeMemories ModeMemories
var path string
var lock sync.Mutex
var initialised bool

// Restores from file if present
func Init(pathParam string) {
	path = pathParam
	yml, err := os.ReadFile(path)
	if err == nil {
		slog.Info("loading memory from file", "path", path)
		err = yaml.Unmarshal([]byte(yml), &modeMemories)
		lib.Check(err == nil, "failed to parse memory file", "err", err)
	} else {
		slog.Info("memory file not found", "path", path)
		modeMemories = make(ModeMemories, 0)
	}
	initialised = true
}

func checkInitialised() {
	lib.Assert(initialised, "memory.Init was not called yet")
}

func save() {
	lock.Lock()
	buf, err := yaml.Marshal(&modeMemories)
	lock.Unlock()
	lib.Assert(err == nil, "failed to marshal memory", "err", err)
	err = os.WriteFile(path, buf, 0644)
	lib.Assert(err == nil, "failed to save memory file", "err", err)
}

// Value and boolean indicating whether defined
func Read(modeName string, addr uint) (uint, bool) {
	checkInitialised()
	lock.Lock()
	defer lock.Unlock()
	addrToData, ok := modeMemories[modeName]
	if !ok {
		return uint(0), false
	}
	return addrToData[addr], true
}

// Triggers a dump of the whole memory to a file.
func Write(modeName string, addr, data uint) {
	checkInitialised()
	lock.Lock()
	if _, ok := modeMemories[modeName]; !ok {
		modeMemories[modeName] = make(map[uint]uint, 1)
	}
	modeMemories[modeName][addr] = data
	lock.Unlock()
	go save()
}

func UsedAddresses(modeName string) []uint {
	checkInitialised()
	lock.Lock()
	defer lock.Unlock()
	addrToData, ok := modeMemories[modeName]
	if !ok {
		addrToData = make(map[uint]uint, 0)
	}
	addresses := make([]uint, 0)
	for addr := range addrToData {
		addresses = append(addresses, addr)
	}
	return addresses
}

func Volume() (float64, bool) {
	val, found := Read("system.levels", 0)
	return float64(val) / 100, found
}

func SaveVolume(vol float64) {
	go Write("system.levels", 0, uint(vol*100))
}

func Brightness() (float64, bool) {
	val, found := Read("system.levels", 1)
	return float64(val) / 100, found
}

func SaveBrightness(n float64) {
	go Write("system.levels", 1, uint(n*100))
}

func SpeechEnabled() (bool, bool) {
	val, found := Read("system.levels", 2)
	return val == 1, found
}

func SaveSpeechEnabled(e bool) {
	n := uint(0)
	if e {
		n = 1
	}
	go Write("system.levels", 2, n)
}
