package executor

// The identity probe: one script that reads the fingerprint identity surface
// and returns JSON. It runs in BOTH executors under the SAME ProfileSnapshot;
// VerifyConsistency compares the two readouts against each other and against
// the profile. The script must therefore stick to APIs that exist in the pure
// V8 environment as well as in real Chrome.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// identityProbeJS reads every identity point the dual-executor architecture
// pins to the device profile. Returns a JSON object of strings/numbers.
const identityProbeJS = `JSON.stringify((function () {
  function safe(fn, d) { try { var v = fn(); return v === undefined || v === null ? d : v; } catch (e) { return d; } }
  var gpu = ['', ''];
  safe(function () {
    var c = document.createElement('canvas');
    var g = c.getContext('webgl') || c.getContext('experimental-webgl');
    if (!g) { return; }
    var ext = g.getExtension('WEBGL_debug_renderer_info');
    if (ext) {
      gpu[0] = String(g.getParameter(ext.UNMASKED_VENDOR_WEBGL));
      gpu[1] = String(g.getParameter(ext.UNMASKED_RENDERER_WEBGL));
    }
  }, '');
  var uad = navigator.userAgentData || null;
  return {
    ua: String(navigator.userAgent),
    appVersion: String(navigator.appVersion),
    platform: String(navigator.platform),
    vendor: String(navigator.vendor),
    language: String(navigator.language),
    languages: (navigator.languages || []).join(','),
    hardwareConcurrency: navigator.hardwareConcurrency,
    deviceMemory: navigator.deviceMemory,
    maxTouchPoints: navigator.maxTouchPoints,
    webdriver: navigator.webdriver === undefined ? 'absent' : String(navigator.webdriver),
    screenWidth: screen.width,
    screenHeight: screen.height,
    screenAvailWidth: screen.availWidth,
    screenAvailHeight: screen.availHeight,
    colorDepth: screen.colorDepth,
    pixelDepth: screen.pixelDepth,
    orientationType: safe(function () { return screen.orientation.type; }, ''),
    dpr: window.devicePixelRatio,
    innerWidth: window.innerWidth,
    innerHeight: window.innerHeight,
    outerWidth: window.outerWidth,
    outerHeight: window.outerHeight,
    timezone: safe(function () { return Intl.DateTimeFormat().resolvedOptions().timeZone; }, ''),
    gpuVendor: gpu[0],
    gpuRenderer: gpu[1],
    uadPlatform: uad ? String(uad.platform) : '',
    uadMobile: uad ? String(uad.mobile) : '',
    uadBrands: uad ? (uad.brands || []).map(function (b) { return b.brand + '/' + b.version; }).sort().join(',') : ''
  };
})())`

// canvasProbeJS draws the standard canvas fingerprint scene (text + geometry)
// and reports the toDataURL length and a short digest. In Chrome this is a
// REAL rasterization; in the V8 sandbox it is the profile's replayed value —
// that difference is exactly what the dual executor exists to expose.
const canvasProbeJS = `JSON.stringify((function () {
  try {
    var c = document.createElement('canvas');
    c.width = 220; c.height = 40;
    var ctx = c.getContext('2d');
    ctx.textBaseline = 'alphabetic';
    ctx.fillStyle = '#f60';
    ctx.fillRect(125, 1, 62, 20);
    ctx.fillStyle = '#069';
    ctx.font = '11pt Arial';
    ctx.fillText('Caramel::walrus\xF0\x9F\x98\x80 @bes', 2, 15);
    ctx.fillStyle = 'rgba(102,204,0,0.7)';
    ctx.fillText('Caramel::walrus @bes', 4, 22);
    ctx.beginPath();
    ctx.arc(50, 50, 20, 0, Math.PI * 2);
    ctx.strokeStyle = '#333';
    ctx.stroke();
    var url = c.toDataURL();
    var png = url.indexOf('data:image/png;base64,') === 0;
    return { png: png, len: url.length, head: url.slice(0, 46) };
  } catch (e) {
    return { png: false, len: 0, head: String(e) };
  }
})())`

// wasmProbeJS compiles and runs a minimal real WebAssembly module. The module
// exports f() returning i32 42 (hand-assembled; see test for the byte layout).
const wasmProbeJS = `JSON.stringify((function () {
  try {
    if (typeof WebAssembly === 'undefined') { return { ok: false, why: 'no-wasm' }; }
    var bytes = new Uint8Array([
      0,97,115,109,1,0,0,0,
      1,5,1,96,0,1,127,
      3,2,1,0,
      7,5,1,1,102,0,0,
      10,6,1,4,0,65,42,11
    ]);
    var mod = new WebAssembly.Module(bytes);
    var inst = new WebAssembly.Instance(mod);
    var val = inst.exports.f();
    return { ok: val === 42, val: val, compile: 'sync' };
  } catch (e) {
    return { ok: false, why: String(e) };
  }
})())`

