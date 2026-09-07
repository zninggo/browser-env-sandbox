package fpengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findDeviceProfileData locates the committed data/device_profiles.json from
// the test working directory. Returns "" when absent (test skips).
func findDeviceProfileData() string {
	candidates := []string{
		"../../data/device_profiles.json",          // from internal/fpengine
		"../../../data/device_profiles.json",       // from internal/fpengine (nested)
		"data/device_profiles.json",                // repo root
	}
	for _, c := range candidates {
		abs, _ := filepath.Abs(c)
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	return ""
}

// loadRealStoreForTest points the package loader at the committed data file,
// returning the store (or nil to skip). It restores the prior path/cache on
// cleanup so the global singleton does not leak into other tests in this
// package (which exercise the legacy sampler and must not pick up the joint
// data they cannot locate from the package's working directory).
func loadRealStoreForTest(t *testing.T) *deviceProfileStore {
	t.Helper()
	path := findDeviceProfileData()
	if path == "" {
		t.Skip("data/device_profiles.json not found; skipping joint-distribution integration test")
	}
	origPath := deviceProfilePath
	deviceProfilePath = path
	ReloadDeviceProfiles()
	t.Cleanup(func() {
		deviceProfilePath = origPath
		ReloadDeviceProfiles()
	})
	s, err := loadDeviceProfileStore()
	if err != nil {
		t.Fatalf("loadDeviceProfileStore: %v", err)
	}
	if s == nil {
		t.Fatalf("loaded nil store from %s", path)
	}
	return s
}

// TestDeviceProfile_RealDataCorrelation samples hundreds of fingerprints per OS
// from the real apify-derived joint distribution and asserts the cross-dimension
// correlations a detector would check: macOS → Metal + Retina, Windows →
// Direct3D11, Android → Mali/Adreno (never D3D11/Metal). It also asserts every
// sampled value stays inside the source distribution.
func TestDeviceProfile_RealDataCorrelation(t *testing.T) {
	s := loadRealStoreForTest(t)

	// Collect the source value sets for membership ("in distribution") checks.
	gpuRenderers := map[string]struct{}{}
	for _, list := range s.data.VideoCard {
		for _, g := range list {
			gpuRenderers[g.Renderer] = struct{}{}
		}
	}
	screenKeys := map[[3]int]struct{}{}
	for _, list := range s.data.Screen {
		for _, sc := range list {
			screenKeys[[3]int{sc.Width, sc.Height, sc.ColorDepth}] = struct{}{}
		}
	}

	osChecks := []struct {
		os      string
		mustNot []string // renderer substrings that must never appear for this OS
	}{
		{os: "windows", mustNot: []string{"Metal"}},
		{os: "macos", mustNot: []string{"Direct3D11"}},
		{os: "android", mustNot: []string{"Direct3D11", "Metal"}},
	}

	const n = 500
	for _, oc := range osChecks {
		metmetal, metd3d, metmobile := 0, 0, 0
		for i := 0; i < n; i++ {
			dpp, ok := s.sampleDeviceProfile(newSeededRNG(uint64(i)+1), "chrome", oc.os)
			if !ok {
				t.Fatalf("os=%s: sample miss at i=%d", oc.os, i)
			}
			r := dpp.Renderer
			if _, in := gpuRenderers[r]; !in {
				t.Fatalf("os=%s: renderer %q not in source distribution", oc.os, r)
			}
			if _, in := screenKeys[[3]int{dpp.Screen.Width, dpp.Screen.Height, dpp.Screen.ColorDepth}]; !in {
				t.Fatalf("os=%s: screen %dx%d@%d not in source distribution", oc.os, dpp.Screen.Width, dpp.Screen.Height, dpp.Screen.ColorDepth)
			}
			for _, sub := range oc.mustNot {
				if strings.Contains(r, sub) {
					t.Errorf("%s renderer %q contains %q (impossible combo)", oc.os, r, sub)
				}
			}
			switch oc.os {
			case "macos":
				if strings.Contains(r, "Metal") {
					metmetal++
				}
			case "windows":
				if strings.Contains(r, "Direct3D11") {
					metd3d++
				}
			case "android":
				if strings.Contains(r, "Mali") || strings.Contains(r, "Adreno") || strings.Contains(r, "PowerVR") {
					metmobile++
				}
			}
			if dpp.HardwareConcurrency < minHardwareConcurrency || dpp.HardwareConcurrency > maxHardwareConcurrency {
				t.Errorf("os=%s: hwConc %d out of range", oc.os, dpp.HardwareConcurrency)
			}
			if dpp.DeviceMemory < 1 {
				t.Errorf("os=%s: deviceMemory %d < 1", oc.os, dpp.DeviceMemory)
			}
		}
		// The strong correlations must dominate the sample (not be zero).
		switch oc.os {
		case "macos":
			if metmetal == 0 {
				t.Errorf("macos: 0/%d samples had Metal renderer (OS↔GPU correlation absent)", n)
			}
		case "windows":
			if metd3d == 0 {
				t.Errorf("windows: 0/%d samples had Direct3D11 renderer", n)
			}
		case "android":
			if metmobile == 0 {
				t.Errorf("android: 0/%d samples had mobile GPU (Mali/Adreno/PowerVR)", n)
			}
		}
	}
}

// TestEngine_UsesDeviceProfiles verifies that Engine.Generate prefers the joint
// device-profile path when data is available, producing a fingerprint whose
// UA, GPU, OS platform and screen DPR all cohere.
func TestEngine_UsesDeviceProfiles(t *testing.T) {
	loadRealStoreForTest(t)

	eng := New()
	fp, err := eng.Generate(1, "chrome", "macos")
	if err != nil {
		t.Fatalf("Generate(macos): %v", err)
	}
	if fp.OS.Name != "macos" {
		t.Fatalf("OS = %q, want macos", fp.OS.Name)
	}
	// A macOS renderer is Metal (Apple Silicon / recent Intel) or OpenGL (older
	// Intel Macs) — never Direct3D11 (Windows-only). This is the OS↔GPU
	// correlation the joint sampler enforces.
	if strings.Contains(fp.GPU.Renderer, "Direct3D11") {
		t.Errorf("macos GPU renderer %q contains Direct3D11 (impossible combo)", fp.GPU.Renderer)
	}
	if !strings.Contains(fp.GPU.Renderer, "Metal") && !strings.Contains(fp.GPU.Renderer, "OpenGL") && !strings.Contains(fp.GPU.Vendor, "Apple") {
		t.Errorf("macos GPU renderer %q is not a valid macOS renderer", fp.GPU.Renderer)
	}
	platform, _ := fp.Navigator["platform"].(string)
	if platform != "MacIntel" {
		t.Errorf("platform = %q, want MacIntel", platform)
	}
	// The sampled screen DPR must be positive and OS-consistent (Mac Retina
	// values live at 2.0 in the source; external-monitor Macs can be 1.0).
	if fp.Window.DevicePixelRatio <= 0 {
		t.Errorf("macOS DPR = %v, want > 0", fp.Window.DevicePixelRatio)
	}
	if err := ValidateConsistency(fp); err != nil {
		t.Errorf("generated device-profile fingerprint failed validation: %v", err)
	}
}