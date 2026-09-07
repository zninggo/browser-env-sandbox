// Package fpengine — data-driven device profiles.
//
// The legacy generator sampled each fingerprint dimension (GPU, screen, fonts,
// CPU cores, memory, platform) independently, with only coarse OS-level
// filtering. Real devices do not work that way: an Apple M-series laptop has a
// Metal renderer *and* a Retina screen *and* Apple fonts *and* 8-10 cores, all
// at once. Sampling those dimensions independently produces impossible combos
// (e.g. an Apple M2 GPU with a Windows D3D11 screen and a 24-core desktop CPU)
// that any cross-checking detector flags.
//
// This file introduces a joint, data-driven sampler. It consumes a device
// profile store whose every dimension is conditioned on the browser User-Agent
// (the conditioning variable that collocates browser version, OS, and device
// class), and draws the whole fingerprint from one UA in one step. The store is
// built from the apify/fingerprint-suite Bayesian network (see
// internal/fpupdate/parser.go), so the correlations are observed frequencies
// from real browsers, not hand-authored rules.
package fpengine

import (
	"strconv"
	"strings"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// DeviceProfile is a coherent joint realization of the correlated fingerprint
// dimensions. Unlike the legacy independent sampler, every field was drawn
// from the same conditioning UA, so GPU ↔ screen ↔ fonts ↔ CPU ↔ memory ↔
// platform co-vary like a real device.
type DeviceProfile struct {
	UserAgent string `json:"user_agent"`
	// Browser is one of "chrome", "firefox", "safari".
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"` // major version, e.g. "147"
	BrowserMajor   int    `json:"browser_major"`
	// OS is one of "windows", "macos", "linux", "android".
	OS       string `json:"os"`
	Platform string `json:"platform"` // navigator.platform, e.g. "Win32"
	Vendor   string `json:"vendor"`   // navigator.vendor, e.g. "Google Inc."
	// GPU (WebGL) renderer + vendor, sampled from the UA-conditioned videoCard
	// distribution (Windows → Direct3D11, macOS → Metal, Android → Mali/Adreno).
	GPUDevice
	Screen              DeviceScreen `json:"screen"`
	Fonts               []string     `json:"fonts"`
	HardwareConcurrency int          `json:"hardware_concurrency"`
	DeviceMemory        int          `json:"device_memory"`
	MaxTouchPoints      int          `json:"max_touch_points"`
}

// GPUDevice is the WebGL renderer/vendor pair (mirrors api.GPUProfile fields).
type GPUDevice struct {
	Renderer string `json:"renderer"`
	Vendor   string `json:"vendor"`
}

// DeviceScreen is a screen/geometry realization drawn from the UA-conditioned
// screen distribution.
type DeviceScreen struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	AvailWidth       int     `json:"avail_width"`
	AvailHeight      int     `json:"avail_height"`
	ColorDepth       int     `json:"color_depth"`
	PixelDepth       int     `json:"pixel_depth"`
	DevicePixelRatio float64 `json:"device_pixel_ratio"`
	OuterWidth       int     `json:"outer_width"`
	OuterHeight      int     `json:"outer_height"`
}

// --- store: the loaded joint distribution -----------------------------------

// dpGPU / dpScreen / dpWeightedInt / dpWeightedStr are the on-disk categorical
// entries with a per-UA conditional probability `P`.
type dpGPU struct {
	Renderer string  `json:"renderer"`
	Vendor   string  `json:"vendor"`
	P        float64 `json:"p"`
}

type dpScreen struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	AvailWidth       int     `json:"avail_width"`
	AvailHeight      int     `json:"avail_height"`
	ColorDepth       int     `json:"color_depth"`
	PixelDepth       int     `json:"pixel_depth"`
	DevicePixelRatio float64 `json:"device_pixel_ratio"`
	OuterWidth       int     `json:"outer_width"`
	OuterHeight      int     `json:"outer_height"`
	P                float64 `json:"p"`
}

