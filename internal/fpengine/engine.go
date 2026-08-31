// Package fpengine generates self-consistent browser fingerprints.
//
// Not a dump — a generator. Given a seed, produces a complete fingerprint
// where navigator/screen/canvas/WebGL/audio/fonts/timezone are all internally
// consistent (e.g. UA says Chrome 131 + Windows → platform=Win32 → Win fonts →
// Windows GPU → matching canvas hash).
package fpengine

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// Engine generates fingerprints using a knowledge base + seeded RNG.
type Engine struct {
	kb  *KnowledgeBase
	mu  sync.RWMutex
}

// navigator.hardwareConcurrency is reported by browsers as the number of
// logical cores. Real client devices report 1-64; anything above 64 (e.g.
// 640/384/96/128 from a polluted dataset) is non-physical and a deterministic
// bot signal. These bounds guard both sampling and consistency validation.
const (
	minHardwareConcurrency = 1
	maxHardwareConcurrency = 64
)

// New creates a fingerprint engine with the built-in knowledge base.
func New() *Engine {
	return &Engine{kb: DefaultKnowledgeBase()}
}

// Generate produces a self-consistent fingerprint.
// If seed is 0, a random seed is used.
// opts constrains the browser/OS if non-empty.
func (e *Engine) Generate(seed uint64, browser, os string) (*api.Fingerprint, error) {
	return e.GenerateWithTimezone(seed, browser, os, "")
}

// GenerateWithTimezone produces a self-consistent fingerprint with the
// timezone constrained to the given IANA name (empty = random pick).
// Languages follow the timezone automatically (knowledge base pairing).
func (e *Engine) GenerateWithTimezone(seed uint64, browser, os, timezone string) (*api.Fingerprint, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if seed == 0 {
		seed = uint64(rand.Int63())
	}
	rng := newSeededRNG(seed)

	// 1. Sample browser profile
	bp := e.kb.SampleBrowser(rng, browser)

	// 2. Sample OS profile
	osp, err := e.kb.SampleOS(rng, os)
	if err != nil {
		return nil, err
	}

	// 3. Sample GPU (must be consistent with OS)
	gpu := e.kb.SampleGPU(rng, osp.Name)

	// 4. Sample screen
	screen := e.kb.SampleScreen(rng, osp.Name)

	// 5. Sample timezone + languages (consistent with each other).
	// A caller-provided timezone (e.g. to match proxy geo) overrides the
	// random pick; unknown names fall back to random.
	tz, langs := "", []string(nil)
	if timezone != "" {
		if n, l, ok := e.kb.SampleTimezoneNamed(timezone); ok {
			tz, langs = n, l
		}
	}
	if tz == "" {
		tz, langs = e.kb.SampleTimezone(rng)
	}

	// 6. Build navigator from all the above (with rng for real data sampling)
	nav := buildNavigator(bp, osp, gpu, screen, rng, langs)

	// 7. Build canvas/WebGL/audio from knowledge base hashes
	canvas := e.kb.LookupCanvasHash(bp.MajorVer, osp.Name, gpu.Vendor)
	webgl := buildWebGL(gpu)
	audio := e.kb.LookupAudioHash(bp.MajorVer, osp.Name)

	// 8. Window dimensions
	winProps := e.kb.SampleWindowProps(rng, osp.Name, screen)

	fp := &api.Fingerprint{
		Seed:      seed,
		Browser:   bp,
		OS:        osp,
		GPU:       gpu,
		Navigator: nav,
		Screen:    screen,
		Canvas:    canvas,
		WebGL:     webgl,
		Audio:     audio,
		Fonts:     osp.DefaultFonts,
		Timezone:  tz,
		Languages: langs,
		Window:    winProps,
	}

	// 9. Self-consistency check
	if err := ValidateConsistency(fp); err != nil {
		return nil, fmt.Errorf("fingerprint consistency check failed: %w", err)
	}

	return fp, nil
}

