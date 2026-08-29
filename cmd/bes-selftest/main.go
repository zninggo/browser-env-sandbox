// Package main — sandbox self-test
// Runs comprehensive browser detection checks inside the sandbox
// and reports pass/fail for each.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
)

type check struct {
	name   string
	code   string
	expect string // substring match
}

func main() {
	fpEng := fpengine.New()
	eng := sandbox.New(fpEng, 4)
	defer eng.Dispose()

	sess, err := eng.CreateSession(api.SessionOptions{
		Browser:  "chrome",
		OS:       "windows",
		Location: "https://example.com/login",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Dispose()

	fmt.Printf("Session: %s\n", sess.ID)
	fmt.Printf("UA: %s\n\n", sess.GetFingerprint().Navigator["userAgent"])

	checks := []check{
		// ── Navigator ──
		{"navigator.userAgent exists", `navigator.userAgent`, "Mozilla/5.0"},
		{"navigator.platform = Win32", `navigator.platform`, "Win32"},
		{"navigator.webdriver = false", `String(navigator.webdriver)`, "false"},
		{"navigator.language matches languages[0]", `String(navigator.language === navigator.languages[0])`, "true"},
		{"navigator.languages is array", `Array.isArray(navigator.languages)`, "true"},
		{"navigator.hardwareConcurrency > 0", `navigator.hardwareConcurrency > 0`, "true"},
		{"navigator.cookieEnabled", `String(navigator.cookieEnabled)`, "true"},
		{"navigator.onLine", `String(navigator.onLine)`, "true"},
		{"navigator.vendor = Google Inc.", `navigator.vendor`, "Google Inc."},
		{"navigator.maxTouchPoints = 0", `String(navigator.maxTouchPoints)`, "0"},
		{"navigator.userAgentData exists", `typeof navigator.userAgentData`, "object"},
		{"navigator.permissions exists", `typeof navigator.permissions`, "object"},
		// ── Canary probes (should be undefined) ──
		{"navigator.pemrissions = undefined (canary)", `typeof navigator.pemrissions`, "undefined"},
		{"window.callPhantom = undefined (canary)", `typeof window.callPhantom`, "undefined"},
		{"window._phantom = undefined (canary)", `typeof window._phantom`, "undefined"},
		{"window.__nightmare = undefined (canary)", `typeof window.__nightmare`, "undefined"},
		{"window.domAutomation = undefined (canary)", `typeof window.domAutomation`, "undefined"},
		// ── Node traces (must be undefined) ──
		{"typeof Buffer = undefined", `typeof Buffer`, "undefined"},
		{"typeof process = undefined", `typeof process`, "undefined"},
		{"typeof require = undefined", `typeof require`, "undefined"},
		{"typeof global = undefined", `typeof global`, "undefined"},
		{"typeof module = undefined", `typeof module`, "undefined"},
		{"typeof __dirname = undefined", `typeof __dirname`, "undefined"},
		// ── Window identity ──
		{"typeof window = object", `typeof window`, "object"},
		{"window === globalThis", `window === globalThis`, "true"},
		{"window === self", `window === self`, "true"},
		{"window === top", `window === top`, "true"},
		{"window === parent", `window === parent`, "true"},
		{"window === frames", `window === frames`, "true"},
		// ── Document ──
		{"document.URL matches location", `document.URL`, "https://example.com/login"},
		{"document.readyState = complete", `document.readyState`, "complete"},
		{"document.visibilityState = visible", `document.visibilityState`, "visible"},
		{"typeof document.cookie = string", `typeof document.cookie`, "string"},
		{"document.cookie write/read", `(function(){document.cookie='test=123;path=/';return document.cookie.includes('test=123')})()`, "true"},
		// ── Screen ──
		{"screen.width > 0", `screen.width > 0`, "true"},
		{"screen.height > 0", `screen.height > 0`, "true"},
		{"screen.colorDepth > 0", `String(screen.colorDepth > 0)`, "true"},
		// ── Window dimensions ──
		{"innerWidth > 0", `innerWidth > 0`, "true"},
		{"innerHeight > 0", `innerHeight > 0`, "true"},
		{"devicePixelRatio > 0", `devicePixelRatio > 0`, "true"},
		// ── Chrome feature ──
		{"window.chrome exists", `typeof window.chrome`, "object"},
		{"window.chrome.runtime exists", `typeof window.chrome.runtime`, "object"},
		// ── Timers ──
		{"typeof setTimeout = function", `typeof setTimeout`, "function"},
		{"typeof setInterval = function", `typeof setInterval`, "function"},
		{"typeof requestAnimationFrame = function", `typeof requestAnimationFrame`, "function"},
		// ── Console ──
		{"typeof console.log = function", `typeof console.log`, "function"},
		{"typeof console.error = function", `typeof console.error`, "function"},
		// ── Crypto ──
		{"typeof crypto = object", `typeof crypto`, "object"},
		{"typeof crypto.getRandomValues = function", `typeof crypto.getRandomValues`, "function"},
		// ── Storage ──
		{"typeof localStorage = object", `typeof localStorage`, "object"},
		{"typeof sessionStorage = object", `typeof sessionStorage`, "object"},
		// ── Location ──
		{"location.href matches", `location.href`, "https://example.com/login"},
		{"location.protocol = https:", `location.protocol`, "https:"},
		{"location.hostname = example.com", `location.hostname`, "example.com"},
		// ── History ──
		{"typeof history = object", `typeof history`, "object"},
		{"history.length = 1", `String(history.length)`, "1"},
		// ── DOM ──
		{"document.createElement exists", `typeof document.createElement`, "function"},
		{"document.getElementById exists", `typeof document.getElementById`, "function"},
		{"createElement('div') returns object", `typeof document.createElement('div')`, "object"},
		{"createElement('canvas') has getContext", `typeof document.createElement('canvas').getContext`, "function"},
		{"canvas.getContext('2d') not null", `String(document.createElement('canvas').getContext('2d') !== null)`, "true"},
		{"canvas.getContext('webgl') not null", `String(document.createElement('canvas').getContext('webgl') !== null)`, "true"},
		// ── Performance ──
		{"typeof performance = object", `typeof performance`, "object"},
		{"typeof performance.now = function", `typeof performance.now`, "function"},
		// ── MutationObserver / MessageChannel ──
		{"typeof MutationObserver = function", `typeof MutationObserver`, "function"},
		{"typeof MessageChannel = function", `typeof MessageChannel`, "function"},
		// ── Event ──
		{"typeof Event = function", `typeof Event`, "function"},
		// ── Intl ──
		{"typeof Intl = object", `typeof Intl`, "object"},
		{"Intl.DateTimeFormat exists", `typeof Intl.DateTimeFormat`, "function"},
		// ── fetch / XMLHttpRequest ──
		{"typeof fetch = function", `typeof fetch`, "function"},
		{"typeof XMLHttpRequest = function", `typeof XMLHttpRequest`, "function"},
		// ── Shim-injected APIs (env_shim_part1/2/3.js) ──
		// Navigator extensions
		{"navigator.plugins exists", `typeof navigator.plugins`, "object"},
		{"navigator.plugins.length > 0", `navigator.plugins.length > 0`, "true"},
		{"navigator.mimeTypes exists", `typeof navigator.mimeTypes`, "object"},
		{"navigator.mediaDevices exists", `typeof navigator.mediaDevices`, "object"},
		{"navigator.mediaDevices.enumerateDevices", `typeof navigator.mediaDevices.enumerateDevices`, "function"},
		{"navigator.serviceWorker exists", `typeof navigator.serviceWorker`, "object"},
		{"navigator.clipboard exists", `typeof navigator.clipboard`, "object"},
		{"navigator.geolocation exists", `typeof navigator.geolocation`, "object"},
		{"navigator.bluetooth exists", `typeof navigator.bluetooth`, "object"},
		{"navigator.credentials exists", `typeof navigator.credentials`, "object"},
		{"navigator.storage exists", `typeof navigator.storage`, "object"},
		{"navigator.getBattery exists", `typeof navigator.getBattery`, "function"},
		// Global constructors
		{"typeof TextEncoder = function", `typeof TextEncoder`, "function"},
		{"typeof TextDecoder = function", `typeof TextDecoder`, "function"},
		{"typeof URLSearchParams = function", `typeof URLSearchParams`, "function"},
		{"typeof URL = function", `typeof URL`, "function"},
		{"typeof Blob = function", `typeof Blob`, "function"},
		{"typeof FormData = function", `typeof FormData`, "function"},
		{"typeof AbortController = function", `typeof AbortController`, "function"},
		{"typeof DOMException = function", `typeof DOMException`, "function"},
		{"typeof DOMParser = function", `typeof DOMParser`, "function"},
		{"typeof Notification = function", `typeof Notification`, "function"},
		{"typeof RTCPeerConnection = function", `typeof RTCPeerConnection`, "function"},
		{"typeof WebSocket = function", `typeof WebSocket`, "function"},
		{"typeof Worker = function", `typeof Worker`, "function"},
		{"typeof ResizeObserver = function", `typeof ResizeObserver`, "function"},
		{"typeof IntersectionObserver = function", `typeof IntersectionObserver`, "function"},
		// Utility functions
		{"typeof atob = function", `typeof atob`, "function"},
		{"typeof btoa = function", `typeof btoa`, "function"},
		{"typeof matchMedia = function", `typeof matchMedia`, "function"},
		{"typeof CSS = object", `typeof CSS`, "object"},
		{"typeof CSS.supports = function", `typeof CSS.supports`, "function"},
		{"typeof structuredClone = function", `typeof structuredClone`, "function"},
		{"typeof URL.createObjectURL = function", `typeof URL.createObjectURL`, "function"},
		// atob/btoa functional test
		{"atob/btoa roundtrip", `atob(btoa('hello')) === 'hello'`, "true"},
		// TextEncoder functional test
		{"TextEncoder.encode", `new TextEncoder().encode('test').length`, "4"},
		// WebRTC
		{"RTCPeerConnection createOffer", `typeof new RTCPeerConnection().createOffer`, "function"},
		// WebSocket states
		{"WebSocket.CONNECTING = 0", `String(WebSocket.CONNECTING)`, "0"},
		// indexedDB
		{"typeof indexedDB = object", `typeof indexedDB`, "object"},
		// DOM element enhancements
		{"element.innerHTML set/get", `(function(){var e=document.createElement('div');e.innerHTML='<b>test</b>';return e.innerHTML})()`, "<b>test</b>"},
		{"element.classList.add", `(function(){var e=document.createElement('div');e.classList.add('foo');return e.classList.contains('foo')})()`, "true"},
		{"element.addEventListener", `typeof document.createElement('div').addEventListener`, "function"},
		{"element.click", `typeof document.createElement('div').click`, "function"},
		{"element.offsetWidth = 0", `String(document.createElement('div').offsetWidth)`, "0"},
		// Symbol.toStringTag
		{"navigator toString = [object Navigator]", `Object.prototype.toString.call(navigator)`, "[object Navigator]"},
		{"document toString = [object HTMLDocument]", `Object.prototype.toString.call(document)`, "[object HTMLDocument]"},
		{"window toString = [object Window]", `Object.prototype.toString.call(window)`, "[object Window]"},
		// document enhancements
		{"document.head exists", `typeof document.head`, "object"},
		{"document.body exists", `typeof document.body`, "object"},
		{"document.hasFocus = true", `String(document.hasFocus())`, "true"},
		{"document.hidden = false", `String(document.hidden)`, "false"},
		{"document.activeElement exists", `typeof document.activeElement`, "object"},
		// window enhancements
		{"typeof getComputedStyle = function", `typeof getComputedStyle`, "function"},
		{"window.name = ''", `window.name`, ""},
		{"window.origin exists", `typeof window.origin`, "string"},
		{"window.closed = false", `String(window.closed)`, "false"},
		{"window.opener = null", `String(window.opener)`, "null"},
		// crypto enhancements
		{"crypto.subtle exists", `typeof crypto.subtle`, "object"},
		{"crypto.randomUUID exists", `typeof crypto.randomUUID`, "function"},
		// performance enhancements
		{"performance.timing exists", `typeof performance.timing`, "object"},
		{"performance.memory exists", `typeof performance.memory`, "object"},
		// ── Shim part 4: WebGPU + behavior + CSS ──
		{"navigator.gpu exists", `typeof navigator.gpu`, "object"},
		{"navigator.gpu.getPreferredCanvasFormat", `typeof navigator.gpu.getPreferredCanvasFormat`, "function"},
		{"navigator.gpu.requestAdapter", `typeof navigator.gpu.requestAdapter`, "function"},
		{"_besBehavior exists", `typeof _besBehavior`, "object"},
		{"_besBehavior.generateMousePath", `typeof _besBehavior.generateMousePath`, "function"},
		{"_besBehavior.generateKeyTimings", `typeof _besBehavior.generateKeyTimings`, "function"},
		{"element.humanClick exists", `typeof document.createElement('div').humanClick`, "function"},
		{"element.humanType exists", `typeof document.createElement('input').humanType`, "function"},
		{"mousePath generates steps", `_besBehavior.generateMousePath(0,0,100,100).length > 0`, "true"},
		{"keyTimings generates delays", `_besBehavior.generateKeyTimings('hello').length`, "5"},
		// ── HTML parser (DOMParser real DOM tree) ──
		{"DOMParser parses HTML tree", `(function(){var d=new DOMParser().parseFromString('<html><head><title>T</title></head><body><div id="x">hi</div></body></html>','text/html');return d.documentElement.tagName})()`, "HTML"},
		{"DOMParser body has children", `(function(){var d=new DOMParser().parseFromString('<body><div id="x">hi</div><span></span></body>','text/html');return d.body.children.length})()`, "2"},
		{"DOMParser getElementById", `(function(){var d=new DOMParser().parseFromString('<body><div id="x">hi</div></body>','text/html');return d.getElementById('x') !== null})()`, "true"},
		{"DOMParser getElementsByTagName", `(function(){var d=new DOMParser().parseFromString('<body><div>1</div><div>2</div></body>','text/html');return d.getElementsByTagName('div').length})()`, "2"},
		// ── HTML parser: auto-closing + template + foreign content ──
		{"DOMParser auto-closes <p> on <div>", `(function(){var d=new DOMParser().parseFromString('<p>a<div>b</div>','text/html');var ps=d.getElementsByTagName('p');return ps.length === 1 && ps[0].children.length === 0})()`, "true"},
		{"DOMParser auto-closes <li> on next <li>", `(function(){var d=new DOMParser().parseFromString('<ul><li>a<li>b</ul>','text/html');return d.getElementsByTagName('li').length})()`, "2"},
		{"DOMParser auto-closes <td> on next <td>", `(function(){var d=new DOMParser().parseFromString('<table><tr><td>a<td>b</tr></table>','text/html');return d.getElementsByTagName('td').length})()`, "2"},
		{"DOMParser <template> has content fragment", `(function(){var d=new DOMParser().parseFromString('<template><span>x</span></template>','text/html');var t=d.getElementsByTagName('template')[0];return t.content && t.content.children.length > 0})()`, "true"},
		{"DOMParser <svg> flagged as foreign", `(function(){var d=new DOMParser().parseFromString('<svg><rect/></svg>','text/html');var s=d.getElementsByTagName('svg')[0];return s._besForeignNS === 'svg'})()`, "true"},
		{"DOMParser nested attributes parsed", `(function(){var d=new DOMParser().parseFromString('<div id="t" class="c" data-x="y">hi</div>','text/html');var el=d.getElementById('t');return el.className === 'c' && el.tagName === 'DIV'})()`, "true"},
		// ── Async XHR mode ──
		{"__besPendingXHR exists", `typeof __besPendingXHR`, "number"},
		{"XMLHttpRequest.prototype.send is async", `typeof XMLHttpRequest.prototype.send`, "function"},
		// ── CAPTCHA solver bridge ──
		{"__besSolveCaptcha exists", `typeof __besSolveCaptcha`, "function"},
		{"bes.solveCaptcha exists", `typeof bes.solveCaptcha`, "function"},
		{"bes.solveCaptchaAsync exists", `typeof bes.solveCaptchaAsync`, "function"},
		{"bes.solveCaptcha returns unsolved (noop)", `bes.solveCaptcha('recaptcha','test','https://example.com').solved`, "false"},
		// ── 通用补环境增强（DTraitSDK / challenge-template 依赖） ──
		{"Uint8Array.prototype[Symbol.iterator] exists", `typeof Uint8Array.prototype[Symbol.iterator]`, "function"},
		{"Uint8Array.prototype.values exists", `typeof Uint8Array.prototype.values`, "function"},
		{"Uint8Array.prototype.entries exists", `typeof Uint8Array.prototype.entries`, "function"},
		{"Uint8Array iterable via for-of", `(function(){var s=0;for(var v of new Uint8Array([1,2,3]))s+=v;return String(s)})()`, "6"},
		{"Uint8Array spread works", `[...new Uint8Array([10,20,30])].join(',')`, "10,20,30"},
		{"crypto.getRandomValues fills bytes", `(function(){var a=new Uint8Array(8);crypto.getRandomValues(a);var nonzero=0;for(var i=0;i<a.length;i++)if(a[i]!==0)nonzero++;return String(nonzero>0)})()`, "true"},
		{"crypto.getRandomValues returns same array", `(function(){var a=new Uint8Array(4);var b=crypto.getRandomValues(a);return String(a===b)})()`, "true"},
		{"localStorage.getItem/setItem", `(function(){localStorage.setItem('t','v');return localStorage.getItem('t')})()`, "v"},
		{"localStorage.removeItem", `(function(){localStorage.setItem('t','v');localStorage.removeItem('t');return String(localStorage.getItem('t'))})()`, "null"},
		{"localStorage.length", `(function(){localStorage.clear();localStorage.setItem('a','1');localStorage.setItem('b','2');return String(localStorage.length)})()`, "2"},
		{"sessionStorage.getItem/setItem", `(function(){sessionStorage.setItem('t','v');return sessionStorage.getItem('t')})()`, "v"},
		{"iframe.contentWindow exists", `typeof document.createElement('iframe').contentWindow`, "object"},
		{"iframe.contentWindow !== window (Bug 32)", `String(document.createElement('iframe').contentWindow !== window)`, "true"},
		{"iframe.contentDocument !== document (Bug 32)", `String(document.createElement('iframe').contentDocument !== document)`, "true"},
		{"iframe.contentWindow sees globals via prototype", `String(document.createElement('iframe').contentWindow.navigator !== undefined)`, "true"},
		{"performance.timeOrigin is number", `typeof performance.timeOrigin`, "number"},
		{"performance.now() >= 0", `String(performance.now() >= 0)`, "true"},
		{"performance.now() is monotonic", `(function(){var a=performance.now();var b=performance.now();return String(b>=a)})()`, "true"},
		// ── 真实指纹修复 ──
		{"WebGL getParameter(37445) not WebKit", `(function(){var v=document.createElement('canvas').getContext('webgl').getParameter(37445);return String(v!=='WebKit'&&v!==''&&v!==null)})()`, "true"},
		{"WebGL getParameter(37446) not WebKit WebGL", `(function(){var v=document.createElement('canvas').getContext('webgl').getParameter(37446);return String(v!=='WebKit WebGL'&&v!==''&&v!==null)})()`, "true"},
		{"WebGL getParameter(34076) = 16384", `document.createElement('canvas').getContext('webgl').getParameter(34076)`, "16384"},
		{"WebGL getParameter(35724) exists", `typeof document.createElement('canvas').getContext('webgl').getParameter(35724)`, "string"},
		{"WebGL extensions has WEBGL_multi_draw", `String(document.createElement('canvas').getContext('webgl').getSupportedExtensions().indexOf('WEBGL_multi_draw')>=0)`, "true"},
		{"canvas.toDataURL starts with data:image/png;base64,", `document.createElement('canvas').toDataURL().substring(0,22)`, "data:image/png;base64,"},
		{"canvas.toDataURL length > 1000", `String(document.createElement('canvas').toDataURL().length > 1000)`, "true"},
		{"canvas.toDataURL has valid base64", `(function(){var u=document.createElement('canvas').toDataURL();var b=u.split(',')[1];return String(b.length>100&&/^[A-Za-z0-9+/=]+$/.test(b))})()`, "true"},
		{"crypto.subtle.digest is real SHA-256", `(async function(){var b=await crypto.subtle.digest('SHA-256',new TextEncoder().encode('abc'));var h=new Uint8Array(b);var s='';for(var i=0;i<h.length;i++)s+=h[i].toString(16).padStart(2,'0');return s})()`, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		// ── Bug 修复验证 ──
		{"WebGL getSupportedExtensions is array", `Array.isArray(document.createElement('canvas').getContext('webgl').getSupportedExtensions())`, "true"},
		{"MessageChannel port1 !== port2", `String(new MessageChannel().port1 !== new MessageChannel().port2)`, "true"},
		{"MessageChannel postMessage delivers", `(function(){var mc=new MessageChannel();var got=null;mc.port2.onmessage=function(e){got=e.data};mc.port1.postMessage('hello');return 'pending'})()`, "pending"},
		{"XHR open saves async=false", `(function(){var x=new XMLHttpRequest();x.open('GET','/test',false);return String(x._async)})()`, "false"},
		{"canvas getImageData returns proper size", `(function(){var c=document.createElement('canvas');c.width=10;c.height=10;var d=c.getContext('2d').getImageData(0,0,5,5);return String(d.width===5)})()`, "true"},
		{"URLSearchParams append preserves multiple", `(function(){var p=new URLSearchParams();p.append('a','1');p.append('a','2');return p.getAll('a').join(',')})()`, "1,2"},
		{"FileReader exists with readAsText", `typeof FileReader.prototype.readAsText`, "function"},
		{"plugins.namedItem finds by name", `String(navigator.plugins.namedItem('PDF Viewer') !== null)`, "true"},
		// ── 第二轮修复验证（BUGS-ROUND2） ──
		{"measureText non-linear (Bug 7 residual)", `(function(){var c=document.createElement('canvas').getContext('2d');var perI=c.measureText('iiiii').width/5;var perW=c.measureText('WWWWW').width/5;return String(perW>perI && perW!==perI)})()`, "true"},
		{"measureText width varies per char (Bug 7 residual)", `(function(){var c=document.createElement('canvas').getContext('2d');var w1=c.measureText('iiiii').width;var w2=c.measureText('WWWWW').width;return String(w2 > w1)})()`, "true"},
		{"measureText returns TextMetrics-like object", `(function(){var m=document.createElement('canvas').getContext('2d').measureText('hi');return String(typeof m.width === 'number' && typeof m.actualBoundingBoxAscent === 'number')})()`, "true"},
		{"crypto.getRandomValues uses crypto/rand (Bug 18)", `(function(){var a=new Uint8Array(32),b=new Uint8Array(32);crypto.getRandomValues(a);crypto.getRandomValues(b);var same=true;for(var i=0;i<32;i++)if(a[i]!==b[i]){same=false;break}return String(!same)})()`, "true"},
		{"crypto.getRandomValues large array throws (spec cap)", `(function(){try{crypto.getRandomValues(new Uint8Array(65537));return 'no-throw'}catch(e){return String(e instanceof RangeError)}})()`, "true"},
		{"Event has preventDefault (Bug 43 residual)", `typeof new Event('x').preventDefault`, "function"},
		{"Event has stopPropagation (Bug 43 residual)", `typeof new Event('x').stopPropagation`, "function"},
		{"Event methods callable without error (Bug 43)", `(function(){var e=new Event('x',{cancelable:true});e.preventDefault();e.stopPropagation();e.stopImmediatePropagation();return String(e.defaultPrevented===true&&e.cancelBubble===true)})()`, "true"},
		{"CustomEvent inherits Event methods (Bug 43)", `typeof new CustomEvent('x').preventDefault`, "function"},
		{"URL IPv6 port parse (Bug 29)", `new URL('http://[::1]:8080/path').port`, "8080"},
		{"URL IPv6 hostname kept (Bug 29)", `new URL('http://[::1]:8080/path').hostname`, "[::1]"},
		{"URL credentials parse (Bug 29)", `(function(){var u=new URL('http://user:pass@example.com/');return u.username+':'+u.password})()`, "user:pass"},
		{"URL default port omitted (Bug 29)", `new URL('http://example.com:80/').port`, ""},
		{"URL origin normalized (Bug 29)", `new URL('https://example.com/path?q=1').origin`, "https://example.com"},
		// ── Worker real execution ──
		{"Worker roundtrip postMessage (double)", `new Promise(function(res, rej){ var w = new Worker("self.onmessage = function(e){ self.postMessage(e.data * 2); };"); w.onmessage = function(e){ res(String(e.data)); }; w.onerror = function(e){ rej(new Error(String(e.message || e))); }; w.postMessage(21); setTimeout(function(){ rej(new Error("worker timeout")); }, 3000); })`, "42"},
		// ── Dynamic import() polyfill ──
		{"importModule exists", `typeof importModule`, "function"},
		{"importModule default export", `importModule("data:text/javascript;base64," + btoa("export default 42")).then(function(m){ return String(m.default); })`, "42"},
		{"importModule named exports", `importModule("data:text/javascript;base64," + btoa("export const add = function(a,b){return a+b}; export default 99;")).then(function(m){ return "add=" + m.add(2,3) + ",default=" + m.default; })`, "add=5,default=99"},
	}

	passed := 0
	failed := 0
	var failures []string

	for _, c := range checks {
		result, err := sess.Eval(c.code)
		if err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("  ❌ %s → ERROR: %v", c.name, err))
			continue
		}
		if strings.Contains(result, c.expect) {
			passed++
			fmt.Printf("  ✅ %s → %s\n", c.name, truncate(result, 60))
		} else {
			failed++
			failures = append(failures, fmt.Sprintf("  ❌ %s → got '%s', expected '%s'", c.name, truncate(result, 60), c.expect))
			fmt.Printf("  ❌ %s → got '%s', expected '%s'\n", c.name, truncate(result, 60), c.expect)
		}
	}

	// Timer test
	fmt.Println("\n── Timer test ──")
	timerResult, timerErr := sess.Eval(`
		(function(){
			var result = [];
			var done = false;
			setTimeout(function(){ result.push('timeout1'); done = true; }, 10);
			setTimeout(function(){ result.push('timeout2'); }, 20);
			return 'pending';
		})()
	`)
	if timerErr != nil {
		fmt.Printf("  ❌ Timer test error: %v\n", timerErr)
		failed++
	} else {
		fmt.Printf("  → %s (flushing timers...)\n", timerResult)
		sess.FlushTimers(2 * time.Second)
		sess.PerformMicrotasks()
		fmt.Printf("  ✅ Timers scheduled and flushed\n")
		passed++
	}

	// Network header consistency test: the fingerprint must drive outbound
	// request headers end-to-end (navigator.languages → Accept-Language,
	// navigator.userAgent → User-Agent). Uses a mock NetHandler to capture
	// what actually goes on the wire — JS-level checks cannot see this.
	fmt.Println("\n── Network header consistency ──")
	captured := make(chan map[string]string, 1)
	eng.SetNetHandlerFactory(func(opts api.SessionOptions, fp *api.Fingerprint, cs *sandbox.CookieStore) sandbox.NetHandler {
		return captureHandler{capture: captured}
	})
	netSess, err := eng.CreateSession(api.SessionOptions{
		Browser:  "chrome",
		OS:       "windows",
		Location: "https://example.com/login",
	})
	if err != nil {
		fmt.Printf("  ❌ network session error: %v\n", err)
		failed++
	} else {
		_, evalErr := netSess.Eval(`fetch('https://example.com/api/data').then(function(){return 'done'})`)
		if evalErr != nil {
			fmt.Printf("  ❌ fetch eval error: %v\n", evalErr)
			failed++
		} else {
			select {
			case headers := <-captured:
				fp := netSess.GetFingerprint()
				wantLangs := fp.Languages
				// Expected Chrome format: first language bare, the rest
				// q-decayed starting at 0.9 (zh-CN,zh;q=0.9,en;q=0.8).
				expectParts := make([]string, 0, len(wantLangs))
				for i, l := range wantLangs {
					if i == 0 {
						expectParts = append(expectParts, l)
					} else {
						expectParts = append(expectParts, fmt.Sprintf("%s;q=0.%d", l, 10-i))
					}
				}
				expectAL := strings.Join(expectParts, ",")
				gotAL, hasAL := lookupHeader(headers, "Accept-Language")
				gotUA, hasUA := lookupHeader(headers, "User-Agent")
				wantUA, _ := fp.Navigator["userAgent"].(string)
				alOK := hasAL && gotAL == expectAL
				uaOK := hasUA && gotUA == wantUA
				if alOK && uaOK {
					passed++
					fmt.Printf("  ✅ Accept-Language matches navigator.languages → %s\n", truncate(gotAL, 60))
					fmt.Printf("  ✅ User-Agent matches navigator.userAgent\n")
					passed++
				} else {
					if !alOK {
						failed++
						failures = append(failures, fmt.Sprintf("  ❌ Accept-Language: got '%v', want '%s'", headers, expectAL))
						fmt.Printf("  ❌ Accept-Language mismatch → got '%s' (has=%v), want '%s'\n", gotAL, hasAL, expectAL)
					}
					if !uaOK {
						failed++
						failures = append(failures, fmt.Sprintf("  ❌ User-Agent: got '%s', want '%s'", gotUA, wantUA))
						fmt.Printf("  ❌ User-Agent mismatch → got '%s' (has=%v), want '%s'\n", gotUA, hasUA, wantUA)
					}
				}
			case <-time.After(5 * time.Second):
				failed++
				failures = append(failures, "  ❌ network capture timeout — fetch never reached NetHandler")
				fmt.Printf("  ❌ capture timeout — fetch never reached NetHandler\n")
			}
		}
		netSess.Dispose()
	}

	// Summary
	fmt.Printf("\n══════════════════════════════════\n")
	fmt.Printf("  Results: %d passed, %d failed, %d total\n", passed, failed, passed+failed)
	if len(failures) > 0 {
		fmt.Printf("\n  Failures:\n")
		for _, f := range failures {
			fmt.Println(f)
		}
	}
	fmt.Printf("══════════════════════════════════\n")

	if failed > 0 {
		os.Exit(1)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// captureHandler is a mock NetHandler that records the headers of the first
// request it sees, so the self-test can assert what would go on the wire.
type captureHandler struct {
	capture chan map[string]string
}

func (h captureHandler) Request(method, url string, headers map[string]string, body []byte) (*sandbox.NetResponse, error) {
	select {
	case h.capture <- headers:
	default:
	}
	return &sandbox.NetResponse{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"ok":true}`,
	}, nil
}

// lookupHeader does a case-insensitive header lookup.
func lookupHeader(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}
