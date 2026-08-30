// Package fpengine — runtime fingerprint data loader.
//
// Loads real fingerprint data from an external JSON file at runtime, with an
// embedded fallback so the binary works offline. The JSON can be updated via
// `bes update-fp` without recompiling.
//
// Data source: apify/fingerprint-suite network.json (Apache-2.0).
// 773 real GPU combos, 4569 screen configs, 39 hardwareConcurrency values.
package fpengine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FpRealData holds all real fingerprint data loaded from JSON.
type FpRealData struct {
	Version            string         `json:"version"`
	Source             string         `json:"source"`
	GeneratedAt        string         `json:"generated_at"`
	GPUs               []FpRealGPU    `json:"gpus"`
	Screens            []FpRealScreen `json:"screens"`
	Fonts              [][]string     `json:"fonts"`
	ChromeUAs          []string       `json:"chrome_uas"`
	HardwareConcurrency []int          `json:"hardware_concurrency"`
	DeviceMemory       []int          `json:"device_memory"`
}

// fpDataPath is the runtime JSON path. Override via BES_FP_DATA env or
// default to <cwd>/data/fp_real_data.json.
var fpDataPath = defaultFpDataPath()

// fpDataCache is the loaded data (nil = use embedded fallback).
var (
	fpDataCache   *FpRealData
	fpDataCacheMu sync.RWMutex
)

func defaultFpDataPath() string {
	if p := os.Getenv("BES_FP_DATA"); p != "" {
		return p
	}
	candidates := []string{
		"data/fp_real_data.json",
		filepath.Join(execDir(), "data", "fp_real_data.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "data/fp_real_data.json"
}

// LoadFpRealData loads fingerprint data from the JSON file. If the file is
// missing or invalid, falls back to embedded data (FpRealGPUs etc.) so the
// engine always works. Thread-safe; safe to call at startup and on updates.
func LoadFpRealData() (*FpRealData, error) {
	fpDataCacheMu.RLock()
	if fpDataCache != nil {
		d := fpDataCache
		fpDataCacheMu.RUnlock()
		return d, nil
	}
	fpDataCacheMu.RUnlock()

	data, err := loadFromJSON(fpDataPath)
	if err != nil {
		// Log the fallback so a polluted/broken embedded dataset is observable.
		// Previously this was silent: a corrupt or missing JSON file would
		// invisibly fall back to the compile-time embedded data, hiding
		// tainted values (e.g. hwConc 640) from operators.
		log.Printf("fpengine: fp data load failed (%s), falling back to embedded data: %v", fpDataPath, err)
		return embeddedFallback(), nil
	}

	fpDataCacheMu.Lock()
	fpDataCache = data
	fpDataCacheMu.Unlock()
	return data, nil
}

// ReloadFpRealData forces a reload from JSON (used after `bes update-fp`
// downloads new data). Clears cache and re-reads the file.
func ReloadFpRealData() (*FpRealData, error) {
	fpDataCacheMu.Lock()
	fpDataCache = nil
	fpDataCacheMu.Unlock()
	return LoadFpRealData()
}

// FpDataLastModified returns the modification time of the JSON data file,
// or time.Time{} if not found.
func FpDataLastModified() time.Time {
	info, err := os.Stat(fpDataPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// FpDataVersion returns the version string from the loaded data.
func FpDataVersion() string {
	d, err := LoadFpRealData()
	if err != nil || d == nil {
		return "embedded-fallback"
	}
	return d.Version + " (" + d.GeneratedAt + ")"
}

// FpDataStats returns a summary of loaded data counts.
func FpDataStats() map[string]int {
	d, err := LoadFpRealData()
	if err != nil || d == nil {
		return map[string]int{
			"gpus":                 len(FpRealGPUs),
			"screens":              len(FpRealScreens),
			"hardware_concurrency": len(FpRealHardwareConcurrency),
			"device_memory":        len(FpRealDeviceMemory),
			"chrome_uas":           len(FpRealChromeUAs),
			"source":               0,
		}
	}
	return map[string]int{
		"gpus":                 len(d.GPUs),
		"screens":              len(d.Screens),
		"hardware_concurrency": len(d.HardwareConcurrency),
		"device_memory":        len(d.DeviceMemory),
		"chrome_uas":           len(d.ChromeUAs),
		"source":               1,
	}
}

func loadFromJSON(path string) (*FpRealData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fp data %s: %w", path, err)
	}
	var d FpRealData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse fp data: %w", err)
	}
	if len(d.GPUs) == 0 {
		return nil, fmt.Errorf("fp data has no GPUs")
	}
	return &d, nil
}

func embeddedFallback() *FpRealData {
	return &FpRealData{
		Version:             "embedded",
		GeneratedAt:         "compile-time",
		GPUs:                FpRealGPUs,
		Screens:             FpRealScreens,
		HardwareConcurrency: FpRealHardwareConcurrency,
		DeviceMemory:        FpRealDeviceMemory,
		ChromeUAs:           FpRealChromeUAs,
	}
}

// CurrentFpRealGPUs returns the currently active GPU list (JSON or embedded).
func CurrentFpRealGPUs() []FpRealGPU {
	d, _ := LoadFpRealData()
	if d != nil && len(d.GPUs) > 0 {
		return d.GPUs
	}
	return FpRealGPUs
}

// CurrentFpRealScreens returns the currently active screen list.
func CurrentFpRealScreens() []FpRealScreen {
	d, _ := LoadFpRealData()
	if d != nil && len(d.Screens) > 0 {
		return d.Screens
	}
	return FpRealScreens
}

// CurrentFpHardwareConcurrency returns the currently active CPU core counts.
func CurrentFpHardwareConcurrency() []int {
	d, _ := LoadFpRealData()
	if d != nil && len(d.HardwareConcurrency) > 0 {
		return d.HardwareConcurrency
	}
	return FpRealHardwareConcurrency
}

// CurrentFpDeviceMemory returns the currently active device memory values.
func CurrentFpDeviceMemory() []int {
	d, _ := LoadFpRealData()
	if d != nil && len(d.DeviceMemory) > 0 {
		return d.DeviceMemory
	}
	return FpRealDeviceMemory
}

func execDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