type dpWeightedInt struct {
	V int     `json:"v"`
	P float64 `json:"p"`
}

type dpWeightedStr struct {
	V string  `json:"v"`
	P float64 `json:"p"`
}

// dpFonts deduplicates font lists into a shared pool; the conditional table
// references pool indices so the 929 distinct font lists are not repeated for
// every UA.
type dpFonts struct {
	Pool [][]string        `json:"pool"`
	Cond map[string][]dpWeightedIdx `json:"cond"`
}

type dpWeightedIdx struct {
	I int     `json:"i"`
	P float64 `json:"p"`
}

// deviceProfileData is the on-disk JSON shape of the joint distribution.
type deviceProfileData struct {
	Version             string                     `json:"version"`
	Source              string                     `json:"source"`
	SourceVersion       string                     `json:"source_version"`
	UserAgents          []string                   `json:"user_agents"`
	UAPriors            []float64                  `json:"ua_priors"`
	VideoCard           map[string][]dpGPU         `json:"video_card"`
	Screen              map[string][]dpScreen      `json:"screen"`
	HardwareConcurrency map[string][]dpWeightedInt `json:"hardware_concurrency"`
	DeviceMemory        map[string][]dpWeightedInt `json:"device_memory"`
	Platform            map[string][]dpWeightedStr `json:"platform"`
	Vendor              map[string][]dpWeightedStr `json:"vendor"`
	MaxTouchPoints      map[string][]dpWeightedInt `json:"max_touch_points"`
	Fonts               dpFonts                    `json:"fonts"`
}

// deviceProfileStore is the parsed, index-ready form of deviceProfileData.
type deviceProfileStore struct {
	// uas holds the sampled conditioning User-Agents kept for sampling.
	uas []string
	// priors aligns with uas: the unconditional probability of each UA.
	priors []float64
	// browserOS precomputes (browser, os) per UA for fast constraint filtering.
	browserOS []uaClass

	data deviceProfileData
}

type uaClass struct {
	browser string
	os      string
}

func (s *deviceProfileStore) len() int { return len(s.uas) }

// buildStore validates a loaded deviceProfileData and precomputes the per-UA
// classes, dropping UAs outside the engine's supported browser
// (chrome/firefox/safari) and OS (windows/macos/linux/android) families.
// Dropped UAs (iOS, Edge, Chromium-variants, ChromeOS) fall back to the legacy
// KB path, which is the documented behavior for those cases.
func buildStore(d deviceProfileData) (*deviceProfileStore, error) {
	if len(d.UserAgents) == 0 || len(d.UAPriors) != len(d.UserAgents) {
		return nil, errNoDeviceProfiles
	}
	s := &deviceProfileStore{data: d}
	for i, ua := range d.UserAgents {
		b, o := classifyUA(ua)
		if !supportedBrowser(b) || !supportedOS(o) {
			continue
		}
		// Pre-filter each UA's GPU distribution to OS-compatible renderers and
		// drop UAs whose GPU distribution is entirely OS-incompatible (a
		// bot/emulator fingerprint in the source, e.g. an Android UA with only
		// desktop Direct3D11 GPUs). The filtered list is stored so sampling
		// never re-filters and never draws an impossible combo.
		gpus := filterDPGPUsByOS(o, d.VideoCard[ua])
		if len(gpus) == 0 {
			continue
		}
		d.VideoCard[ua] = gpus
		s.uas = append(s.uas, ua)
		s.priors = append(s.priors, d.UAPriors[i])
		s.browserOS = append(s.browserOS, uaClass{browser: b, os: o})
	}
	if len(s.uas) == 0 {
		return nil, errNoDeviceProfiles
	}
	return s, nil
}

