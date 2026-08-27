package fpengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zninggo/bes/pkg/api"
)

// KnowledgeBase holds the browser × OS × GPU × screen × timezone matrix
// that the fingerprint engine samples from.
type KnowledgeBase struct {
	Browsers  []BrowserEntry
	OSes      []OSEntry
	GPUs      map[string][]GPUEntry // keyed by OS name
	Screens   map[string][]ScreenEntry
	Timezones []TimezoneEntry
	CanvasHashes  map[string]string // key: "chrome131_windows_nvidia"
	AudioHashes   map[string]string
	WindowProps   map[string][]api.WindowProps
}

type BrowserEntry struct {
	Name       string
	Version    string
	MajorVer   int
	UATemplate string // e.g. "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36"
}

type OSEntry struct {
	Name         string
	Version      string
	Platform     string
	UASegment    string
	DefaultFonts []string
}

type GPUEntry struct {
	Vendor   string
	Renderer string
}

type ScreenEntry struct {
	Width       int
	Height      int
	AvailWidth  int
	AvailHeight int
	ColorDepth  int
}

type TimezoneEntry struct {
	Name      string // "Asia/Shanghai"
	Languages []string
}

// DefaultKnowledgeBase returns the built-in knowledge base with real-world
// browser/OS/GPU/screen combinations.
func DefaultKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		Browsers: []BrowserEntry{
			// Chrome stable as of 2026-08 (Chrome ~150-152)
			{
				Name:       "chrome",
				Version:    "152",
				MajorVer:   152,
				UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			},
			{
				Name:       "chrome",
				Version:    "151",
				MajorVer:   151,
				UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			},
			{
				Name:       "chrome",
				Version:    "150",
				MajorVer:   150,
				UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			},
			{
				Name:       "chrome",
				Version:    "149",
				MajorVer:   149,
				UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			},
			{
				Name:       "chrome",
				Version:    "148",
				MajorVer:   148,
				UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36",
			},
		},
		OSes: []OSEntry{
			{
				Name:      "windows",
				Version:   "11",
				Platform:  "Win32",
				UASegment: "Windows NT 10.0; Win64; x64",
				DefaultFonts: []string{
					"Arial", "Arial Black", "Arial Narrow", "Calibri", "Cambria",
					"Cambria Math", "Comic Sans MS", "Consolas", "Courier New",
					"Ebrima", "Franklin Gothic Medium", "Gabriola", "Gadugi",
					"Georgia", "Impact", "Javanese Text", "Leelawadee UI",
					"Lucida Console", "Lucida Sans Unicode", "MS Gothic",
					"MS PGothic", "MS Sans Serif", "MS Serif", "MS UI Gothic",
					"MV Boli", "Malgun Gothic", "Microsoft Himalaya",
					"Microsoft JhengHei", "Microsoft New Tai Lue",
					"Microsoft PhagsPa", "Microsoft Sans Serif",
					"Microsoft Tai Le", "Microsoft YaHei", "Microsoft Yi Baiti",
					"MingLiU-ExtB", "Mongolian Baiti", "Myanmar Text",
					"Nirmala UI", "Palatino Linotype", "Segoe Print",
					"Segoe Script", "Segoe UI", "Segoe UI Emoji",
					"Segoe UI Historic", "Segoe UI Symbol", "SimSun",
					"Sitka Small", "Sylfaen", "Symbol", "Tahoma",
					"Times New Roman", "Trebuchet MS", "Verdana",
					"Webdings", "Wingdings", "Yu Gothic",
				},
			},
			{
				Name:      "macos",
				Version:   "14",
				Platform:  "MacIntel",
				UASegment: "Macintosh; Intel Mac OS X 10_15_7",
				DefaultFonts: []string{
					"Arial", "Arial Black", "Arial Narrow", "Arial Unicode MS",
					"Avenir", "Avenir Next", "Avenir Next Condensed",
					"Baskerville", "Big Caslon", "Bodoni 72", "Brush Script MT",
					"Chalkboard", "Chalkduster", "Charcoal", "Cochin",
					"Comic Sans MS", "Copperplate", "Courier", "Courier New",
					"Didot", "Futura", "Geneva", "Georgia",
					"Gill Sans", "Helvetica", "Helvetica Neue",
					"Herculanum", "Hoefler Text", "Impact", "Lucida Grande",
					"Luminari", "Marker Felt", "Menlo", "Monaco",
					"Noteworthy", "Optima", "Palatino", "Papyrus",
					"Phosphate", "Rockwell", "SF Pro", "Savoye LET",
					"SignPainter", "Skia", "Snell Roundhand", "Tahoma",
					"Times", "Times New Roman", "Trebuchet MS", "Verdana",
					"Zapfino",
				},
			},
			{
				Name:      "linux",
				Version:   "",
				Platform:  "Linux x86_64",
				UASegment: "X11; Linux x86_64",
				DefaultFonts: []string{
					"DejaVu Sans", "DejaVu Sans Mono", "DejaVu Serif",
					"Liberation Sans", "Liberation Mono", "Liberation Serif",
					"Arial", "Courier New", "Georgia", "Times New Roman",
					"Trebuchet MS", "Verdana", "Noto Sans", "Noto Sans Mono",
					"Noto Serif", "Ubuntu", "Ubuntu Mono",
				},
			},
		},
		GPUs: map[string][]GPUEntry{
			"windows": {
				{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Direct3D11 vs_5_0 ps_5_0)"},
				{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0)"},
				{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 Direct3D11 vs_5_0 ps_5_0)"},
				{Vendor: "Google Inc. (Intel)", Renderer: "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)"},
				{Vendor: "Google Inc. (Intel)", Renderer: "ANGLE (Intel, Intel(R) Iris(R) Xe Graphics Direct3D11 vs_5_0 ps_5_0)"},
				{Vendor: "Google Inc. (AMD)", Renderer: "ANGLE (AMD, AMD Radeon RX 6700 XT Direct3D11 vs_5_0 ps_5_0)"},
			},
			"macos": {
				{Vendor: "Google Inc. (Apple)", Renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)"},
				{Vendor: "Google Inc. (Apple)", Renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M2, Unspecified Version)"},
				{Vendor: "Google Inc. (Apple)", Renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M3, Unspecified Version)"},
			},
			"linux": {
				{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 OpenGL 4.5)"},
				{Vendor: "Google Inc. (Intel)", Renderer: "ANGLE (Intel, Intel(R) UHD Graphics OpenGL)"},
				{Vendor: "Mesa/X.org", Renderer: "llvmpipe (LLVM 15.0.7, 256 bits)"},
			},
		},
		Screens: map[string][]ScreenEntry{
			"windows": {
				{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1040, ColorDepth: 24},
				{Width: 2560, Height: 1440, AvailWidth: 2560, AvailHeight: 1400, ColorDepth: 24},
				{Width: 1366, Height: 768, AvailWidth: 1366, AvailHeight: 728, ColorDepth: 24},
				{Width: 3840, Height: 2160, AvailWidth: 3840, AvailHeight: 2160, ColorDepth: 24},
			},
			"macos": {
				{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1055, ColorDepth: 30},
				{Width: 2560, Height: 1600, AvailWidth: 2560, AvailHeight: 1575, ColorDepth: 30},
				{Width: 3456, Height: 2234, AvailWidth: 3456, AvailHeight: 2194, ColorDepth: 30},
			},
			"linux": {
				{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1080, ColorDepth: 24},
				{Width: 2560, Height: 1440, AvailWidth: 2560, AvailHeight: 1440, ColorDepth: 24},
			},
		},
		Timezones: []TimezoneEntry{
			{Name: "Asia/Shanghai", Languages: []string{"zh-CN", "zh"}},
			{Name: "Asia/Tokyo", Languages: []string{"ja", "en"}},
			{Name: "America/New_York", Languages: []string{"en-US", "en"}},
			{Name: "Europe/London", Languages: []string{"en-GB", "en"}},
			{Name: "Asia/Seoul", Languages: []string{"ko-KR", "ko", "en"}},
			{Name: "Asia/Singapore", Languages: []string{"en-SG", "en", "zh-SG", "zh"}},
		},
		// Canvas/Audio hashes: real values need pre-collection from real machines
		// (see experiments/collect-hashes.js). Until then, LookupCanvasHash /
		// LookupAudioHash generate deterministic synthetic hashes so the
		// fingerprint is never left with a placeholder.
		CanvasHashes: map[string]string{},
		AudioHashes:   map[string]string{},
		WindowProps: map[string][]api.WindowProps{
			"windows": {
				{InnerWidth: 1920, InnerHeight: 969, OuterWidth: 1920, OuterHeight: 1040, DevicePixelRatio: 1, ScreenX: 0, ScreenY: 0},
				{InnerWidth: 1536, InnerHeight: 768, OuterWidth: 1536, OuterHeight: 816, DevicePixelRatio: 1.25, ScreenX: 0, ScreenY: 0},
			},
			"macos": {
				{InnerWidth: 1920, InnerHeight: 985, OuterWidth: 1920, OuterHeight: 1055, DevicePixelRatio: 2, ScreenX: 0, ScreenY: 25},
			},
			"linux": {
				{InnerWidth: 1920, InnerHeight: 1080, OuterWidth: 1920, OuterHeight: 1080, DevicePixelRatio: 1, ScreenX: 0, ScreenY: 0},
			},
		},
	}
}

