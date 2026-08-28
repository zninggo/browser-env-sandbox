package sandbox

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/tommie/v8go"

	"github.com/zninggo/bes/pkg/api"
)

// EnvBuilder constructs browser environment objects on the ObjectTemplate
// (pre-context phase). Uses Go-native v8go injection — harder to detect than
// JS-side mocks because the objects are created in Go, not in JavaScript.
type EnvBuilder struct {
	iso         *v8go.Isolate
	global      *v8go.ObjectTemplate
	fp          *api.Fingerprint
	location    string
	cookieStore *CookieStore
	timerMgr    *TimerManager
	consoleSink ConsoleSink
	netHandler  NetHandler
}

// Build injects all browser globals into the ObjectTemplate.
func (b *EnvBuilder) Build() {
	b.injectNavigator()
	b.injectScreen()
	b.injectLocation()
	b.injectDocument()
	b.injectWindow()
	b.injectChrome()
	b.injectStorage()
	b.injectPerformance()
	b.injectCrypto()
	b.injectTimers()
	b.injectConsole()
	b.injectEventClasses()
	b.injectMutationObserver()
}

func (b *EnvBuilder) injectNavigator() {
	nav := v8go.NewObjectTemplate(b.iso)
	for key, val := range b.fp.Navigator {
		if key == "webdriver" || key == "toString" {
			continue // set explicitly below
		}
		b.setTemplateValue(nav, key, val)
	}
	// Constraint: webdriver MUST be false (ReadOnly)
	nav.Set("webdriver", false, v8go.ReadOnly)
	// Constraint: navigator toString
	nav.Set("toString", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, "[object Navigator]")
		return v
	}))
	b.global.Set("navigator", nav)
}

func (b *EnvBuilder) injectScreen() {
	screen := v8go.NewObjectTemplate(b.iso)
	for key, val := range b.fp.Screen {
		b.setTemplateValue(screen, key, val)
	}
	b.global.Set("screen", screen)
}

func (b *EnvBuilder) injectLocation() {
	u, err := url.Parse(b.location)
	if err != nil {
		u = &url.URL{Scheme: "https", Host: "example.com", Path: "/"}
	}
	loc := v8go.NewObjectTemplate(b.iso)
	loc.Set("href", u.String())
	loc.Set("origin", u.Scheme+"://"+u.Host)
	loc.Set("protocol", u.Scheme+":")
	loc.Set("host", u.Host)
	loc.Set("hostname", u.Hostname())
	loc.Set("port", u.Port())
	loc.Set("pathname", u.Path)
	loc.Set("search", u.RawQuery)
	loc.Set("hash", u.Fragment)
	// Methods (no-ops)
	loc.Set("assign", v8go.NewFunctionTemplate(b.iso, noopCallback))
	loc.Set("replace", v8go.NewFunctionTemplate(b.iso, noopCallback))
	loc.Set("reload", v8go.NewFunctionTemplate(b.iso, noopCallback))
	b.global.Set("location", loc)
}

func (b *EnvBuilder) injectDocument() {
	doc := v8go.NewObjectTemplate(b.iso)
	doc.Set("URL", b.location)
	doc.Set("documentURI", b.location)
	doc.Set("baseURI", b.location)
	doc.Set("referrer", "")
	u, _ := url.Parse(b.location)
	if u != nil {
		doc.Set("domain", u.Hostname())
		doc.Set("origin", u.Scheme+"://"+u.Host)
	}
	doc.Set("title", "")
	doc.Set("readyState", "complete")
	doc.Set("visibilityState", "visible")
	doc.Set("characterSet", "UTF-8")
	doc.Set("charset", "UTF-8")
	doc.Set("contentType", "text/html")
	doc.Set("compatMode", "CSS1Compat")

	// Cookie: getter/setter via function templates
	// v8go doesn't support getters/setters on templates directly,
	// so we inject document.cookie as a function that JS code calls.
	// In PostContextBuilder we override with an accessor.
	doc.Set("getCookie", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, b.cookieStore.String())
		return v
	}))
	doc.Set("setCookie", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			b.cookieStore.SetRaw(info.Args()[0].String())
		}
		return nil
	}))

	// DOM methods
	doc.Set("createElement", v8go.NewFunctionTemplate(b.iso, b.createElementCallback))
	doc.Set("createElementNS", v8go.NewFunctionTemplate(b.iso, b.createElementCallback))
	doc.Set("createTextNode", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		text := ""
		if len(info.Args()) > 0 {
			text = info.Args()[0].String()
		}
		obj := v8go.NewObjectTemplate(b.iso)
		obj.Set("nodeType", int32(3))
		obj.Set("nodeValue", text)
		obj.Set("textContent", text)
		inst, _ := obj.NewInstance(info.Context())
		return inst.Value
	}))
	doc.Set("getElementById", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		return v8go.Null(b.iso)
	}))
	doc.Set("getElementsByTagName", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		arr, _ := v8go.NewValue(b.iso, "[]")
		return arr
	}))
	doc.Set("querySelector", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		return v8go.Null(b.iso)
	}))
	doc.Set("querySelectorAll", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		arr, _ := v8go.NewValue(b.iso, "[]")
		return arr
	}))
	doc.Set("addEventListener", v8go.NewFunctionTemplate(b.iso, noopCallback))
	doc.Set("removeEventListener", v8go.NewFunctionTemplate(b.iso, noopCallback))

	b.global.Set("document", doc)
}

