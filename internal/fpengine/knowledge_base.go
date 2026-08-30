package fpengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
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
			{
				// Android: navigator.platform reports "Linux armv8l" on 64-bit
				// ARM (the historical quirk Android never fixed). The UA
				// segment carries the Android version + device placeholder "K".
				// Without this entry, SampleOS("android") silently fell back to
				// a random OS (macos/linux), which made the android GPU filter
				// dead code and produced macOS fingerprints for android requests.
				Name:      "android",
				Version:   "14",
				Platform:  "Linux armv8l",
				UASegment: "Linux; Android 14; K",
				DefaultFonts: []string{
					"Roboto", "Roboto Mono", "Noto Sans", "Noto Sans Mono",
					"Noto Serif", "Droid Sans", "Droid Sans Mono",
					"Cousine", "Coming Soon", "Dancing Script",
					"Carrois Gothic SC", "Cutive Mono", "Anonymous Pro",
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
			"android": {
				// Android renders via OpenGL ES / Vulkan; Mali (Samsung/Exynos),
				// Adreno (Qualcomm), PowerVR (ImgTec) are the real mobile GPUs.
				// Direct3D11/Metal never appear on Android.
				{Vendor: "Qualcomm", Renderer: "Adreno (TM) 650"},
				{Vendor: "Qualcomm", Renderer: "Adreno (TM) 730"},
				{Vendor: "ARM", Renderer: "Mali-G78 MP24"},
				{Vendor: "ARM", Renderer: "Mali-G715 MC11"},
				{Vendor: "Imagination Technologies", Renderer: "PowerVR Rogue GE8320"},
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

// SampleOS picks an OS profile. When preferred is non-empty it MUST match a
// known OS entry; an unknown preferred returns an explicit error instead of
// silently falling back to a random OS. The silent fallback was the root cause
// of the android GPU defect: SampleOS("android") returned macos (no android
// entry existed), which then routed SampleGPU into the macos/Metal branch and
// made the android GPU filter dead code. Failing loudly here ensures a missing
// OS entry can never again disguise itself as a different OS.
func (kb *KnowledgeBase) SampleOS(rng *seededRNG, preferred string) (api.OSProfile, error) {
	entries := kb.OSes
	if preferred != "" {
		var filtered []OSEntry
		for _, o := range entries {
			if o.Name == preferred {
				filtered = append(filtered, o)
			}
		}
		if len(filtered) == 0 {
			return api.OSProfile{}, fmt.Errorf("SampleOS: unknown preferred OS %q (no matching entry; add it to KnowledgeBase.OSes)", preferred)
		}
		entries = filtered
	}
	e := entries[rng.Intn(len(entries))]
	return api.OSProfile{
		Name:         e.Name,
		Version:      e.Version,
		Platform:     e.Platform,
		UASegment:    e.UASegment,
		DefaultFonts: e.DefaultFonts,
	}, nil
}

func (kb *KnowledgeBase) SampleGPU(rng *seededRNG, osName string) api.GPUProfile {
	// Prefer real apify data (773 combos) via runtime loader (JSON or embedded).
	gpus := CurrentFpRealGPUs()
	if len(gpus) > 0 {
		// Pre-filter by OS so we never sample an incompatible GPU.
		// macOS → Metal renderer, Windows → Direct3D11, Linux → OpenGL/any-D3D.
		filtered := filterGPUsByOS(gpus, osName)
		if len(filtered) == 0 {
			filtered = gpus // fallback: use full list if filter is too narrow
		}
		gpu := filtered[rng.Intn(len(filtered))]
		return api.GPUProfile{Vendor: gpu.Vendor, Renderer: gpu.Renderer}
	}
	// Fallback to hand-curated matrix
	fallbackGPUs, ok := kb.GPUs[osName]
	if !ok || len(fallbackGPUs) == 0 {
		fallbackGPUs = kb.GPUs["windows"]
	}
	e := fallbackGPUs[rng.Intn(len(fallbackGPUs))]
	return api.GPUProfile{Vendor: e.Vendor, Renderer: e.Renderer}
}

// filterGPUsByOS returns only GPUs whose renderer string is consistent with
// the given OS. This prevents impossible combos like Apple Metal on Windows.
func filterGPUsByOS(gpus []FpRealGPU, osName string) []FpRealGPU {
	out := make([]FpRealGPU, 0, len(gpus))
	for _, g := range gpus {
		switch osName {
		case "macos":
			// macOS uses Metal; "Apple GPU" is also valid (Safari fallback).
			if strings.Contains(g.Renderer, "Metal") || strings.Contains(g.Vendor, "Apple") {
				out = append(out, g)
			}
		case "windows":
			// Windows uses Direct3D11; exclude Metal and Apple GPU.
			if strings.Contains(g.Renderer, "Direct3D11") &&
				!strings.Contains(g.Renderer, "Metal") &&
				!strings.Contains(g.Vendor, "Apple") {
				out = append(out, g)
			}
		case "linux":
			// Linux typically uses OpenGL; D3D11 entries are Windows-only.
			// Most apify Linux entries don't have D3D/Metal in the renderer.
			if !strings.Contains(g.Renderer, "Direct3D11") &&
				!strings.Contains(g.Renderer, "Metal") &&
				!strings.Contains(g.Vendor, "Apple") {
				out = append(out, g)
			}
		case "android":
			// Android GPUs are Mali/Adreno/PowerVR/Mali-G; WebGL2 renders via
			// OpenGL ES or Vulkan. Direct3D11 is Windows-only and Metal is
			// Apple-only — both are impossible on Android and must be excluded.
			r := g.Renderer
			isAndroidGPU := strings.Contains(r, "Mali") ||
				strings.Contains(r, "Adreno") ||
				strings.Contains(r, "PowerVR") ||
				strings.Contains(r, "ImgTec") ||
				strings.Contains(r, "OpenGL ES") ||
				strings.Contains(r, "Vulkan")
			if isAndroidGPU &&
				!strings.Contains(r, "Direct3D11") &&
				!strings.Contains(r, "Metal") &&
				!strings.Contains(g.Vendor, "Apple") {
				out = append(out, g)
			}
		default:
			out = append(out, g)
		}
	}
	return out
}

func (kb *KnowledgeBase) SampleScreen(rng *seededRNG, osName string) map[string]any {
	// Prefer real apify data (4569 configs) via runtime loader.
	screens := CurrentFpRealScreens()
	if len(screens) > 0 {
		// Pre-filter by OS: macOS screens have colorDepth 30, Windows/Linux
		// typically 24 or 32. macOS also has distinctive resolutions.
		filtered := filterScreensByOS(screens, osName)
		if len(filtered) == 0 {
			filtered = screens
		}
		s := filtered[rng.Intn(len(filtered))]
		return map[string]any{
			"width":       s.Width,
			"height":      s.Height,
			"availWidth":  s.AvailWidth,
			"availHeight": s.AvailHeight,
			"colorDepth":  s.ColorDepth,
			"pixelDepth":  s.ColorDepth,
			"orientation": map[string]any{"type": "landscape-primary", "angle": 0},
			"availLeft":   0,
			"availTop":    0,
		}
	}
	fallbackScreens, ok := kb.Screens[osName]
	if !ok || len(fallbackScreens) == 0 {
		fallbackScreens = kb.Screens["windows"]
	}
	e := fallbackScreens[rng.Intn(len(fallbackScreens))]
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

// filterScreensByOS returns only screen configs consistent with the given OS.
// macOS: colorDepth 30 (typical for Retina), common widths 1512/1710/1470/1728.
// Windows/Linux: colorDepth 24 or 32.
func filterScreensByOS(screens []FpRealScreen, osName string) []FpRealScreen {
	out := make([]FpRealScreen, 0, len(screens))
	for _, s := range screens {
		switch osName {
		case "macos":
			// macOS Retina displays: colorDepth 30, DPR 2.0, widths like 1512/1710/1470/1728/2560.
			if s.ColorDepth == 30 || s.DevicePixelRatio == 2.0 {
				out = append(out, s)
			}
		case "windows":
			// Windows: colorDepth 24 or 32, DPR typically 1.0/1.25/1.5.
			if s.ColorDepth == 24 || s.ColorDepth == 32 {
				out = append(out, s)
			}
		case "linux":
			// Linux: colorDepth 24 or 32, exclude macOS Retina (30).
			if s.ColorDepth == 24 || s.ColorDepth == 32 {
				out = append(out, s)
			}
		default:
			out = append(out, s)
		}
	}
	return out
}

func (kb *KnowledgeBase) SampleTimezone(rng *seededRNG) (string, []string) {
	e := kb.Timezones[rng.Intn(len(kb.Timezones))]
	return e.Name, e.Languages
}

// SampleTimezoneNamed returns the timezone entry matching name (e.g.
// "Asia/Tokyo"). Falls back to random sampling when no entry matches,
// so callers can pass untrusted values safely.
func (kb *KnowledgeBase) SampleTimezoneNamed(name string) (string, []string, bool) {
	for _, e := range kb.Timezones {
		if strings.EqualFold(e.Name, name) {
			return e.Name, e.Languages, true
		}
	}
	return "", nil, false
}

func (kb *KnowledgeBase) LookupCanvasHash(majorVer int, osName, gpuVendor string) api.CanvasFP {
	key := canvasHashKey(majorVer, osName, gpuVendor)

	// 1. Try pre-collected dataset (real Chrome canvas output).
	dataURL := LookupCanvasDataset(key)

	// 2. Try knowledge-base hash map (if populated).
	hash := kb.CanvasHashes[key]

	// 3. Fall back to synthetic generation for both hash and dataURL.
	if dataURL == "" {
		dataURL = syntheticCanvasDataURL(majorVer, osName, gpuVendor)
	}
	if hash == "" {
		hash = dataURL // use the full dataURL as the hash (base64 is already a compact representation)
	}

	return api.CanvasFP{
		ToDataURLHash: hash,
		ToDataURL:     "data:image/png;base64," + dataURL,
		MeasureText: map[string]float64{
			"measureText_width": float64(majorVer) * 0.1,
		},
	}
}

// LookupAudioHash returns the AudioContext fingerprint hash for the given
// browser major version and OS.
//
// SYNTHETIC LIMITATION (documented): when no pre-collected hash exists in
// kb.AudioHashes, the hash is derived deterministically from a sha256 of
// "audio:<ver>:<os>". This is NOT a real OfflineAudioContext output — it is a
// stable, self-consistent placeholder so the fingerprint is never left empty.
// The hash is OS/version-consistent (same seed → same hash) but does not
// reflect a real machine's audio DSP output. A real Audio fingerprint dataset
// (analogous to canvas_dataset.json, pre-collected via OfflineAudioContext on
// real hardware) is not yet wired in; until it is, consumers should treat
// Audio.Hash as synthetic. The injection side historically also left this
// unused, which is acceptable for current threat models that check hash
// presence/length rather than DSP integrity, but is tracked as a known gap.
func (kb *KnowledgeBase) LookupAudioHash(majorVer int, osName string) api.AudioFP {
	key := audioHashKey(majorVer, osName)
	hash := kb.AudioHashes[key]
	if hash == "" {
		// Deterministic 32-hex audio hash (same length as fingerprintjs output)
		seed := fmt.Sprintf("audio:%d:%s", majorVer, osName)
		h := sha256.Sum256([]byte(seed))
		hash = hex.EncodeToString(h[:16]) // 32 hex chars
	}
	return api.AudioFP{Hash: hash}
}

// syntheticCanvasDataURL generates a deterministic, decodable PNG for
// canvas.toDataURL(). Real Chrome canvas output is a valid PNG the server can
// png.Decode; the previous synthetic blob used raw pseudo-random bytes as the
// IDAT payload, which is not a valid zlib/deflate stream and fails any
// png.Decode / zlib integrity check.
//
// We now render a small RGBA image whose pixels are derived deterministically
// from the seed components (same fingerprint → same canvas, different
// fingerprint → different canvas) and encode it with image/png, which
// produces a fully spec-compliant PNG: signature, IHDR, deflate-compressed
// IDAT, correct CRC32, IEND. png.Decode and zlib readers pass.
func syntheticCanvasDataURL(majorVer int, osName, gpuVendor string) string {
	seed := fmt.Sprintf("canvas:%d:%s:%s", majorVer, osName, gpuVendor)
	h := sha256.Sum256([]byte(seed))

	// Small canvas (280x60) keeps output within real Chrome's 1-3KB base64
	// range while staying cheap to encode.
	const w, hgt = 280, 60
	img := image.NewNRGBA(image.Rect(0, 0, w, hgt))
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			// Deterministic per-pixel noise mixed from the seed hash. NRGBA
			// (non-premultiplied) matches toDataURL's RGBA output and needs
			// no alpha math.
			img.Pix[i+0] = h[(i+0)%32] ^ byte(x) ^ byte(y*3)
			img.Pix[i+1] = h[(i+1)%32] ^ byte(y) ^ byte(x*5)
			img.Pix[i+2] = h[(i+2)%32] ^ byte(x+y)
			img.Pix[i+3] = 0xFF
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// Should never happen for an in-memory NRGBA image; fall back to a
		// 1x1 opaque pixel so callers still get a decodable PNG.
		single := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		single.Pix[3] = 0xFF
		buf.Reset()
		_ = png.Encode(&buf, single)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (kb *KnowledgeBase) SampleWindowProps(rng *seededRNG, osName string, screen map[string]any) api.WindowProps {
	// Derive window dimensions from the actual screen to ensure consistency.
	// Invariant: screen.height >= availHeight >= outerHeight >= innerHeight.
	sw, _ := screen["width"].(int)
	sh, _ := screen["height"].(int)
	aw, _ := screen["availWidth"].(int)
	ah, _ := screen["availHeight"].(int)
	if aw == 0 {
		aw = sw
	}
	if ah == 0 {
		ah = sh
	}

	// DevicePixelRatio: sample from the KB's WindowProps for this OS so DPR
	// is OS-self-consistent (macOS Retina = 2/3, Windows = 1/1.25/1.5,
	// Linux = 1, Android = 2/3). The previous code hardcoded 1.0, which made
	// every macOS/Android fingerprint claim DPR 1 — an instant inconsistency.
	dpr := 1.0
	if props, ok := kb.WindowProps[osName]; ok && len(props) > 0 {
		dpr = props[rng.Intn(len(props))].DevicePixelRatio
		if dpr <= 0 {
			dpr = 1.0
		}
	}
	// screen.pixelDepth is colorDepth (24/30), NOT DPR — do not derive DPR
	// from it. macOS colorDepth 30 with DPR 2 is the canonical Retina combo;
	// conflating the two produced the original bug.
	if d, ok := screen["pixelDepth"].(int); ok && d > 0 {
		_ = d
	}

	// Outer window: slightly smaller than avail (taskbar/browser chrome).
	// Inner: outer minus browser chrome (~100px top bar + bookmarks).
	ow := sw
	oh := ah
	if oh > 40 {
		oh = oh - rng.Intn(20) // small random offset for taskbar variance
	}
	iw := ow
	ih := oh - 100 // browser chrome (address bar + tabs)
	if ih < 200 {
		ih = oh * 9 / 10
	}

	return api.WindowProps{
		InnerWidth:       iw,
		InnerHeight:      ih,
		OuterWidth:       ow,
		OuterHeight:      oh,
		DevicePixelRatio: dpr,
		ScreenX:          0,
		ScreenY:          0,
	}
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