// --- Sampling methods ---

func (kb *KnowledgeBase) SampleBrowser(rng *seededRNG, preferred string) api.BrowserProfile {
	entries := kb.Browsers
	if preferred != "" {
		var filtered []BrowserEntry
		for _, b := range entries {
			if b.Name == preferred {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) > 0 {
			entries = filtered
		}
	}
	e := entries[rng.Intn(len(entries))]
	return api.BrowserProfile{
		Name:       e.Name,
		Version:    e.Version,
		MajorVer:   e.MajorVer,
		UATemplate: e.UATemplate,
	}
}

func (kb *KnowledgeBase) SampleOS(rng *seededRNG, preferred string) api.OSProfile {
	entries := kb.OSes
	if preferred != "" {
		var filtered []OSEntry
		for _, o := range entries {
			if o.Name == preferred {
				filtered = append(filtered, o)
			}
		}
		if len(filtered) > 0 {
			entries = filtered
		}
	}
	e := entries[rng.Intn(len(entries))]
	return api.OSProfile{
		Name:         e.Name,
		Version:      e.Version,
		Platform:     e.Platform,
		UASegment:    e.UASegment,
		DefaultFonts: e.DefaultFonts,
	}
}

func (kb *KnowledgeBase) SampleGPU(rng *seededRNG, osName string) api.GPUProfile {
	gpus, ok := kb.GPUs[osName]
	if !ok || len(gpus) == 0 {
		gpus = kb.GPUs["windows"] // fallback
	}
	e := gpus[rng.Intn(len(gpus))]
	return api.GPUProfile{Vendor: e.Vendor, Renderer: e.Renderer}
}

func (kb *KnowledgeBase) SampleScreen(rng *seededRNG, osName string) map[string]any {
	screens, ok := kb.Screens[osName]
	if !ok || len(screens) == 0 {
		screens = kb.Screens["windows"]
	}
	e := screens[rng.Intn(len(screens))]
	return map[string]any{
		"width":       e.Width,
		"height":      e.Height,
		"availWidth":  e.AvailWidth,
		"availHeight": e.AvailHeight,
		"colorDepth":  e.ColorDepth,
		"pixelDepth":  e.ColorDepth,
		"orientation": map[string]any{"type": "landscape-primary", "angle": 0},
		"availLeft":   0,
		"availTop":    0,
	}
}

func (kb *KnowledgeBase) SampleTimezone(rng *seededRNG) (string, []string) {
	e := kb.Timezones[rng.Intn(len(kb.Timezones))]
	return e.Name, e.Languages
}

func (kb *KnowledgeBase) LookupCanvasHash(majorVer int, osName, gpuVendor string) api.CanvasFP {
	key := canvasHashKey(majorVer, osName, gpuVendor)
	hash := kb.CanvasHashes[key]
	if hash == "" {
		hash = syntheticHash("canvas", majorVer, osName, gpuVendor)
	}
	return api.CanvasFP{
		ToDataURLHash: hash,
		MeasureText: map[string]float64{
			"measureText_width": float64(majorVer) * 0.1,
		},
	}
}

func (kb *KnowledgeBase) LookupAudioHash(majorVer int, osName string) api.AudioFP {
	key := audioHashKey(majorVer, osName)
	hash := kb.AudioHashes[key]
	if hash == "" {
		hash = syntheticHash("audio", majorVer, osName, "")
	}
	return api.AudioFP{Hash: hash}
}

// syntheticHash generates a deterministic hash from the given components.
// This is NOT a real machine hash — it ensures the fingerprint has a
// consistent, non-placeholder value until real hashes are collected.
// Replace CanvasHashes/AudioHashes entries with real values from
// experiments/collect-hashes.js to use actual machine fingerprints.
func syntheticHash(prefix string, majorVer int, osName, gpuVendor string) string {
	data := fmt.Sprintf("%s:%d:%s:%s", prefix, majorVer, osName, gpuVendor)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:8]) // 16 hex chars
}

func (kb *KnowledgeBase) SampleWindowProps(rng *seededRNG, screen map[string]any) api.WindowProps {
	osName := "windows"
	screens, ok := kb.WindowProps[osName]
	if !ok || len(screens) == 0 {
		screens = kb.WindowProps["windows"]
	}
	return screens[rng.Intn(len(screens))]
}

func canvasHashKey(majorVer int, osName, gpuVendor string) string {
	gpuShort := "nvidia"
	if contains(gpuVendor, "Intel") {
		gpuShort = "intel"
	} else if contains(gpuVendor, "AMD") {
		gpuShort = "amd"
	} else if contains(gpuVendor, "Apple") {
		gpuShort = "apple"
	}
	return fmt.Sprintf("chrome%d_%s_%s", majorVer, osName, gpuShort)
}

func audioHashKey(majorVer int, osName string) string {
	return fmt.Sprintf("chrome%d_%s", majorVer, osName)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
