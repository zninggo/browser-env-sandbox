package sandbox

import (
	"fmt"
	"net/url"
	"strings"

	"rogchap.com/v8go"

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
				injectCanvas2D(ctxObj, b.iso)
			} else if strings.HasPrefix(ctxType, "webgl") {
				injectWebGL(ctxObj, b.iso, b.fp)
			}
			inst, _ := ctxObj.NewInstance(info.Context())
			return inst.Value
		}))
		obj.Set("toDataURL", v8go.NewFunctionTemplate(b.iso, func(info *v8go.FunctionCallbackInfo) *v8go.Value {
			v, _ := v8go.NewValue(b.iso, "data:image/png;base64,")
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
	p.global.Set("require", v8go.Undefined(p.iso))
	p.global.Set("global", v8go.Undefined(p.iso))
	p.global.Set("module", v8go.Undefined(p.iso))
	p.global.Set("exports", v8go.Undefined(p.iso))
	p.global.Set("__dirname", v8go.Undefined(p.iso))
	p.global.Set("__filename", v8go.Undefined(p.iso))

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

	// Inject permissions API (navigator.permissions)
	p.injectPermissions()

	// Inject fetch + XMLHttpRequest
	p.injectFetchXHR()

	// Inject comprehensive env shim (missing browser APIs)
	// This is the big one — adds ~30 missing APIs via JS
	if _, err := p.ctx.RunScript(envShimJS(), "env-shim.js"); err != nil {
		// Log but don't fail — some APIs might conflict with existing ones
		fmt.Printf("[sandbox] env shim warning: %v\n", err)
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

func injectCanvas2D(ctx *v8go.ObjectTemplate, iso *v8go.Isolate) {
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
		v, _ := v8go.NewValue(iso, fmt.Sprintf(`{"width":%d}`, len(text)*6))
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

	js := fmt.Sprintf(`
		(function(){
			Object.defineProperty(navigator, 'languages', {value: %s, configurable: true, writable: true});
			Object.defineProperty(navigator, 'userAgentData', {value: %s, configurable: true, writable: true});
			Object.defineProperty(navigator, 'connection', {value: %s, configurable: true, writable: true});
		})();
	`, langArr, uadJSON, connJSON)
	p.ctx.RunScript(js, "nav-complex.js")
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

// injectFetchXHR adds fetch and XMLHttpRequest stubs via JS
func (p *PostContextBuilder) injectFetchXHR() {
	js := `
		(function(){
			// fetch stub — real impl in Phase 4 via netlayer
			window.fetch = function(resource, options) {
				var mockResp = {
					ok: true,
					status: 200,
					headers: { get: function() { return null; } },
					json: function() { return Promise.resolve({}); },
					text: function() { return Promise.resolve(''); },
					arrayBuffer: function() { return Promise.resolve(new ArrayBuffer(0)); }
				};
				return Promise.resolve(mockResp);
			};

			// XMLHttpRequest stub
			window.XMLHttpRequest = function() {
				this.readyState = 0;
				this.status = 0;
				this.responseText = '';
				this.response = '';
				this.onreadystatechange = null;
				this.onload = null;
				this.onerror = null;
				this._method = 'GET';
				this._url = '';
				this._headers = {};
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
				// Stub: real impl in Phase 4 via netlayer
			};
			window.XMLHttpRequest.prototype.getAllResponseHeaders = function() {
				return '';
			};
			window.XMLHttpRequest.prototype.getResponseHeader = function(name) {
				return null;
			};
			window.XMLHttpRequest.prototype.abort = function() {};
		})();
	`
	p.ctx.RunScript(js, "fetch-xhr.js")
}
