// Package fpengine — device profile data loader.
//
// Loads the joint device-profile distribution from a JSON file at runtime.
// The file is produced by `bes update-fp` (see internal/fpupdate/parser.go)
// from the apify/fingerprint-suite Bayesian network; a committed copy lives at
// data/device_profiles.json so the engine works out of the box.
//
// Data source: apify/fingerprint-suite network.json (Apache-2.0), modeled as a
// Bayesian network where every fingerprint dimension (videoCard, screen,
// fonts, hardwareConcurrency, deviceMemory, platform, vendor, maxTouchPoints)
// is conditioned on the browser User-Agent.
//
// When the file is absent, loadDeviceProfileStore returns (nil, nil) and the
// engine falls back to the legacy independent sampler.
package fpengine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// deviceProfilePath is the runtime JSON path. Override via BES_DEVICE_PROFILE
// or default to <cwd>/data/device_profiles.json.
var deviceProfilePath = defaultDeviceProfilePath()

var (
	deviceProfileCache   *deviceProfileStore
	deviceProfileCacheMu sync.RWMutex
	deviceProfileErr     error
	deviceProfileLoaded  bool
)

func defaultDeviceProfilePath() string {
	if p := os.Getenv("BES_DEVICE_PROFILE"); p != "" {
		return p
	}
	candidates := []string{
		"data/device_profiles.json",
		filepath.Join(execDir(), "data", "device_profiles.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "data/device_profiles.json"
}

// loadDeviceProfileStore returns the current device profile store, loading it
// from the runtime JSON on first use. A nil store (with nil error) means the
// engine should use the legacy sampler.
func loadDeviceProfileStore() (*deviceProfileStore, error) {
	deviceProfileCacheMu.RLock()
	if deviceProfileLoaded {
		s, e := deviceProfileCache, deviceProfileErr
		deviceProfileCacheMu.RUnlock()
		return s, e
	}
	deviceProfileCacheMu.RUnlock()

	deviceProfileCacheMu.Lock()
	defer deviceProfileCacheMu.Unlock()
	if deviceProfileLoaded {
		return deviceProfileCache, deviceProfileErr
	}

	store, err := loadDeviceProfilesFromFile(deviceProfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No data file — not an error; the engine falls back to the legacy
			// sampler. Cache the miss so we don't re-stat the path every call.
			deviceProfileLoaded = true
			return nil, nil
		}
		log.Printf("fpengine: device profile parse failed (%s): %v", deviceProfilePath, err)
		deviceProfileErr = err
		return nil, err
	}

	deviceProfileCache = store
	deviceProfileLoaded = true
	return store, nil
}

// ReloadDeviceProfiles clears the cache so the next call re-reads the file.
// Used after `bes update-fp` writes fresh data.
func ReloadDeviceProfiles() {
	deviceProfileCacheMu.Lock()
	deviceProfileCache = nil
	deviceProfileErr = nil
	deviceProfileLoaded = false
	deviceProfileCacheMu.Unlock()
}

func loadDeviceProfilesFromFile(path string) (*deviceProfileStore, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDeviceProfileData(raw)
}

func parseDeviceProfileData(raw []byte) (*deviceProfileStore, error) {
	var d deviceProfileData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse device profiles: %w", err)
	}
	return buildStore(d)
}

// DeviceProfileStoreStats returns a summary of the loaded distribution.
func DeviceProfileStoreStats() map[string]int {
	s, _ := loadDeviceProfileStore()
	if s == nil {
		return map[string]int{"user_agents": 0, "gpu_dims": 0, "screen_dims": 0, "fonts_pool": 0}
	}
	return map[string]int{
		"user_agents": s.len(),
		"gpu_dims":    len(s.data.VideoCard),
		"screen_dims": len(s.data.Screen),
		"fonts_pool":  len(s.data.Fonts.Pool),
	}
}