// uaPref returns the subset of kept-UA indices whose (browser, os) match the
// given constraints (empty = wildcard). ok=false when nothing matches.
func (s *deviceProfileStore) uaPref(browser, os string) ([]int, bool) {
	out := make([]int, 0, len(s.uas))
	for i := range s.uas {
		c := s.browserOS[i]
		if browser != "" && c.browser != browser {
			continue
		}
		if os != "" && c.os != os {
			continue
		}
		out = append(out, i)
	}
	return out, len(out) > 0
}

// sampleDeviceProfile draws a coherent DeviceProfile from the joint
// distribution, restricted to the given browser/os (empty = unconstrained).
// Returns ok=false when no UA satisfies the constraint (caller falls back to
// the legacy KB sampler).
func (s *deviceProfileStore) sampleDeviceProfile(rng *seededRNG, browser, os string) (DeviceProfile, bool) {
	idxs, ok := s.uaPref(browser, os)
	if !ok {
		return DeviceProfile{}, false
	}

	// 1. Sample the conditioning UA from the filtered prior.
	weights := make([]float64, len(idxs))
	for j, ui := range idxs {
		weights[j] = s.priors[ui]
	}
	ui := idxs[sampleWeightedIndex(rng, weights)]
	ua := s.uas[ui]

	c := s.browserOS[ui]
	b, o := c.browser, c.os
	verStr, major := browserVersion(ua, b)

	// 2. Sample every dimension conditioned on this UA.
	d := DeviceProfile{
		UserAgent:       ua,
		Browser:         b,
		BrowserVersion:  verStr,
		BrowserMajor:    major,
		OS:              o,
		Platform:        sampleWeightedStr(rng, s.data.Platform[ua]),
		Vendor:          sampleWeightedStr(rng, s.data.Vendor[ua]),
		DeviceMemory:    clampDevMem(sampleWeightedInt(rng, s.data.DeviceMemory[ua])),
		MaxTouchPoints:  sampleWeightedInt(rng, s.data.MaxTouchPoints[ua]),
	}
	d.HardwareConcurrency = clampHWConc(sampleWeightedInt(rng, s.data.HardwareConcurrency[ua]))

	if g, okk := sampleWeightedGPU(rng, filterDPGPUsByOS(o, s.data.VideoCard[ua])); okk {
		d.GPUDevice = GPUDevice{Renderer: g.Renderer, Vendor: g.Vendor}
	}
	if sc, okk := sampleWeightedScreen(rng, s.data.Screen[ua]); okk {
		d.Screen = toDeviceScreen(sc)
	}
	if f, okk := sampleWeightedFonts(rng, s.data.Fonts, ua); okk {
		d.Fonts = f
	}

	// A realization that failed to resolve its core correlated dimensions is
	// unusable; signal a miss so the caller falls back to the legacy path.
	if d.Platform == "" && d.Vendor == "" && d.GPUDevice.Renderer == "" && len(d.Fonts) == 0 {
		return DeviceProfile{}, false
	}

	return d, true
}

func toDeviceScreen(sc dpScreen) DeviceScreen {
	return DeviceScreen{
		Width:            sc.Width,
		Height:           sc.Height,
		AvailWidth:       sc.AvailWidth,
		AvailHeight:      sc.AvailHeight,
		ColorDepth:       sc.ColorDepth,
		PixelDepth:       sc.PixelDepth,
		DevicePixelRatio: sc.DevicePixelRatio,
		OuterWidth:       sc.OuterWidth,
		OuterHeight:      sc.OuterHeight,
	}
}

// --- weighted categorical sampling ------------------------------------------

// sampleWeightedIndex returns the index of a categorical sample using `weights`
// as (unnormalized) probabilities.
func sampleWeightedIndex(rng *seededRNG, weights []float64) int {
	var total float64
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return rng.Intn(len(weights))
	}
	r := rng.Float64() * total
	var acc float64
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		acc += w
		if r < acc {
			return i
		}
	}
	return len(weights) - 1
}