// IdentityProbe runs the identity probe in a session and parses the JSON.
func IdentityProbe(sess Session, timeoutSec int) (map[string]any, error) {
	code := fmt.Sprintf(`(function(){ var s = %s; return s; })()`, identityProbeJS)
	out, err := evalTimed(sess, code, timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("identity probe: %w", err)
	}
	return parseProbeJSON(out)
}

// CanvasProbe runs the canvas probe in a session.
func CanvasProbe(sess Session, timeoutSec int) (map[string]any, error) {
	out, err := evalTimed(sess, canvasProbeJS, timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("canvas probe: %w", err)
	}
	return parseProbeJSON(out)
}

// WasmProbe runs the wasm probe in a session.
func WasmProbe(sess Session, timeoutSec int) (map[string]any, error) {
	out, err := evalTimed(sess, wasmProbeJS, timeoutSec)
	if err != nil {
		return nil, fmt.Errorf("wasm probe: %w", err)
	}
	return parseProbeJSON(out)
}

func evalTimed(sess Session, code string, timeoutSec int) (string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	return sess.Eval(ctx, code, 0)
}

func parseProbeJSON(out string) (map[string]any, error) {
	out = strings.TrimSpace(out)
	// Session adapters stringify results; strip any surrounding quotes from
	// double-stringification before parsing.
	if strings.HasPrefix(out, `"`) && strings.HasSuffix(out, `"`) && len(out) >= 2 {
		var unquoted string
		if err := json.Unmarshal([]byte(out), &unquoted); err == nil {
			out = unquoted
		}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return nil, fmt.Errorf("probe output is not JSON: %.120s", out)
	}
	return m, nil
}

// ExpectedIdentity derives the profile's expected probe readout from the
// fingerprint. Every field the probe reports must match this under both
// executors — that is the cross-executor consistency contract.
func ExpectedIdentity(fp *api.Fingerprint) map[string]any {
	if fp == nil {
		return nil
	}
	nav := fp.Navigator
	ua, _ := nav["userAgent"].(string)
	appVersion, _ := nav["appVersion"].(string)
	platform, _ := nav["platform"].(string)
	if platform == "" {
		platform = fp.OS.Platform
	}
	vendor, _ := nav["vendor"].(string)
	language, _ := nav["language"].(string)
	languages := fp.Languages
	screenW := intFromAny(fp.Screen["width"])
	screenH := intFromAny(fp.Screen["height"])
	screenAW := intFromAny(fp.Screen["availWidth"])
	screenAH := intFromAny(fp.Screen["availHeight"])
	colorDepth := intFromAny(fp.Screen["colorDepth"])
	pixelDepth := intFromAny(fp.Screen["pixelDepth"])
	if pixelDepth == 0 {
		pixelDepth = colorDepth
	}
	orientationType := ""
	if o, ok := fp.Screen["orientation"].(map[string]any); ok {
		orientationType, _ = o["type"].(string)
	}
	uad, _ := nav["userAgentData"].(map[string]any)
	uadPlatform, _ := uad["platform"].(string)
	uadMobile := "false"
	if uad == nil {
		// Non-Chromium profiles: userAgentData is absent, and the probe reads
		// '' for every uad field (navigator.userAgentData is undefined).
		uadMobile = ""
	} else if m, ok := uad["mobile"].(bool); ok {
		uadMobile = fmt.Sprintf("%t", m)
	}
	uadBrands := ""
	if brands, ok := uad["brands"].([]map[string]any); ok {
		parts := make([]string, 0, len(brands))
		for _, b := range brands {
			brand, _ := b["brand"].(string)
			version, _ := b["version"].(string)
			parts = append(parts, brand+"/"+version)
		}
		// The probe sorts brand pairs; mirror that here.
		sortStrings(parts)
		uadBrands = strings.Join(parts, ",")
	}
	return map[string]any{
		"ua":                  ua,
		"appVersion":          appVersion,
		"platform":            platform,
		"vendor":              vendor,
		"language":            language,
		"languages":           strings.Join(languages, ","),
		"hardwareConcurrency": intFromAny(nav["hardwareConcurrency"]),
		"deviceMemory":        intFromAny(nav["deviceMemory"]),
		"maxTouchPoints":      intFromAny(nav["maxTouchPoints"]),
		"webdriver":           "false",
		"screenWidth":         screenW,
		"screenHeight":        screenH,
		"screenAvailWidth":    screenAW,
		"screenAvailHeight":   screenAH,
		"colorDepth":          colorDepth,
		"pixelDepth":          pixelDepth,
		"orientationType":     orientationType,
		"dpr":                 fp.Window.DevicePixelRatio,
		"innerWidth":          fp.Window.InnerWidth,
		"innerHeight":         fp.Window.InnerHeight,
		"outerWidth":          fp.Window.OuterWidth,
		"outerHeight":         fp.Window.OuterHeight,
		"timezone":            fp.Timezone,
		"gpuVendor":           fp.WebGL.Vendor,
		"gpuRenderer":         fp.WebGL.Renderer,
		"uadPlatform":         uadPlatform,
		"uadMobile":           uadMobile,
		"uadBrands":           uadBrands,
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
