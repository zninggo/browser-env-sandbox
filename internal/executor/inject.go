package executor

// BuildInjectionJS translates a device profile into the JavaScript that makes
// a real Chrome surface the SAME identity points as the V8 sandbox running
// the same profile. This is the "one profile drives every executor" core:
// the values below are read straight out of the api.Fingerprint the V8
// EnvBuilder injects, so cross-executor consistency is by construction.
//
// What is deliberately NOT overridden in Chrome:
//   - canvas rasterization, AudioContext DSP, WebAssembly — the entire point
//     of the Chrome executor is that these execute for real;
//   - WebGL non-identity parameters — real GL values are more realistic than
//     the profile's canned ones; only the identity strings (vendor/renderer/
//     version constants, incl. UNMASKED_*) are pinned to the profile;
//   - font enumeration — the local host's real fonts would leak under a
//     foreign OS profile, so font points route to V8 instead (see Router).

import (
	"encoding/json"
	"fmt"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// injectionPayload is the data half of the injection: every identity value,
// serialized once and consumed by the script half.
type injectionPayload struct {
	UserAgent   string         `json:"userAgent"`
	AppVersion  string         `json:"appVersion"`
	Platform    string         `json:"platform"`
	Vendor      string         `json:"vendor"`
	VendorSub   string         `json:"vendorSub"`
	ProductSub  string         `json:"productSub"`
	ProductName string         `json:"productName"`
	Product     string         `json:"product"`
	Language    string         `json:"language"`
	Languages   []string       `json:"languages"`
	HWConc      int            `json:"hwConc"`
	DeviceMem   int            `json:"deviceMemory"`
	MaxTouch    int            `json:"maxTouchPoints"`
	UserAgentD  map[string]any `json:"userAgentData"`
	Connection  map[string]any `json:"connection"`

	Screen map[string]any `json:"screen"`

	InnerW  int     `json:"innerW"`
	InnerH  int     `json:"innerH"`
	OuterW  int     `json:"outerW"`
	OuterH  int     `json:"outerH"`
	DPR     float64 `json:"dpr"`
	ScreenX int     `json:"screenX"`
	ScreenY int     `json:"screenY"`

	WebGLParams map[string]string `json:"webglParams"`
}

// BuildInjectionJS renders the injection script for a fingerprint.
func BuildInjectionJS(fp *api.Fingerprint) (string, error) {
	if fp == nil {
		return "", fmt.Errorf("executor: fingerprint required")
	}
	p := buildPayload(fp)
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("executor: marshal injection payload: %w", err)
	}
	return fmt.Sprintf(injectionTemplate, data), nil
}