// sampleWeightedBy picks one element using weight() as (unnormalized)
// probability. Used by the typed samplers below.
func sampleWeightedBy[T any](rng *seededRNG, items []T, weight func(T) float64) (T, bool) {
	var zero T
	if len(items) == 0 {
		return zero, false
	}
	weights := make([]float64, len(items))
	for i, it := range items {
		weights[i] = weight(it)
	}
	return items[sampleWeightedIndex(rng, weights)], true
}

func sampleWeightedGPU(rng *seededRNG, items []dpGPU) (dpGPU, bool) {
	return sampleWeightedBy(rng, items, func(g dpGPU) float64 { return g.P })
}

// filterDPGPUsByOS returns only GPUs whose renderer string is OS-consistent,
// mirroring the legacy filterGPUsByOS. The apify source conditions videoCard on
// UA, which usually implies the OS, but the android UA bucket in particular
// contains a small sub-population of desktop Direct3D11 renderers (emulators /
// bot fingerprints in the source). Sampling those would produce a physically
// impossible combo (Android + desktop NVIDIA/Intel D3D11), so they are filtered
// out here — exactly the correlation guard the device-profile model exists to
// enforce. Unlike the legacy global filter, an empty result is NOT replaced by
// the unfiltered list: the caller (buildStore) treats a UA whose entire GPU
// distribution is OS-incompatible as contaminated and drops it.
func filterDPGPUsByOS(osName string, gpus []dpGPU) []dpGPU {
	out := make([]dpGPU, 0, len(gpus))
	for _, g := range gpus {
		r, v := g.Renderer, g.Vendor
		switch osName {
		case "macos":
			if (strings.Contains(r, "Metal") || strings.Contains(r, "OpenGL") || strings.Contains(v, "Apple")) &&
				!strings.Contains(r, "Direct3D11") {
				out = append(out, g)
			}
		case "windows":
			if strings.Contains(r, "Direct3D11") && !strings.Contains(r, "Metal") && !strings.Contains(v, "Apple") {
				out = append(out, g)
			}
		case "linux":
			if !strings.Contains(r, "Direct3D11") && !strings.Contains(r, "Metal") && !strings.Contains(v, "Apple") {
				out = append(out, g)
			}
		case "android":
			isAndroid := strings.Contains(r, "Mali") || strings.Contains(r, "Adreno") ||
				strings.Contains(r, "PowerVR") || strings.Contains(r, "ImgTec") ||
				strings.Contains(r, "OpenGL ES") || strings.Contains(r, "Vulkan")
			if isAndroid && !strings.Contains(r, "Direct3D11") && !strings.Contains(r, "Metal") && !strings.Contains(v, "Apple") {
				out = append(out, g)
			}
		default:
			out = append(out, g)
		}
	}
	return out
}

func sampleWeightedScreen(rng *seededRNG, items []dpScreen) (dpScreen, bool) {
	return sampleWeightedBy(rng, items, func(s dpScreen) float64 { return s.P })
}

func sampleWeightedInt(rng *seededRNG, items []dpWeightedInt) int {
	v, ok := sampleWeightedBy(rng, items, func(x dpWeightedInt) float64 { return x.P })
	if !ok {
		return 0
	}
	return v.V
}

func sampleWeightedStr(rng *seededRNG, items []dpWeightedStr) string {
	v, ok := sampleWeightedBy(rng, items, func(x dpWeightedStr) float64 { return x.P })
	if !ok {
		return ""
	}
	return v.V
}

// sampleWeightedFonts resolves a font list from the deduplicated pool, sampled
// by the UA-conditioned index distribution.
func sampleWeightedFonts(rng *seededRNG, f dpFonts, ua string) ([]string, bool) {
	cond, ok := f.Cond[ua]
	if !ok || len(cond) == 0 {
		return nil, false
	}
	idx, ok := sampleWeightedBy(rng, cond, func(x dpWeightedIdx) float64 { return x.P })
	if !ok || idx.I < 0 || idx.I >= len(f.Pool) {
		return nil, false
	}
	return f.Pool[idx.I], true
}

