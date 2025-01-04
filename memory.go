package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Memory spaces for each mode. Persisted to a file.
type Memory struct {
	path             string
	ModeToAddrToData map[string]map[uint]uint `yaml:"ModeToAddrToData"`
}

// Restores from file if present
func NewMemory(cfg Config) *Memory {
	path := cfg.Manager.Memory
	mem := Memory{}
	yml, err := os.ReadFile(path)
	if err == nil {
		Debug(LOG_MEMORY, "loading memory state from %s", path)
		err = yaml.Unmarshal([]byte(yml), &mem)
		if err != nil {
			panic(err)
		}
	} else {
		Debug(LOG_MEMORY, "no memory state file: %s", path)
		mem.ModeToAddrToData = make(map[string]map[uint]uint, 0)
	}
	mem.path = path
	return &mem
}

// xxx should be async
func (mem *Memory) Save() {
	Debug(LOG_MEMORY, "dumping memory")
	buf, err := yaml.Marshal(&mem)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(mem.path, buf, 0644)
	if err != nil {
		Err("Memory.save(): %v", err)
	}
	Debug(LOG_MEMORY, "dumped memory")
}

func (mem *Memory) Read(modeName string, addr uint) uint {
	addrToData, ok := mem.ModeToAddrToData[modeName]
	if !ok {
		return uint(0)
	}
	return addrToData[addr]
}

// Triggers a dump of the whole memory to a file.
func (mem *Memory) Write(modeName string, addr, data uint) {
	if _, ok := mem.ModeToAddrToData[modeName]; !ok {
		mem.ModeToAddrToData[modeName] = make(map[uint]uint, 1)
	}
	mem.ModeToAddrToData[modeName][addr] = data
	mem.Save()
}

func (mem *Memory) UsedAddresses(modeName string) []uint {
	addrToData, ok := mem.ModeToAddrToData[modeName]
	if !ok {
		addrToData = make(map[uint]uint, 0)
	}
	addresses := make([]uint, len(addrToData))
	for addr := range addrToData {
		addresses = append(addresses, addr)
	}
	return addresses
}