func (b *EnvBuilder) injectWindow() {
	// Window dimensions from fingerprint
	w := b.fp.Window
	b.global.Set("innerWidth", int32(w.InnerWidth))
	b.global.Set("innerHeight", int32(w.InnerHeight))
	b.global.Set("outerWidth", int32(w.OuterWidth))
	b.global.Set("outerHeight", int32(w.OuterHeight))
	b.global.Set("devicePixelRatio", w.DevicePixelRatio)
	b.global.Set("screenX", int32(w.ScreenX))
	b.global.Set("screenY", int32(w.ScreenY))
	b.global.Set("scrollX", int32(0))
	b.global.Set("scrollY", int32(0))
	b.global.Set("pageXOffset", int32(0))
	b.global.Set("pageYOffset", int32(0))

	// History
	hist := v8go.NewObjectTemplate(b.iso)
	hist.Set("length", int32(1))
	hist.Set("state", v8go.Null(b.iso))
	hist.Set("scrollRestoration", "auto")
	hist.Set("back", v8go.NewFunctionTemplate(b.iso, noopCallback))
	hist.Set("forward", v8go.NewFunctionTemplate(b.iso, noopCallback))
	hist.Set("go", v8go.NewFunctionTemplate(b.iso, noopCallback))
	hist.Set("pushState", v8go.NewFunctionTemplate(b.iso, noopCallback))
	hist.Set("replaceState", v8go.NewFunctionTemplate(b.iso, noopCallback))
	b.global.Set("history", hist)
}

func (b *EnvBuilder) injectChrome() {
	// Constraint #10: Chrome feature object
	chrome := v8go.NewObjectTemplate(b.iso)
	runtime := v8go.NewObjectTemplate(b.iso)
	runtime.Set("onConnect", v8go.Null(b.iso))
	runtime.Set("onMessage", v8go.Null(b.iso))
	chrome.Set("runtime", runtime)
	chrome.Set("loadTimes", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, "{}")
		return v
	}))
	chrome.Set("csi", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, "{}")
		return v
	}))
	b.global.Set("chrome", chrome)
}

func (b *EnvBuilder) injectStorage() {
	// localStorage and sessionStorage as simple object templates
	// (full Storage API with getItem/setItem will be built post-context)
	ls := v8go.NewObjectTemplate(b.iso)
	b.global.Set("localStorage", ls)
	b.global.Set("sessionStorage", ls)
}

func (b *EnvBuilder) injectPerformance() {
	perf := v8go.NewObjectTemplate(b.iso)
	perf.Set("now", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, float64(0))
		return v
	}))
	b.global.Set("performance", perf)
}

func (b *EnvBuilder) injectCrypto() {
	// crypto.getRandomValues — Go callback generating real random bytes
	crypto := v8go.NewObjectTemplate(b.iso)
	crypto.Set("getRandomValues", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		// Return the input array unchanged (real impl would fill it)
		if len(info.Args()) > 0 {
			return info.Args()[0]
		}
		return v8go.Null(b.iso)
	}))
	b.global.Set("crypto", crypto)
}

func (b *EnvBuilder) injectTimers() {
	b.global.Set("setTimeout", v8go.NewFunctionTemplate(b.iso, b.timerMgr.SetTimeoutCallback(b.iso)))
	b.global.Set("clearTimeout", v8go.NewFunctionTemplate(b.iso, b.timerMgr.ClearTimeoutCallback(b.iso)))
	b.global.Set("setInterval", v8go.NewFunctionTemplate(b.iso, b.timerMgr.SetIntervalCallback(b.iso)))
	b.global.Set("clearInterval", v8go.NewFunctionTemplate(b.iso, b.timerMgr.ClearTimeoutCallback(b.iso)))
	b.global.Set("requestAnimationFrame", v8go.NewFunctionTemplate(b.iso, b.timerMgr.RAFCallback(b.iso)))
	b.global.Set("cancelAnimationFrame", v8go.NewFunctionTemplate(b.iso, b.timerMgr.ClearTimeoutCallback(b.iso)))
	b.global.Set("queueMicrotask", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		// Queue as microtask — v8go has PerformMicrotaskCheckpoint
		if len(info.Args()) > 0 && info.Args()[0].IsFunction() {
			fn, _ := info.Args()[0].AsFunction()
			fn.Call(info.Context().Global())
		}
		return nil
	}))
}

func (b *EnvBuilder) injectConsole() {
	console := v8go.NewObjectTemplate(b.iso)
	for _, method := range []string{"log", "debug", "info", "warn", "error"} {
		m := method // capture
		console.Set(m, v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			args := info.Args()
			parts := make([]string, len(args))
			for i, a := range args {
				parts[i] = a.String()
			}
			msg := strings.Join(parts, " ")
			if b.consoleSink != nil {
				b.consoleSink.Write(m, msg)
			} else {
				fmt.Printf("[console.%s] %s\n", m, msg)
			}
			return nil
		}))
	}
	console.Set("dir", v8go.NewFunctionTemplate(b.iso, noopCallback))
	console.Set("trace", v8go.NewFunctionTemplate(b.iso, noopCallback))
	console.Set("group", v8go.NewFunctionTemplate(b.iso, noopCallback))
	console.Set("groupEnd", v8go.NewFunctionTemplate(b.iso, noopCallback))
	b.global.Set("console", console)
}

func (b *EnvBuilder) injectEventClasses() {
	// Event, CustomEvent, EventTarget
	b.global.Set("Event", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		obj := v8go.NewObjectTemplate(b.iso)
		eventType := ""
		if len(info.Args()) > 0 {
			eventType = info.Args()[0].String()
		}
		obj.Set("type", eventType)
		obj.Set("bubbles", false)
		obj.Set("cancelable", false)
		obj.Set("timeStamp", float64(0))
		inst, _ := obj.NewInstance(info.Context())
		return inst.Value
	}))
	b.global.Set("EventTarget", v8go.NewFunctionTemplate(b.iso, noopCallbackReturningObj))
}

