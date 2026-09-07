package fpengine

import (
	"strings"
	"testing"
)

// winUA / macUA / andUA are minimal, real-shaped Chrome UAs used to drive the
// in-memory joint-distribution tests.
const (
	winUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	macUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	andUA = "Mozilla/5.0 (Linux; Android 14; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Mobile Safari/537.36"
)

// testDeviceProfileData builds a tiny joint distribution where every dimension
// is conditioned on one of the three UAs with a single, OS-consistent value.
// It exercises the sampler's correlation logic without the full data file.
func testDeviceProfileData() deviceProfileData {
	return deviceProfileData{
		UserAgents: []string{winUA, macUA, andUA},
		UAPriors:   []float64{0.4, 0.3, 0.3},
		VideoCard: map[string][]dpGPU{
			winUA: {{Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Direct3D11 vs_5_0 ps_5_0, D3D11)", Vendor: "Google Inc. (NVIDIA)", P: 1}},
			macUA: {{Renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)", Vendor: "Google Inc. (Apple)", P: 1}},
			andUA: {{Renderer: "Adreno (TM) 650", Vendor: "Qualcomm", P: 1}},
		},
		Screen: map[string][]dpScreen{
			winUA: {{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1040, ColorDepth: 24, PixelDepth: 24, DevicePixelRatio: 1, OuterWidth: 1920, OuterHeight: 1080, P: 1}},
			macUA: {{Width: 1512, Height: 982, AvailWidth: 1512, AvailHeight: 949, ColorDepth: 30, PixelDepth: 30, DevicePixelRatio: 2, OuterWidth: 1512, OuterHeight: 949, P: 1}},
			andUA: {{Width: 393, Height: 873, AvailWidth: 393, AvailHeight: 873, ColorDepth: 24, PixelDepth: 24, DevicePixelRatio: 2.75, OuterWidth: 393, OuterHeight: 873, P: 1}},
		},
		HardwareConcurrency: map[string][]dpWeightedInt{
			winUA: {{V: 8, P: 1}}, macUA: {{V: 10, P: 1}}, andUA: {{V: 8, P: 1}},
		},
		DeviceMemory: map[string][]dpWeightedInt{
			winUA: {{V: 16, P: 1}}, macUA: {{V: 16, P: 1}}, andUA: {{V: 8, P: 1}},
		},
		Platform: map[string][]dpWeightedStr{
			winUA: {{V: "Win32", P: 1}}, macUA: {{V: "MacIntel", P: 1}}, andUA: {{V: "Linux armv8l", P: 1}},
		},
		Vendor: map[string][]dpWeightedStr{
			winUA: {{V: "Google Inc.", P: 1}}, macUA: {{V: "Google Inc.", P: 1}}, andUA: {{V: "Google Inc.", P: 1}},
		},
		MaxTouchPoints: map[string][]dpWeightedInt{
			winUA: {{V: 0, P: 1}}, macUA: {{V: 0, P: 1}}, andUA: {{V: 5, P: 1}},
		},
		Fonts: dpFonts{
			Pool: [][]string{
				{"Segoe UI", "Calibri"},
				{"Helvetica Neue", "SF Pro"},
				{"Roboto"},
			},
			Cond: map[string][]dpWeightedIdx{
				winUA: {{I: 0, P: 1}}, macUA: {{I: 1, P: 1}}, andUA: {{I: 2, P: 1}},
			},
		},
	}
}

func TestBuildStore_DropsUnsupportedUAs(t *testing.T) {
	d := testDeviceProfileData()
	// Add an Edge and an iOS UA that must be dropped (outside the engine's
	// supported browser/OS families).
	d.UserAgents = append(d.UserAgents,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1",
	)
	d.UAPriors = append(d.UAPriors, 0.05, 0.05)

	s, err := buildStore(d)
	if err != nil {
		t.Fatalf("buildStore: %v", err)
	}
	if got, want := s.len(), 3; got != want {
		t.Fatalf("kept %d UAs, want %d (Edge/iOS should be dropped)", got, want)
	}
}

func TestSampleDeviceProfile_Correlation(t *testing.T) {
	s, err := buildStore(testDeviceProfileData())
	if err != nil {
		t.Fatalf("buildStore: %v", err)
	}

	cases := []struct {
		name        string
		browser, os string
		wantOS      string
		wantPlatformCheck func(string) bool
		wantRendererSubstr string
		wantFont        string
		wantDPRMin      float64
	}{
		{
			name: "windows", browser: "chrome", os: "windows", wantOS: "windows",
			wantRendererSubstr: "Direct3D11", wantFont: "Segoe UI", wantDPRMin: 0,
		},
		{
			name: "macos", browser: "chrome", os: "macos", wantOS: "macos",
			wantRendererSubstr: "Metal", wantFont: "Helvetica Neue", wantDPRMin: 2,
		},
		{
			name: "android", browser: "chrome", os: "android", wantOS: "android",
			wantRendererSubstr: "Adreno", wantFont: "Roboto", wantDPRMin: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dpp, ok := s.sampleDeviceProfile(newSeededRNG(1), tc.browser, tc.os)
			if !ok {
				t.Fatalf("sampleDeviceProfile(%s,%s) = not ok", tc.browser, tc.os)
			}
			if dpp.OS != tc.wantOS {
				t.Errorf("OS = %q, want %q", dpp.OS, tc.wantOS)
			}
			if !strings.Contains(dpp.Renderer, tc.wantRendererSubstr) {
				t.Errorf("renderer %q does not contain %q (OS↔GPU correlation broken)", dpp.Renderer, tc.wantRendererSubstr)
			}
			if len(dpp.Fonts) == 0 || dpp.Fonts[0] != tc.wantFont {
				t.Errorf("fonts = %v, want first %q (OS↔font correlation broken)", dpp.Fonts, tc.wantFont)
			}
			if dpp.Screen.DevicePixelRatio < tc.wantDPRMin {
				t.Errorf("DPR = %v, want >= %v (OS↔screen correlation broken)", dpp.Screen.DevicePixelRatio, tc.wantDPRMin)
			}
			if dpp.HardwareConcurrency < minHardwareConcurrency || dpp.HardwareConcurrency > maxHardwareConcurrency {
				t.Errorf("hardwareConcurrency = %d out of range", dpp.HardwareConcurrency)
			}
			if dpp.Browser != tc.browser {
				t.Errorf("browser = %q, want %q", dpp.Browser, tc.browser)
			}
		})
	}
}

func TestSampleDeviceProfile_ConstraintMiss(t *testing.T) {
	s, err := buildStore(testDeviceProfileData())
	if err != nil {
		t.Fatalf("buildStore: %v", err)
	}
	// No Firefox UA exists in the fixture, so a firefox request must miss
	// (and the engine would fall back to the legacy sampler).
	if _, ok := s.sampleDeviceProfile(newSeededRNG(1), "firefox", ""); ok {
		t.Fatalf("expected miss for firefox (no firefox UA in fixture)")
	}
}

func TestSampleWeightedIndex_Deterministic(t *testing.T) {
	rng := newSeededRNG(42)
	// A single candidate always wins.
	if got := sampleWeightedIndex(rng, []float64{1.0}); got != 0 {
		t.Fatalf("single-weight sample = %d, want 0", got)
	}
	// Zero-sum weights fall back to uniform without panicking.
	if got := sampleWeightedIndex(rng, []float64{0, 0, 0}); got < 0 || got > 2 {
		t.Fatalf("zero-sum sample out of range: %d", got)
	}
	// Determinism: same seed → same index.
	a := sampleWeightedIndex(newSeededRNG(7), []float64{0.1, 0.2, 0.7})
	b := sampleWeightedIndex(newSeededRNG(7), []float64{0.1, 0.2, 0.7})
	if a != b {
		t.Fatalf("same seed produced different indices: %d vs %d", a, b)
	}
}

func TestClassifyUA(t *testing.T) {
	cases := []struct {
		ua      string
		browser string
		os      string
	}{
		{winUA, "chrome", "windows"},
		{macUA, "chrome", "macos"},
		{andUA, "chrome", "android"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0", "firefox", "linux"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.4 Safari/605.1.15", "safari", "macos"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 Version/26.3 Mobile/15E148 Safari/604.1", "safari", "ios"},
	}
	for _, c := range cases {
		b, o := classifyUA(c.ua)
		if b != c.browser || o != c.os {
			t.Errorf("classifyUA(%q) = (%q,%q), want (%q,%q)", c.ua, b, o, c.browser, c.os)
		}
	}
}

func TestBrowserVersion(t *testing.T) {
	if v, m := browserVersion(winUA, "chrome"); v != "147" || m != 147 {
		t.Errorf("browserVersion(winUA) = (%q,%d), want (\"147\",147)", v, m)
	}
	firefox := "Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0"
	if v, m := browserVersion(firefox, "firefox"); v != "150" || m != 150 {
		t.Errorf("browserVersion(firefox) = (%q,%d), want (\"150\",150)", v, m)
	}
}