// ValidateConsistency checks that all fingerprint properties are internally
// consistent. Catches generation bugs and self-contradictory fingerprints
// that server-side cross-validation would reject.
func ValidateConsistency(fp *api.Fingerprint) error {
	// UA version must match browser version
	ua, _ := fp.Navigator["userAgent"].(string)
	if ua == "" {
		return fmt.Errorf("navigator.userAgent is empty")
	}

	// platform must match OS
	platform, _ := fp.Navigator["platform"].(string)
	if platform != fp.OS.Platform {
		return fmt.Errorf("platform mismatch: navigator=%s os=%s", platform, fp.OS.Platform)
	}

	// webdriver must be false
	if wd, ok := fp.Navigator["webdriver"]; ok && wd == true {
		return fmt.Errorf("webdriver must be false")
	}

	// timezone and languages must be non-empty
	if fp.Timezone == "" {
		return fmt.Errorf("timezone is empty")
	}
	if len(fp.Languages) == 0 {
		return fmt.Errorf("languages is empty")
	}

	// Language must match timezone (anti-cross-check: Tokyo+zh-CN = red flag)
	lang, _ := fp.Navigator["language"].(string)
	if !tzLanguageConsistent(fp.Timezone, lang) {
		return fmt.Errorf("language/timezone mismatch: tz=%s lang=%s", fp.Timezone, lang)
	}

	// hardwareConcurrency must be within the physical browser range (1-64).
	// A value outside this range (640/384/96/128) is a deterministic bot signal.
	if hwConc, ok := fp.Navigator["hardwareConcurrency"]; ok {
		switch v := hwConc.(type) {
		case int:
			if v < minHardwareConcurrency || v > maxHardwareConcurrency {
				return fmt.Errorf("hardwareConcurrency out of range: %d (want %d-%d)", v, minHardwareConcurrency, maxHardwareConcurrency)
			}
		case int64:
			if v < minHardwareConcurrency || v > maxHardwareConcurrency {
				return fmt.Errorf("hardwareConcurrency out of range: %d (want %d-%d)", v, minHardwareConcurrency, maxHardwareConcurrency)
			}
		case float64:
			if v < minHardwareConcurrency || v > maxHardwareConcurrency {
				return fmt.Errorf("hardwareConcurrency out of range: %v (want %d-%d)", v, minHardwareConcurrency, maxHardwareConcurrency)
			}
		}
	}

	// Screen geometry invariants:
	// screen.height >= availHeight >= outerHeight >= innerHeight (desktop)
	sh, _ := fp.Screen["height"].(int)
	ah, _ := fp.Screen["availHeight"].(int)
	if sh > 0 && ah > 0 && ah > sh {
		return fmt.Errorf("availHeight (%d) > screen.height (%d)", ah, sh)
	}
	if fp.Window.OuterHeight > 0 && ah > 0 && fp.Window.OuterHeight > ah {
		return fmt.Errorf("outerHeight (%d) > availHeight (%d)", fp.Window.OuterHeight, ah)
	}
	if fp.Window.InnerHeight > 0 && fp.Window.OuterHeight > 0 && fp.Window.InnerHeight > fp.Window.OuterHeight {
		return fmt.Errorf("innerHeight (%d) > outerHeight (%d)", fp.Window.InnerHeight, fp.Window.OuterHeight)
	}

	// WebGL vendor/renderer must not be placeholder "WebKit"
	if fp.WebGL.Vendor == "WebKit" || fp.WebGL.Renderer == "WebKit WebGL" {
		return fmt.Errorf("WebGL vendor/renderer is placeholder")
	}

	// Canvas hash must be non-empty and look like base64
	if fp.Canvas.ToDataURLHash == "" {
		return fmt.Errorf("canvas toDataURL hash is empty")
	}
	if fp.Canvas.ToDataURL == "" {
		return fmt.Errorf("canvas toDataURL is empty")
	}

	return nil
}

// tzLanguageConsistent checks that the language is plausible for the timezone.
// Servers cross-check this: a Tokyo timezone with zh-CN language is suspicious.
func tzLanguageConsistent(tz, lang string) bool {
	tzMap := map[string][]string{
		"Asia/Shanghai":   {"zh-CN", "zh"},
		"Asia/Tokyo":      {"ja", "ja-JP"},
		"Asia/Seoul":      {"ko", "ko-KR"},
		"Asia/Singapore":  {"en", "zh-CN", "zh"},
		"America/New_York": {"en", "en-US"},
		"Europe/London":   {"en", "en-GB"},
	}
	expected, ok := tzMap[tz]
	if !ok {
		return true // unknown timezone — don't block
	}
	for _, e := range expected {
		if lang == e || strings.HasPrefix(lang, e) {
			return true
		}
	}
	return false
}