func (b *EnvBuilder) injectMutationObserver() {
	b.global.Set("MutationObserver", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		obj := v8go.NewObjectTemplate(b.iso)
		obj.Set("observe", v8go.NewFunctionTemplate(b.iso, noopCallback))
		obj.Set("disconnect", v8go.NewFunctionTemplate(b.iso, noopCallback))
		obj.Set("takeRecords", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			v, _ := v8go.NewValue(b.iso, "[]")
			return v
		}))
		inst, _ := obj.NewInstance(info.Context())
		return inst.Value
	}))
	b.global.Set("MessageChannel", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		obj := v8go.NewObjectTemplate(b.iso)
		port := v8go.NewObjectTemplate(b.iso)
		port.Set("onmessage", v8go.Null(b.iso))
		port.Set("postMessage", v8go.NewFunctionTemplate(b.iso, noopCallback))
		port.Set("close", v8go.NewFunctionTemplate(b.iso, noopCallback))
		obj.Set("port1", port)
		obj.Set("port2", port)
		inst, _ := obj.NewInstance(info.Context())
		return inst.Value
	}))
}

func (b *EnvBuilder) createElementCallback(info *v8go.FunctionCallbackInfo) *v8go.Value {
	tagName := "div"
	if len(info.Args()) > 0 {
		tagName = strings.ToLower(info.Args()[0].String())
	}
	obj := v8go.NewObjectTemplate(b.iso)
	obj.Set("tagName", strings.ToUpper(tagName))
	obj.Set("nodeName", strings.ToUpper(tagName))
	obj.Set("nodeType", int32(1))
	obj.Set("id", "")
	obj.Set("className", "")
	obj.Set("innerHTML", "")
	obj.Set("textContent", "")
	obj.Set("style", v8go.NewObjectTemplate(b.iso))

	// Methods
	obj.Set("setAttribute", v8go.NewFunctionTemplate(b.iso, noopCallback))
	obj.Set("getAttribute", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		return v8go.Null(b.iso)
	}))
	obj.Set("removeAttribute", v8go.NewFunctionTemplate(b.iso, noopCallback))
	obj.Set("hasAttribute", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, false)
		return v
	}))
	obj.Set("appendChild", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			return info.Args()[0]
		}
		return v8go.Null(b.iso)
	}))
	obj.Set("removeChild", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			return info.Args()[0]
		}
		return v8go.Null(b.iso)
	}))
	obj.Set("addEventListener", v8go.NewFunctionTemplate(b.iso, noopCallback))
	obj.Set("removeEventListener", v8go.NewFunctionTemplate(b.iso, noopCallback))
	obj.Set("getBoundingClientRect", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(b.iso, `{"x":0,"y":0,"width":0,"height":0,"top":0,"right":0,"bottom":0,"left":0}`)
		return v
	}))

	// Canvas support
	if tagName == "canvas" {
		obj.Set("getContext", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			ctxType := ""
			if len(info.Args()) > 0 {
				ctxType = info.Args()[0].String()
			}
			ctxObj := v8go.NewObjectTemplate(b.iso)
			if ctxType == "2d" {
				injectCanvas2D(ctxObj, b.iso, b.fp)
			} else if strings.HasPrefix(ctxType, "webgl") {
				injectWebGL(ctxObj, b.iso, b.fp)
			}
			inst, _ := ctxObj.NewInstance(info.Context())
			return inst.Value
		}))
		obj.Set("toDataURL", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			// Deterministic dataURL based on the fingerprint's canvas hash.
			// This keeps toDataURL output consistent with fp.Canvas.ToDataURLHash.
			dataURL := "data:image/png;base64," + b.fp.Canvas.ToDataURLHash
			v, _ := v8go.NewValue(b.iso, dataURL)
			return v
		}))
	}

	// Script element special
	if tagName == "script" {
		obj.Set("text", "")
		obj.Set("src", "")
		obj.Set("type", "")
	}

	inst, _ := obj.NewInstance(info.Context())
	return inst.Value
}

// --- Post-context builder (needs actual context, not template) ---

type PostContextBuilder struct {
	iso         *v8go.Isolate
	ctx         *v8go.Context
	global      *v8go.Object
	fp          *api.Fingerprint
	location    string
	cookieStore *CookieStore
	timerMgr    *TimerManager
	netHandler  NetHandler
	opts        api.SessionOptions
}

func (p *PostContextBuilder) Build() {
	// window = self = top = parent = frames = globalThis
	// Constraint #4: direct references, NOT Proxy
	p.global.Set("window", p.global)
	p.global.Set("self", p.global)
	p.global.Set("top", p.global)
	p.global.Set("parent", p.global)
	p.global.Set("frames", p.global)
	p.global.Set("globalThis", p.global)

	// Constraint #6: Node痕迹抹除 — in v8go these don't exist by default,
	// but explicitly set to undefined for safety
	p.global.Set("Buffer", v8go.Undefined(p.iso))
	p.global.Set("process", v8go.Undefined(p.iso))
	// Node.js traces: deleted in env_shim_part3.js (they must not appear as
	// own properties on window — real browsers don't have them at all).

	// document.cookie accessor: override the template placeholder
	// with a proper get/set via JS shim
	cookieShim := fmt.Sprintf(`
		Object.defineProperty(document, 'cookie', {
			get: function() { return document.getCookie(); },
			set: function(v) { document.setCookie(v); },
			configurable: true
		});
	`)
	p.ctx.RunScript(cookieShim, "cookie-shim.js")

	// Inject complex navigator properties that can't be set via ObjectTemplate
	// (v8go Set only accepts primitives, FunctionTemplate, ObjectTemplate)
	p.injectComplexNavigator()
	p.injectTimezone()

	// Inject permissions API (navigator.permissions)
	p.injectPermissions()

	// Inject fetch + XMLHttpRequest
	p.injectFetchXHR()

	// Inject comprehensive env shim (missing browser APIs)
	// Each part runs independently so a parse error in one part doesn't
	// block the others — all 5 parts are self-contained IIFEs.
	parts := envShimParts()
	for i, part := range parts {
		name := fmt.Sprintf("env-shim-part%d.js", i+1)
		if _, err := p.ctx.RunScript(part, name); err != nil {
			fmt.Printf("[sandbox] env shim %s warning: %v\n", name, err)
		}
	}
}