func buildPayload(fp *api.Fingerprint) injectionPayload {
	nav := fp.Navigator
	ua, _ := nav["userAgent"].(string)
	appVersion, _ := nav["appVersion"].(string)
	if appVersion == "" && len(ua) > len("Mozilla/") {
		appVersion = ua[len("Mozilla/"):]
	}
	platform, _ := nav["platform"].(string)
	if platform == "" {
		platform = fp.OS.Platform
	}
	vendor, _ := nav["vendor"].(string)
	vendorSub, _ := nav["vendorSub"].(string)
	productSub, _ := nav["productSub"].(string)
	productName, _ := nav["productName"].(string)
	product, _ := nav["product"].(string)
	language, _ := nav["language"].(string)
	languages := fp.Languages
	if len(languages) == 0 {
		if l, ok := nav["languages"].([]string); ok {
			languages = l
		}
	}
	hw := intFromAny(nav["hardwareConcurrency"])
	mem := intFromAny(nav["deviceMemory"])
	touch := intFromAny(nav["maxTouchPoints"])
	uad, _ := nav["userAgentData"].(map[string]any)
	conn, _ := nav["connection"].(map[string]any)

	// Screen: pass through the fp.Screen map (already the shape the V8 env
	// injects) minus non-serializable entries.
	screen := map[string]any{}
	for k, v := range fp.Screen {
		switch v.(type) {
		case int, int32, int64, float64, string, bool, map[string]any:
			screen[k] = v
		}
	}

	// WebGL identity parameters: string-keyed for JSON, consumed by the
	// getParameter shim.
	params := map[string]string{}
	for k, v := range fp.WebGL.Params {
		params[fmt.Sprintf("%d", k)] = v
	}
	// The vendor/renderer identity constants must always be present even if
	// the params map lacks them (buildWebGL guarantees them, defensive here).
	if _, ok := params["37445"]; !ok {
		params["37445"] = fp.WebGL.Vendor
	}
	if _, ok := params["37446"]; !ok {
		params["37446"] = fp.WebGL.Renderer
	}

	w := fp.Window
	return injectionPayload{
		UserAgent:   ua,
		AppVersion:  appVersion,
		Platform:    platform,
		Vendor:      vendor,
		VendorSub:   vendorSub,
		ProductSub:  productSub,
		ProductName: productName,
		Product:     product,
		Language:    language,
		Languages:   languages,
		HWConc:      hw,
		DeviceMem:   mem,
		MaxTouch:    touch,
		UserAgentD:  uad,
		Connection:  conn,
		Screen:      screen,
		InnerW:      w.InnerWidth,
		InnerH:      w.InnerHeight,
		OuterW:      w.OuterWidth,
		OuterH:      w.OuterHeight,
		DPR:         w.DevicePixelRatio,
		ScreenX:     w.ScreenX,
		ScreenY:     w.ScreenY,
		WebGLParams: params,
	}
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// injectionTemplate is the script half. It runs before any page script
// (Page.addScriptToEvaluateOnNewDocument) and never throws: every override is
// individually guarded. %s receives the JSON payload.
const injectionTemplate = `(function () {
  'use strict';
  var P = %s;

  function def(obj, prop, val) {
    try {
      Object.defineProperty(obj, prop, { value: val, configurable: true, writable: true });
    } catch (e) { /* keep native value */ }
  }

  // --- navigator identity -------------------------------------------------
  // userAgent itself comes from Emulation.setUserAgentOverride (CDP-native, so
  // HTTP headers and iframes agree); the derived/extra props are pinned here.
  var N = navigator;
  def(N, 'appVersion', P.appVersion);
  def(N, 'platform', P.platform);
  def(N, 'vendor', P.vendor);
  def(N, 'vendorSub', P.vendorSub);
  def(N, 'productSub', P.productSub);
  def(N, 'productName', P.productName);
  def(N, 'product', P.product);
  def(N, 'language', P.language);
  def(N, 'languages', P.languages);
  def(N, 'hardwareConcurrency', P.hwConc);
  def(N, 'deviceMemory', P.deviceMemory);
  def(N, 'maxTouchPoints', P.maxTouchPoints);
  // Non-Chromium profiles have no userAgentData (mirrors the V8 env's
  // browser adjustments): pin it to undefined instead of leaking Chrome's
  // native one.
  def(N, 'userAgentData', P.userAgentData === null ? undefined : P.userAgentData);
  if (P.connection) { try { def(N, 'connection', P.connection); } catch (e) {} }

  // --- screen geometry ----------------------------------------------------
  var S = screen;
  def(S, 'width', P.screen.width);
  def(S, 'height', P.screen.height);
  def(S, 'availWidth', P.screen.availWidth);
  def(S, 'availHeight', P.screen.availHeight);
  def(S, 'colorDepth', P.screen.colorDepth);
  def(S, 'pixelDepth', P.screen.pixelDepth);
  if (P.screen.availLeft !== undefined) { def(S, 'availLeft', P.screen.availLeft); }
  if (P.screen.availTop !== undefined) { def(S, 'availTop', P.screen.availTop); }
  if (P.screen.orientation) { def(S, 'orientation', P.screen.orientation); }

  // --- window geometry ----------------------------------------------------
  def(window, 'innerWidth', P.innerW);
  def(window, 'innerHeight', P.innerH);
  def(window, 'outerWidth', P.outerW);
  def(window, 'outerHeight', P.outerH);
  def(window, 'devicePixelRatio', P.dpr);
  def(window, 'screenX', P.screenX);
  def(window, 'screenY', P.screenY);

  // --- WebGL identity strings --------------------------------------------
  // Only the identity constants answer from the profile; every other
  // getParameter call reaches the REAL GL driver — the Chrome executor's
  // whole reason to exist.
  var WEBGL_PARAMS = P.webglParams;
  function patchGetParameter(proto) {
    if (!proto || !proto.getParameter) { return; }
    var orig = proto.getParameter;
    try {
      Object.defineProperty(proto, 'getParameter', {
        value: function (pname) {
          var v = WEBGL_PARAMS[String(pname)];
          if (v !== undefined) { return v; }
          return orig.call(this, pname);
        },
        configurable: true,
        writable: true
      });
    } catch (e) { /* leave native */ }
  }
  if (typeof WebGLRenderingContext !== 'undefined') {
    patchGetParameter(WebGLRenderingContext.prototype);
  }
  if (typeof WebGL2RenderingContext !== 'undefined') {
    patchGetParameter(WebGL2RenderingContext.prototype);
  }
})();`