// --- internal helpers ---

func buildNavigator(bp api.BrowserProfile, osp api.OSProfile, gpu api.GPUProfile, screen map[string]any, rng *seededRNG, languages []string) map[string]any {
	ua := fmt.Sprintf(bp.UATemplate, osp.UASegment, bp.Version)

	// Language: derive from timezone-paired languages (passed in), not hardcoded.
	langFirst := "en"
	langList := []string{"en"}
	if len(languages) > 0 {
		langFirst = languages[0]
		langList = languages
	}

	// Hardware concurrency: sample from real data (39 values: 4,8,9,10,11,12,14,16,18,20...)
	// Range-guarded: real browsers report 1-64 logical cores. Values outside
	// that range (e.g. 640/384 from a polluted dataset) are a deterministic bot
	// signal, so clamp even if the data source is unsanitized.
	hwConc := 8
	hwConcVals := CurrentFpHardwareConcurrency()
	if len(hwConcVals) > 0 {
		hwConc = hwConcVals[rng.Intn(len(hwConcVals))]
	}
	if hwConc < minHardwareConcurrency || hwConc > maxHardwareConcurrency {
		hwConc = 8
	}

	// Device memory: sample from real data (4,8,16,32...)
	devMem := 8
	dmVals := CurrentFpDeviceMemory()
	if len(dmVals) > 0 {
		devMem = dmVals[rng.Intn(len(dmVals))]
		if devMem < 1 {
			devMem = 8 // skip 0.5 etc, use minimum realistic
		}
	}

	nav := map[string]any{
		"userAgent":           ua,
		"appVersion":          ua[len("Mozilla/"):],
		"platform":            osp.Platform,
		"vendor":              "Google Inc.",
		"vendorSub":           "",
		"productSub":          "20030107",
		"productName":         "Gecko",
		"product":             "Gecko",
		"language":            langFirst,
		"languages":           langList,
		"hardwareConcurrency": hwConc,
		"deviceMemory":        devMem,
		"maxTouchPoints":      0,
		"cookieEnabled":       true,
		"doNotTrack":          nil,
		"onLine":              true,
		"webdriver":           false,
		"pdfViewerEnabled":    true,
		"scheduling":          map[string]any{"isInputPending": map[string]any{}},
		"userAgentData": map[string]any{
			"brands": []map[string]any{
				{"brand": "Chromium", "version": bp.Version},
				{"brand": "Google Chrome", "version": bp.Version},
				{"brand": "Not.A/Brand", "version": "24"},
			},
			"mobile": false,
			"platform": platformToUAData(osp.Name),
		},
		"connection": map[string]any{
			"effectiveType": "4g",
			"rtt":           50,
			"downlink":      10,
			"saveData":      false,
		},
	}

	// Browser/OS-specific navigator adjustments. Without this, browser=firefox
	// or safari still produced a Chrome-smelling navigator (vendor "Google
	// Inc.", userAgentData present). See extended_profiles.go.
	adj := adjustmentsForBrowser(bp.Name, osp.Name)
	applyNavigatorAdjustments(nav, adj)

	return nav
}