// --- helpers ---

func (b *EnvBuilder) setTemplateValue(tmpl *v8go.ObjectTemplate, key string, val interface{}) {
	switch v := val.(type) {
	case string:
		tmpl.Set(key, v)
	case int:
		tmpl.Set(key, int32(v))
	case int32:
		tmpl.Set(key, v)
	case float64:
		tmpl.Set(key, v)
	case bool:
		tmpl.Set(key, v)
	case nil:
		// skip nil values — they'll be undefined naturally
	default:
		// For complex types (maps, slices), skip on template;
		// will be injected via JS post-context
		_ = v
	}
}

func noopCallback(info *v8go.FunctionCallbackInfo) *v8go.Value {
	return nil
}

func noopCallbackReturningObj(info *v8go.FunctionCallbackInfo) *v8go.Value {
	obj := v8go.NewObjectTemplate(info.Context().Isolate())
	inst, _ := obj.NewInstance(info.Context())
	return inst.Value
}

func injectCanvas2D(ctx *v8go.ObjectTemplate, iso *v8go.Isolate, fp *api.Fingerprint) {
	ctx.Set("fillStyle", "#000000")
	ctx.Set("strokeStyle", "#000000")
	ctx.Set("lineWidth", float64(1))
	ctx.Set("font", "10px sans-serif")
	for _, m := range []string{"fillRect", "strokeRect", "clearRect", "beginPath", "closePath",
		"moveTo", "lineTo", "arc", "arcTo", "rect", "fill", "stroke", "clip", "drawImage",
		"putImageData", "translate", "rotate", "scale", "transform", "setTransform", "save", "restore"} {
		ctx.Set(m, v8go.NewFunctionTemplate(iso, noopCallback))
	}
	ctx.Set("getImageData", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(iso, `{"data":[0,0,0,0],"width":1,"height":1}`)
		return v
	}))
	ctx.Set("createImageData", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		v, _ := v8go.NewValue(iso, `{"data":[0,0,0,0],"width":1,"height":1}`)
		return v
	}))
	ctx.Set("measureText", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		text := ""
		if len(info.Args()) > 0 {
			text = info.Args()[0].String()
		}
		// Use fingerprint's measureText data if available, otherwise fallback
		// to a deterministic width based on text length.
		width := float64(len(text) * 6)
		if fp != nil && fp.Canvas.MeasureText != nil {
			if w, ok := fp.Canvas.MeasureText["measureText_width"]; ok {
				// Scale by text length relative to a baseline of 10 chars
				width = w * float64(len(text)) / 10.0
			}
		}
		v, _ := v8go.NewValue(iso, fmt.Sprintf(`{"width":%g}`, width))
		return v
	}))
}

func injectWebGL(ctx *v8go.ObjectTemplate, iso *v8go.Isolate, fp *api.Fingerprint) {
	ctx.Set("getParameter", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		if len(info.Args()) > 0 {
			pname := info.Args()[0].Int32()
			if val, ok := fp.WebGL.Params[pname]; ok {
				v, _ := v8go.NewValue(iso, val)
				return v
			}
		}
		return v8go.Null(iso)
	}))
	ctx.Set("getExtension", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		obj := v8go.NewObjectTemplate(iso)
		inst, _ := obj.NewInstance(info.Context())
		return inst.Value
	}))
	ctx.Set("getSupportedExtensions", v8go.NewFunctionTemplate(iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		exts := ""
		for i, e := range fp.WebGL.Extensions {
			if i > 0 {
				exts += ","
			}
			exts += fmt.Sprintf("%q", e)
		}
		v, _ := v8go.NewValue(iso, "["+exts+"]")
		return v
	}))
	for _, m := range []string{"createShader", "shaderSource", "compileShader",
		"createProgram", "attachShader", "linkProgram", "useProgram",
		"createBuffer", "bindBuffer", "bufferData"} {
		ctx.Set(m, v8go.NewFunctionTemplate(iso, noopCallback))
	}
}

// --- PostContext complex injections ---

// injectComplexNavigator injects map/slice navigator properties via JS
// (v8go ObjectTemplate.Set can't handle Go maps/slices directly)
func (p *PostContextBuilder) injectComplexNavigator() {
	// navigator.languages
	langs := p.fp.Languages
	langArr := "["
	for i, l := range langs {
		if i > 0 {
			langArr += ","
		}
		langArr += fmt.Sprintf("%q", l)
	}
	langArr += "]"

	// navigator.userAgentData
	uad := p.fp.Navigator["userAgentData"]
	uadJSON := "{}"
	if uad != nil {
		if m, ok := uad.(map[string]any); ok {
			// Build brands array
			brandsStr := "[]"
			if brands, ok := m["brands"].([]map[string]any); ok && len(brands) > 0 {
				parts := make([]string, 0, len(brands))
				for _, b := range brands {
					parts = append(parts, fmt.Sprintf(`{"brand":%q,"version":%q}`, b["brand"], b["version"]))
				}
				brandsStr = "[" + strings.Join(parts, ",") + "]"
			}
			platform, _ := m["platform"].(string)
			mobile, _ := m["mobile"].(bool)
			uadJSON = fmt.Sprintf(`{"brands":%s,"mobile":%t,"platform":%q}`, brandsStr, mobile, platform)
		}
	}

	// navigator.connection
	connJSON := "{}"
	if conn, ok := p.fp.Navigator["connection"].(map[string]any); ok {
		connJSON = fmt.Sprintf(`{"effectiveType":%q,"rtt":%v,"downlink":%v,"saveData":false}`,
			conn["effectiveType"], conn["rtt"], conn["downlink"])
	}

	// navigator.language must equal languages[0] (browser invariant)
	langFirst := "en"
	if len(langs) > 0 {
		langFirst = langs[0]
	}

	js := fmt.Sprintf(`
		(function(){
			Object.defineProperty(navigator, 'language', {value: %q, configurable: true, writable: true});
			Object.defineProperty(navigator, 'languages', {value: %s, configurable: true, writable: true});
			Object.defineProperty(navigator, 'userAgentData', {value: %s, configurable: true, writable: true});
			Object.defineProperty(navigator, 'connection', {value: %s, configurable: true, writable: true});
		})();
	`, langFirst, langArr, uadJSON, connJSON)
	p.ctx.RunScript(js, "nav-complex.js")
}