func clampHWConc(v int) int {
	if v < minHardwareConcurrency {
		return minHardwareConcurrency
	}
	if v > maxHardwareConcurrency {
		return maxHardwareConcurrency
	}
	return v
}

func clampDevMem(v int) int {
	if v < 1 {
		return 4
	}
	return v
}

// --- User-Agent classification ---------------------------------------------

func supportedBrowser(b string) bool {
	return b == "chrome" || b == "firefox" || b == "safari"
}

func supportedOS(o string) bool {
	return o == "windows" || o == "macos" || o == "linux" || o == "android"
}

// classifyUA returns (browser, os) for a UA string.
func classifyUA(ua string) (string, string) {
	return uaBrowser(ua), uaOS(ua)
}

func uaBrowser(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "edg/"):
		return "edge"
	case strings.Contains(l, "firefox/"):
		return "firefox"
	case strings.Contains(l, "chrome/"):
		return "chrome"
	case strings.Contains(l, "version/") && strings.Contains(l, "safari"):
		return "safari"
	default:
		return "other"
	}
}

func uaOS(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "android"):
		return "android"
	case strings.Contains(l, "iphone") || strings.Contains(l, "ipad") || strings.Contains(l, "ipod"):
		return "ios"
	case strings.Contains(l, "mac os x") || strings.Contains(l, "macintosh"):
		return "macos"
	case strings.Contains(l, "windows nt") || strings.Contains(l, "windows"):
		return "windows"
	case strings.Contains(l, "cros"):
		return "chromeos"
	case strings.Contains(l, "linux"):
		return "linux"
	default:
		return "other"
	}
}

// browserVersion extracts the major version from a UA for the given browser.
func browserVersion(ua, browser string) (string, int) {
	var token string
	switch browser {
	case "firefox":
		token = "Firefox/"
	case "safari":
		token = "Version/"
	default: // chrome / edge / fallback
		token = "Chrome/"
	}
	v := extractVersionToken(ua, token)
	if v == "" {
		return "0", 0
	}
	major := 0
	if i := strings.IndexByte(v, '.'); i > 0 {
		major, _ = strconv.Atoi(v[:i])
	} else {
		major, _ = strconv.Atoi(v)
	}
	if major == 0 {
		return v, 0
	}
	return strconv.Itoa(major), major
}

// extractVersionToken returns the version substring that immediately follows
// token (e.g. "Chrome/"), up to the next non-version character.
func extractVersionToken(ua, token string) string {
	i := strings.Index(ua, token)
	if i < 0 {
		return ""
	}
	rest := ua[i+len(token):]
	j := 0
	for j < len(rest) && (rest[j] == '.' || (rest[j] >= '0' && rest[j] <= '9')) {
		j++
	}
	return rest[:j]
}

// buildDeviceOSProfile constructs an api.OSProfile for a sampled DeviceProfile,
// reusing the built-in KB OS entry for version/UASegment and overriding
// platform + fonts with the joint values.
func buildDeviceOSProfile(kb *KnowledgeBase, d DeviceProfile) api.OSProfile {
	osp := kb.oseByName(d.OS)
	osp.Platform = d.Platform
	if len(d.Fonts) > 0 {
		osp.DefaultFonts = d.Fonts
	}
	return osp
}

func (kb *KnowledgeBase) oseByName(name string) api.OSProfile {
	for _, o := range kb.OSes {
		if o.Name == name {
			return api.OSProfile{
				Name:         o.Name,
				Version:      o.Version,
				Platform:     o.Platform,
				UASegment:    o.UASegment,
				DefaultFonts: o.DefaultFonts,
			}
		}
	}
	return api.OSProfile{Name: name}
}

// errNoDeviceProfiles is returned when no usable joint distribution is loaded.
var errNoDeviceProfiles = errDeviceProfilesUnavailable{}

type errDeviceProfilesUnavailable struct{}

func (errDeviceProfilesUnavailable) Error() string {
	return "no device profile data available"
}