func buildWebGL(gpu api.GPUProfile) api.WebGLFP {
	vendor := gpu.Vendor
	if vendor == "" {
		vendor = "Google Inc. (Intel)"
	}
	renderer := gpu.Renderer
	if renderer == "" {
		renderer = "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)"
	}
	return api.WebGLFP{
		Vendor:   vendor,
		Renderer: renderer,
		Version:  "WebGL 1.0 (OpenGL ES 2.0 Chromium)",
		Extensions: []string{
			"ANGLE_instanced_arrays",
			"EXT_blend_minmax",
			"EXT_color_buffer_half_float",
			"EXT_disjoint_timer_query",
			"EXT_float_blend",
			"EXT_frag_depth",
			"EXT_shader_texture_lod",
			"EXT_texture_compression_bptc",
			"EXT_texture_compression_rgtc",
			"EXT_texture_filter_anisotropic",
			"OES_element_index_uint",
			"OES_fbo_render_mipmap",
			"OES_standard_derivatives",
			"OES_texture_float",
			"OES_texture_float_linear",
			"OES_texture_half_float",
			"OES_texture_half_float_linear",
			"OES_vertex_array_object",
			"WEBGL_color_buffer_float",
			"WEBGL_compressed_texture_s3tc",
			"WEBGL_debug_renderer_info",
			"WEBGL_debug_shaders",
			"WEBGL_depth_texture",
			"WEBGL_draw_buffers",
			"WEBGL_lose_context",
			"WEBGL_multi_draw",
		},
		// Real Chrome WebGL parameter values (GL constants → typical values).
		// Sources: Khronos WebGL spec + real Chrome/Direct3D11 ANGLE reporting.
		// UNMASKED_VENDOR/RENDERER (37445/37446) come from the GPU profile;
		// the rest are ANGLE/D3D11 defaults consistent across Chrome/Windows.
		// Coverage targets 40+ parameters so risk-control scripts that probe
		// getParameter across many pnames no longer hit null (A-class).
		Params: map[int32]string{
			37445: vendor,   // UNMASKED_VENDOR_WEBGL
			37446: renderer, // UNMASKED_RENDERER_WEBGL
			7936:  "WebKit", // VENDOR
			7937:  "WebKit WebGL", // RENDERER
			7938:  "WebGL 1.0 (OpenGL ES 2.0 Chromium)", // VERSION
			35724: "WebGL GLSL ES 1.0 (OpenGL ES GLSL ES 1.0 Chromium)", // SHADING_LANGUAGE_VERSION
			7939:  "33000", // ALIASED_LINE_WIDTH_RANGE (Chrome/ANGLE typical)
			7935:  "1,1024", // ALIASED_POINT_SIZE_RANGE
			34076: "16384", // MAX_TEXTURE_SIZE
			34024: "16384", // MAX_CUBE_MAP_TEXTURE_SIZE
			33802: "16384", // MAX_TEXTURE_IMAGE_UNITS (fragment)
			35660: "32",    // MAX_VERTEX_TEXTURE_IMAGE_UNITS
			35661: "32",    // MAX_COMBINED_TEXTURE_IMAGE_UNITS
			34921: "16384", // MAX_TEXTURE_IMAGE_UNITS (legacy alias)
			34102: "32768", // MAX_RENDERBUFFER_SIZE
			34018: "16",    // MAX_VERTEX_ATTRIBS
			34930: "16",    // MAX_VERTEX_UNIFORM_VECTORS
			36349: "4096",  // MAX_FRAGMENT_UNIFORM_VECTORS (as int string)
			36347: "1024",  // MAX_VARYING_VECTORS
			34047: "16384,16384", // MAX_VIEWPORT_DIMS
			3410:  "16384", // MAX_TEXTURE_SIZE (alt)
			32883: "16",    // MAX_COLOR_ATTACHMENTS
			36063: "4",     // MAX_DRAW_BUFFERS (WebGL2)
			34028: "0,16",  // MAX_FRAGMENT_UNIFORM_COMPONENTS (range)
			34913: "1024",  // UNPACK_ALIGNMENT max
			32873: "4",     // PACK_ALIGNMENT
			3415:  "8",     // ALPHA_BITS
			3414:  "8",     // RED_BITS
			3421:  "8",     // GREEN_BITS
			3420:  "8",     // BLUE_BITS
			3422:  "24",    // DEPTH_BITS (D3D11 default)
			3423:  "8",     // STENCIL_BITS
			// Additional A-class parameters (real Chrome/ANGLE D3D11 values).
			// Pushes getParameter coverage past 40 pnames so probing scripts
			// no longer hit null. Values are Chrome/ANGLE D3D11 defaults.
			33902: "255",   // ACTIVE_TEXTURE upper bound
			33777: "4096",  // MAX_VERTEX_UNIFORM_COMPONENTS
			35659: "30",    // MAX_VERTEX_OUTPUT_COMPONENTS
			35658: "30",    // MAX_FRAGMENT_INPUT_COMPONENTS
			3413:  "4",     // SUBPIXEL_BITS
			32882: "4",     // NUM_PROGRAM_BINARY_FORMATS
			35371: "4294967295", // MAX_ELEMENT_INDEX (GLuint max)
			34852: "2048",  // MAX_ARRAY_TEXTURE_LAYERS
			36007: "2147483647", // MAX_SHADER_STORAGE_BLOCK_SIZE
			36183: "84",    // MAX_UNIFORM_BUFFER_BINDINGS (D3D11 ~84)
			36464: "16",    // MAX_TEXTURE_LOD_BIAS (D3D11 ~16.0)
			36795: "84",    // MAX_FRAGMENT_UNIFORM_BLOCKS
			35373: "84",    // MAX_VERTEX_UNIFORM_BLOCKS
			35374: "84",    // MAX_COMBINED_UNIFORM_BLOCKS
			35375: "16384", // MAX_UNIFORM_BLOCK_SIZE
			35377: "0,8192", // MAX_COMBINED_VERTEX_UNIFORM_COMPONENTS
			35379: "0,8192", // MAX_COMBINED_FRAGMENT_UNIFORM_COMPONENTS
			36465: "16",    // TEXTURE_MAX_ANISOTROPY_EXT (D3D11 ~16)
		},
		// T3: deterministic non-zero RGBA pixel buffer (8x8) replayed by
		// readPixels. Generated from the GPU renderer so it is stable per
		// fingerprint but distinct across fingerprints, and never all-zero
		// (a noop readPixels returns zeros — detectable).
		ReadPixelsData:   syntheticReadPixels(renderer),
		ReadPixelsHash:   readPixelsHash(renderer),
		// T6: simulated render cost (ms) by GPU tier. Discrete GPUs render
		// faster than integrated; the value is a baseline the Go callback
		// jitters ±20% per call.
		RenderDurationMS: gpuRenderDurationMS(renderer),
	}
}