// injectTimezone overrides Intl.DateTimeFormat and Date timezone-facing
// methods so the sandbox reports the fingerprint's timezone instead of the
// host's (UTC in containers). Offsets come from a small per-zone table
// covering the KB's six zones, including DST (northern-hemisphere
// approximation) for NY/London.
func (p *PostContextBuilder) injectTimezone() {
	tz := p.fp.Timezone
	if tz == "" {
		return
	}
	// Base offsets in minutes (positive = east of UTC) and DST start/end
	// (month indexes, 0-based) for zones that observe DST.
	type zoneInfo struct {
		offsetMin    int
		hasDST       bool
		dstOffsetMin int
	}
	zones := map[string]zoneInfo{
		"Asia/Shanghai":    {offsetMin: 480},
		"Asia/Tokyo":       {offsetMin: 540},
		"Asia/Seoul":       {offsetMin: 540},
		"Asia/Singapore":   {offsetMin: 480},
		"America/New_York": {offsetMin: -300, hasDST: true, dstOffsetMin: -240},
		"Europe/London":    {offsetMin: 0, hasDST: true, dstOffsetMin: -60},
	}
	zi, ok := zones[tz]
	if !ok {
		return
	}
	js := fmt.Sprintf(`
		(function(){
			var TZ = %q, BASE = %d, HAS_DST = %v, DST = %d;
			function localOffsetMinutes(tsMin) {
				if (!HAS_DST) return BASE;
				// Approximate DST: northern hemisphere, 2nd Sun Mar 02:00 → 1st Sun Nov 02:00 (local)
				var d = new Date(tsMin * 60000);
				// Work in UTC terms of the month boundaries; approximation is
				// acceptable for fingerprint reporting granularity.
				var m = d.getUTCMonth();
				var dst = (m > 2 && m < 10);
				return dst ? DST : BASE;
			}
			var OrigDTF = Intl.DateTimeFormat;
			var NativeTZ = OrigDTF().resolvedOptions().timeZone;
			Intl.DateTimeFormat = function(locale, options) {
				options = options || {};
				if (!options.timeZone) { options.timeZone = TZ; }
				return new OrigDTF(locale, options);
			};
			Intl.DateTimeFormat.prototype = OrigDTF.prototype;
			Object.getOwnPropertyNames(OrigDTF).forEach(function(k){
				try { Intl.DateTimeFormat[k] = OrigDTF[k]; } catch(e) {}
			});
			Intl.DateTimeFormat.supportedLocalesOf = OrigDTF.supportedLocalesOf;
			var origGetTimezoneOffset = Date.prototype.getTimezoneOffset;
			Date.prototype.getTimezoneOffset = function() {
				return -localOffsetMinutes(this.getTime() / 60000);
			};
			// Date.prototype.toString local fields stay UTC-based (acceptable
			// approximation); timezone name reporting flows through the
			// wrapped Intl.DateTimeFormat above.
		})();
	`, tz, zi.offsetMin, zi.hasDST, zi.dstOffsetMin)
	p.ctx.RunScript(js, "timezone.js")
}

// injectPermissions adds navigator.permissions API
func (p *PostContextBuilder) injectPermissions() {
	js := `
		(function(){
			var permissions = {
				query: function(desc) {
					return Promise.resolve({state: 'prompt', name: desc.name});
				}
			};
			Object.defineProperty(navigator, 'permissions', {value: permissions, configurable: true});
		})();
	`
	p.ctx.RunScript(js, "permissions.js")
}

// injectFetchXHR adds fetch and XMLHttpRequest to the sandbox.
// When a NetHandler is configured, XHR/fetch make real HTTP requests via a
// Go callback. When no handler is set, they fall back to stubs (for
// offline/signing-only use cases).
func (p *PostContextBuilder) injectFetchXHR() {
	if p.netHandler != nil {
		p.injectRealFetchXHR()
		return
	}
	// Stub implementation (no network — for pure JS execution / signing)
	stubJS := `
		(function(){
			window.fetch = function(resource, options) {
				var mockResp = {
					ok: true, status: 200,
					headers: { get: function() { return null; } },
					json: function() { return Promise.resolve({}); },
					text: function() { return Promise.resolve(''); },
					arrayBuffer: function() { return Promise.resolve(new ArrayBuffer(0)); }
				};
				return Promise.resolve(mockResp);
			};
			window.XMLHttpRequest = function() {
				this.readyState = 0; this.status = 0;
				this.responseText = ''; this.response = '';
				this._method = 'GET'; this._url = ''; this._headers = {};
			};
			window.XMLHttpRequest.prototype.open = function(method, url) {
				this._method = method; this._url = url; this.readyState = 1;
			};
			window.XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
				this._headers[name] = value;
			};
			window.XMLHttpRequest.prototype.send = function(body) {};
			window.XMLHttpRequest.prototype.getAllResponseHeaders = function() { return ''; };
			window.XMLHttpRequest.prototype.getResponseHeader = function(name) { return null; };
			window.XMLHttpRequest.prototype.abort = function() {};
		})();
	`
	p.ctx.RunScript(stubJS, "fetch-xhr.js")
}

