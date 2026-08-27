// Package fpengine generates self-consistent browser fingerprints.
//
// Not a dump — a generator. Given a seed, produces a complete fingerprint
// where navigator/screen/canvas/WebGL/audio/fonts/timezone are all internally
// consistent (e.g. UA says Chrome 131 + Windows → platform=Win32 → Win fonts →
// Windows GPU → matching canvas hash).
package fpengine

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"

	"github.com/xiaoxun/bes/pkg/api"
)

// Engine generates fingerprints using a knowledge base + seeded RNG.
type Engine struct {
	kb  *KnowledgeBase
	mu  sync.RWMutex
}

// New creates a fingerprint engine with the built-in knowledge base.
func New() *Engine {
	return &Engine{kb: DefaultKnowledgeBase()}
}

// NewWithKB creates an engine with a custom knowledge base.
func NewWithKB(kb *KnowledgeBase) *Engine {
	return &Engine{kb: kb}
}

// Generate produces a self-consistent fingerprint.
// If seed is 0, a random seed is used.
// opts constrains the browser/OS if non-empty.
func (e *Engine) Generate(seed uint64, browser, os string) (*api.Fingerprint, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if seed == 0 {
		seed = uint64(rand.Int63())
	}
	rng := newSeededRNG(seed)

	// 1. Sample browser profile
	bp := e.kb.SampleBrowser(rng, browser)

	// 2. Sample OS profile
	osp := e.kb.SampleOS(rng, os)

	// 3. Sample GPU (must be consistent with OS)
	gpu := e.kb.SampleGPU(rng, osp.Name)

	// 4. Sample screen
	screen := e.kb.SampleScreen(rng, osp.Name)

	// 5. Sample timezone + languages (consistent with each other)
	tz, langs := e.kb.SampleTimezone(rng)

	// 6. Build navigator from all the above
	nav := buildNavigator(bp, osp, gpu, screen)

	// 7. Build canvas/WebGL/audio from knowledge base hashes
	canvas := e.kb.LookupCanvasHash(bp.MajorVer, osp.Name, gpu.Vendor)
	webgl := buildWebGL(gpu)
	audio := e.kb.LookupAudioHash(bp.MajorVer, osp.Name)

	// 8. Window dimensions
	winProps := e.kb.SampleWindowProps(rng, screen)

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
// consistent. This catches generation bugs early.
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

	return nil
}

// --- internal helpers ---

func buildNavigator(bp api.BrowserProfile, osp api.OSProfile, gpu api.GPUProfile, screen map[string]any) map[string]any {
	ua := fmt.Sprintf(bp.UATemplate, osp.UASegment, bp.Version)
	nav := map[string]any{
		"userAgent":           ua,
		"appVersion":          ua[len("Mozilla/"):],
		"platform":            osp.Platform,
		"vendor":              "Google Inc.",
		"vendorSub":           "",
		"productSub":          "20030107",
		"productName":         "Gecko",
		"product":             "Gecko",
		"language":            "zh-CN",
		"languages":           []string{"zh-CN", "zh"},
		"hardwareConcurrency": 8,
		"deviceMemory":        8,
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

	// OS-specific adjustments
	switch osp.Name {
	case "android":
		nav["maxTouchPoints"] = 5
		nav["hardwareConcurrency"] = 8
	}

	return nav
}

func buildWebGL(gpu api.GPUProfile) api.WebGLFP {
	return api.WebGLFP{
		Vendor:   "WebKit",
		Renderer: "WebKit WebGL",
		Version:  "WebGL 1.0 (OpenGL)",
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
		},
		Params: map[int32]string{
			37445: gpu.Vendor,   // UNMASKED_VENDOR_WEBGL
			37446: gpu.Renderer, // UNMASKED_RENDERER_WEBGL
			7936:  "WebKit",
			7937:  "WebKit WebGL",
			7938:  "WebGL 1.0 (OpenGL)",
		},
	}
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