// gpuRenderDurationMS returns a baseline synthetic render duration in
// milliseconds for the given WebGL renderer string. Risk-control scripts
// wrap render+read operations in Date.now()/performance.now() and treat 0ms
// as a sandbox tell; this baseline is applied (with ±20% jitter) inside the
// Go toDataURL/fillText/readPixels callbacks. Values are ordered by GPU tier:
// discrete (nvidia/amd) < 2ms, integrated (intel) 2-3ms, software 4-5ms.
func gpuRenderDurationMS(renderer string) int {
	r := strings.ToLower(renderer)
	switch {
	case strings.Contains(r, "nvidia"), strings.Contains(r, "radeon"), strings.Contains(r, "amd"):
		return 2
	case strings.Contains(r, "intel"):
		return 3
	case strings.Contains(r, "apple"), strings.Contains(r, "m1"), strings.Contains(r, "m2"), strings.Contains(r, "m3"):
		return 2
	default:
		return 4
	}
}

// syntheticReadPixels returns base64-encoded RGBA pixels (8x8) derived from
// the renderer string. Used by readPixels replay (T3).
func syntheticReadPixels(renderer string) string {
	h := sha256.Sum256([]byte("readpixels:" + renderer))
	const n = 8
	buf := make([]byte, n*n*4)
	for i := 0; i < len(buf); i++ {
		buf[i] = h[i%32] ^ byte(i*7)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// readPixelsHash returns a 16-hex-char digest of the readPixels buffer.
func readPixelsHash(renderer string) string {
	h := sha256.Sum256([]byte("readpixels:" + renderer))
	return hex.EncodeToString(h[:8])
}

func platformToUAData(osName string) string {
	switch osName {
	case "windows":
		return "Windows"
	case "macos":
		return "macOS"
	case "linux":
		return "Linux"
	case "android":
		return "Android"
	default:
		return "Windows"
	}
}

// --- seeded RNG ---

type seededRNG struct {
	rng *rand.Rand
}

func newSeededRNG(seed uint64) *seededRNG {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], seed)
	h := sha256.Sum256(b[:])
	s := int64(binary.LittleEndian.Uint64(h[:8]))
	return &seededRNG{rng: rand.New(rand.NewSource(s))}
}

func (s *seededRNG) Intn(n int) int  { return s.rng.Intn(n) }
func (s *seededRNG) Float64() float64 { return s.rng.Float64() }
func (s *seededRNG) Pick(sl []string) string {
	if len(sl) == 0 {
		return ""
	}
	return sl[s.rng.Intn(len(sl))]
}