// injectRealFetchXHR wires XHR/fetch to a Go FunctionCallback (__besNetRequest)
// that calls the session's NetHandler to make real HTTP requests.
func (p *PostContextBuilder) injectRealFetchXHR() {
	iso := p.iso
	handler := p.netHandler
	cookies := p.cookieStore
	location := p.location
	userAgent := ""
	if p.fp != nil {
		if ua, ok := p.fp.Navigator["userAgent"].(string); ok {
			userAgent = ua
		}
	}
	// Accept-Language derived from the fingerprint languages, so header and
	// navigator.languages stay consistent (a common server-side check).
	// Chrome format: "zh-CN,zh;q=0.9,en;q=0.8" — first language full, the
	// rest q-decayed starting at 0.9.
	acceptLanguage := ""
	if p.fp != nil && len(p.fp.Languages) > 0 {
		parts := make([]string, 0, len(p.fp.Languages))
		for i, lang := range p.fp.Languages {
			if i == 0 {
				parts = append(parts, lang)
			} else {
				parts = append(parts, fmt.Sprintf("%s;q=0.%d", lang, 10-i))
			}
		}
		acceptLanguage = strings.Join(parts, ",")
	}
	// Default referer derived from the session location, as a browser would
	// send for a subresource request: full URL same-origin, origin cross-origin
	// (strict-origin-when-cross-origin). Empty when no valid location.
	defaultRefererFull, defaultRefererOrigin := "", ""
	if u, err := url.Parse(location); err == nil && u.Host != "" {
		defaultRefererFull = u.Scheme + "://" + u.Host + u.Path
		if u.RawQuery != "" {
			defaultRefererFull += "?" + u.RawQuery
		}
		defaultRefererOrigin = u.Scheme + "://" + u.Host + "/"
	}

	// Session-level defaults from SessionOptions (bridge /api/session fields).
	// Precedence: request-level _bes_* override > explicit session field >
	// browser-default derivation (full URL same-origin / origin cross-origin).
	sessReferer := p.opts.Referer
	sessOrigin := p.opts.Origin
	sessUA := p.opts.UserAgent
	sessExtra := p.opts.ExtraHeaders

	// Go callback: __besNetRequest(method, url, headersJSON, body) → responseJSON
	callback := func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := info.Args()
		if len(args) < 2 {
			v, _ := v8go.NewValue(iso, `{"status":0,"body":"","headers":{}}`)
			return v
		}
		method := args[0].String()
		urlStr := args[1].String()
		headersJSON := "{}"
		if len(args) > 2 {
			headersJSON = args[2].String()
		}
		body := ""
		if len(args) > 3 {
			body = args[3].String()
		}

		// Parse headers
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			headers = make(map[string]string)
		}

		// Resolve request URL parts for cookie scoping and referer policy.
		u, err := url.Parse(urlStr)
		if err != nil || u.Host == "" {
			u = nil
		}

		// Request-level overrides via reserved _bes_* headers (stripped before
		// sending — the target server never sees them). These let a single
		// eval'd request override the session-level defaults without touching
		// subsequent requests.
		reqReferer := popReservedHeader(headers, "_bes_referer")
		reqOrigin := popReservedHeader(headers, "_bes_origin")
		reqUA := popReservedHeader(headers, "_bes_user_agent")

		// Inject default headers. Precedence: manually set (real) headers >
		// request-level _bes_* override > session-level field > derivation.
		if u != nil {
			scheme := strings.ToLower(u.Scheme)
			reqPath := u.Path
			if reqPath == "" {
				reqPath = "/"
			}

			// Cookies per RFC 6265 §5.4, scoped to this request's
			// scheme/host/path (domain match, path match, secure).
			if _, hasCookie := headerLookup(headers, "Cookie"); !hasCookie {
				if cookieStr := cookies.CookieHeaderFor(scheme, u.Hostname(), reqPath); cookieStr != "" {
					headers = setHeaderAbsent(headers, "Cookie", cookieStr)
				}
			}

			// Referer: request override > session field > browser-default
			// (full URL same-origin, origin cross-origin —
			// strict-origin-when-cross-origin).
			if _, hasReferer := headerLookup(headers, "Referer"); !hasReferer {
				ref := reqReferer
				if ref == "" {
					ref = sessReferer
				}
				if ref == "" && defaultRefererFull != "" {
					refOrigin := ""
					if lu, lerr := url.Parse(location); lerr == nil {
						refOrigin = strings.ToLower(lu.Scheme) + "://" + strings.ToLower(lu.Host)
					}
					reqOriginURL := scheme + "://" + strings.ToLower(u.Host)
					if refOrigin != "" && refOrigin != reqOriginURL {
						ref = defaultRefererOrigin
					} else {
						ref = defaultRefererFull
					}
				}
				if ref != "" {
					headers = setHeaderAbsent(headers, "Referer", ref)
				}
			}

			// Origin: request override > session field (no browser-default —
			// Chrome only sends Origin on CORS/non-GET, callers opt in).
			if _, hasOrigin := headerLookup(headers, "Origin"); !hasOrigin {
				origin := reqOrigin
				if origin == "" {
					origin = sessOrigin
				}
				if origin != "" {
					headers = setHeaderAbsent(headers, "Origin", origin)
				}
			}

			// User-Agent: request override > session field > fingerprint UA,
			// so TLS/UA/server-side checks stay aligned.
			if _, hasUA := headerLookup(headers, "User-Agent"); !hasUA {
				ua := reqUA
				if ua == "" {
					ua = sessUA
				}
				if ua == "" {
					ua = userAgent
				}
				if ua != "" {
					headers = setHeaderAbsent(headers, "User-Agent", ua)
				}
			}

			// Accept-Language: derived from the fingerprint languages (same
			// chain as navigator.languages), aligned with User-Agent so
			// language-consistency checks pass. netlayer backends have their
			// own hardcoded zh-CN fallback when this is absent.
			if _, hasAL := headerLookup(headers, "Accept-Language"); !hasAL && acceptLanguage != "" {
				headers = setHeaderAbsent(headers, "Accept-Language", acceptLanguage)
			}

			// Session-level extra headers: appended only when the request
			// does not already carry a value (manually set headers win).
			for k, v := range sessExtra {
				if _, has := headerLookup(headers, k); !has {
					headers[k] = v
				}
			}
		}

		// Make the real HTTP request via NetHandler
		resp, err := handler.Request(method, urlStr, headers, []byte(body))
		if err != nil {
			result, _ := json.Marshal(map[string]any{
				"status": 0, "body": "", "headers": map[string]string{},
				"error": err.Error(),
			})
			v, _ := v8go.NewValue(iso, string(result))
			return v
		}

		// Update cookie store from response Set-Cookie lines, scoped to the
		// response URL's host (host-only vs domain attribute per RFC 6265 §5.3).
		respHost := ""
		if u != nil {
			respHost = u.Hostname()
		}
		if len(resp.SetCookies) > 0 {
			cookies.ApplySetCookie(resp.SetCookies, respHost)
		} else if resp.Cookies != nil {
			for k, v := range resp.Cookies {
				cookies.Set(k, v, "/", "")
			}
		}

		// Binary-safe body transport: BodyB64 carries the raw bytes losslessly
		// across the Go→V8 boundary (JSON string). The plain body field gets
		// latin1 semantics (byte value == charCodeAt value) so legacy text()
		// readers see every byte; real UTF-8 text is served by response.text()
		// decoding from the same bytes.
		bodyOut := resp.Body
		if resp.BodyB64 != "" {
			if raw, err := base64.StdEncoding.DecodeString(resp.BodyB64); err == nil {
				var sb strings.Builder
				for _, b := range raw {
					sb.WriteByte(b)
				}
				bodyOut = sb.String()
			}
		}

		// Return JSON response
		result, _ := json.Marshal(map[string]any{
			"status":   resp.Status,
			"body":     bodyOut,
			"bodyB64":  resp.BodyB64,
			"headers":  resp.Headers,
		})
		v, _ := v8go.NewValue(iso, string(result))
		return v
	}

	// Register the Go callback as a global function
	fn := v8go.NewFunctionTemplate(iso, callback)
	fnVal := fn.GetFunction(p.ctx)
	p.global.Set("__besNetRequest", fnVal)

	// Real XHR + fetch implementation that calls __besNetRequest.
	//
	// Binary-safe by design: the Go callback returns the response body twice —
	// bodyB64 (base64 of the raw bytes, lossless) and body (latin1 semantics:
	// byte value == charCodeAt value). __besB64ToUint8Array decodes bodyB64
	// into a real Uint8Array so arrayBuffer()/blob()/XHR responseType all get
	// byte-exact data; text() re-decodes the bytes as UTF-8 per the WHATWG
	// spec (invalid sequences become U+FFFD, same as a real browser).
	realJS := `
		(function(){
			// __besB64ToUint8Array: shared base64 → Uint8Array decoder (std alphabet).
			window.__besB64ToUint8Array = function(b64) {
				var bin = atob(b64);
				var out = new Uint8Array(bin.length);
				for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
				return out;
			};

			// __besBytesToUTF8: Uint8Array → real UTF-8 string.
			// WHATWG fetch/TextDecoder semantics: malformed sequences → U+FFFD.
			window.__besBytesToUTF8 = function(bytes) {
				var s = '';
				for (var i = 0; i < bytes.length; ) {
					var b0 = bytes[i];
					if (b0 < 0x80) { s += String.fromCharCode(b0); i++; }
					else if (b0 >= 0xC2 && b0 < 0xE0 && i + 1 < bytes.length && (bytes[i+1] & 0xC0) === 0x80) {
						s += String.fromCharCode(((b0 & 0x1F) << 6) | (bytes[i+1] & 0x3F)); i += 2;
					}
					else if (b0 >= 0xE0 && b0 < 0xF0 && i + 2 < bytes.length && (bytes[i+1] & 0xC0) === 0x80 && (bytes[i+2] & 0xC0) === 0x80) {
						s += String.fromCharCode(((b0 & 0x0F) << 12) | ((bytes[i+1] & 0x3F) << 6) | (bytes[i+2] & 0x3F)); i += 3;
					}
					else if (b0 >= 0xF0 && b0 < 0xF5 && i + 3 < bytes.length && (bytes[i+1] & 0xC0) === 0x80 && (bytes[i+2] & 0xC0) === 0x80 && (bytes[i+3] & 0xC0) === 0x80) {
						var cp = ((b0 & 0x07) << 18) | ((bytes[i+1] & 0x3F) << 12) | ((bytes[i+2] & 0x3F) << 6) | (bytes[i+3] & 0x3F);
						cp -= 0x10000;
						s += String.fromCharCode(0xD800 + (cp >> 10), 0xDC00 + (cp & 0x3FF));
						i += 4;
					}
					else { s += '\uFFFD'; i++; }
				}
				return s;
			};

			// __besMakeResponse builds the fetch Response object from the
			// netlayer JSON. All readers share one set of decoded bytes.
			window.__besMakeResponse = function(r) {
				var bytes = r.bodyB64 ? __besB64ToUint8Array(r.bodyB64) : null;
				var headerObj = {
					get: function(name) {
						name = String(name).toLowerCase();
						for (var k in (r.headers || {})) {
							if (k.toLowerCase() === name) return r.headers[k];
						}
						return null;
					},
					has: function(name) { return this.get(name) !== null; }
				};
				return {
					ok: r.status >= 200 && r.status < 300,
					status: r.status,
					statusText: '',
					headers: headerObj,
					bodyUsed: false,
					text: function() {
						this.bodyUsed = true;
						// UTF-8 decode straight from the raw bytes (WHATWG semantics);
						// never via the latin1 string — that would double-mangle CJK.
						return Promise.resolve(bytes ? __besBytesToUTF8(bytes) : (r.body || ''));
					},
					json: function() {
						this.bodyUsed = true;
						var t = bytes ? __besBytesToUTF8(bytes) : (r.body || '');
						try { return Promise.resolve(JSON.parse(t || '{}')); }
						catch(e) { return Promise.reject(e); }
					},
					arrayBuffer: function() {
						this.bodyUsed = true;
						if (!bytes) return Promise.resolve(new ArrayBuffer(0));
						return Promise.resolve(bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength));
					},
					blob: function() {
						this.bodyUsed = true;
						return Promise.resolve(new Blob(bytes ? [bytes] : [], { type: headerObj.get('content-type') || '' }));
					},
					clone: function() { return this; }
				};
			};

			window.XMLHttpRequest = function() {
				this.readyState = 0;
				this.status = 0;
				this.statusText = '';
				this.responseText = '';
				this.response = '';
				this.responseType = '';
				this.responseText = '';
				this.onreadystatechange = null;
				this.onload = null;
				this.onerror = null;
				this.onabort = null;
				this.ontimeout = null;
				this.withCredentials = false;
				this.timeout = 0;
				this._method = 'GET';
				this._url = '';
				this._headers = {};
				this._responseHeaders = {};
			};
			window.XMLHttpRequest.prototype.open = function(method, url) {
				this._method = method;
				this._url = url;
				this.readyState = 1;
				if (this.onreadystatechange) this.onreadystatechange();
			};
			window.XMLHttpRequest.prototype.setRequestHeader = function(name, value) {
				this._headers[name] = value;
			};
			window.XMLHttpRequest.prototype.send = function(body) {
				var respJSON = __besNetRequest(this._method, this._url, JSON.stringify(this._headers), body || '');
				try {
					var r = JSON.parse(respJSON);
					this.status = r.status;
					this.statusText = '';
					this._responseHeaders = r.headers || {};
					// latin1 body: charCodeAt(i) == byte i (byte-exact, lossless).
					this.responseText = r.body || '';
					if (this.responseType === 'arraybuffer') {
						this.response = r.bodyB64 ? __besB64ToUint8Array(r.bodyB64).buffer : new ArrayBuffer(0);
					} else if (this.responseType === 'blob') {
						this.response = new Blob(r.bodyB64 ? [__besB64ToUint8Array(r.bodyB64)] : [], { type: this.getResponseHeader('Content-Type') || '' });
					} else if (this.responseType === 'json') {
						try { this.response = JSON.parse(r.body || 'null'); } catch(e) { this.response = null; }
					} else if (this.responseType === 'document') {
						this.response = (new DOMParser()).parseFromString(r.body || '', 'text/xml');
					} else {
						// '' (text) and 'text': the latin1 string.
						this.response = r.body || '';
					}
					this.readyState = 4;
					if (this.onreadystatechange) this.onreadystatechange();
					if (r.status > 0) {
						if (this.onload) this.onload();
					} else {
						if (this.onerror) this.onerror(new Error(r.error || 'network error'));
					}
				} catch(e) {
					this.status = 0;
					this.readyState = 4;
					if (this.onerror) this.onerror(e);
				}
			};
			window.XMLHttpRequest.prototype.getAllResponseHeaders = function() {
				var result = '';
				for (var k in this._responseHeaders) {
					result += k + ': ' + this._responseHeaders[k] + '\r\n';
				}
				return result;
			};
			window.XMLHttpRequest.prototype.getResponseHeader = function(name) {
				name = String(name).toLowerCase();
				for (var k in this._responseHeaders) {
					if (k.toLowerCase() === name) return this._responseHeaders[k];
				}
				return null;
			};
			window.XMLHttpRequest.prototype.overrideMimeType = function(mime) {
				this._overrideMime = mime;
			};
			window.XMLHttpRequest.prototype.abort = function() {};

			window.fetch = function(resource, options) {
				options = options || {};
				var method = options.method || 'GET';
				var urlStr = typeof resource === 'string' ? resource : (resource && resource.url || '');
				var headers = options.headers || {};
				var body = options.body || '';
				var respJSON = __besNetRequest(method, urlStr, JSON.stringify(headers), typeof body === 'string' ? body : '');
				return new Promise(function(resolve, reject) {
					try {
						resolve(__besMakeResponse(JSON.parse(respJSON)));
					} catch(e) { reject(e); }
				});
			};
		})();
	`
	p.ctx.RunScript(realJS, "fetch-xhr.js")
}

// headerLookup does a case-insensitive lookup of a header in a map whose keys
// may be in any case (JS callers usually send canonical names, but be lenient).
func headerLookup(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// setHeaderAbsent removes any existing entry equal to name (any case) and sets
// name=value with the given casing.
func setHeaderAbsent(headers map[string]string, name, value string) map[string]string {
	for k := range headers {
		if strings.EqualFold(k, name) {
			delete(headers, k)
		}
	}
	headers[name] = value
	return headers
}

// popReservedHeader removes the reserved header (any case) from the map and
// returns its value ("" when absent). Used for request-level _bes_* overrides
// which must never reach the wire.
func popReservedHeader(headers map[string]string, name string) string {
	v, ok := headerLookup(headers, name)
	if ok {
		for k := range headers {
			if strings.EqualFold(k, name) {
				delete(headers, k)
			}
		}
	}
	return v
}
