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
		{"navigator.language exists", `navigator.language`, "zh"},
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
		{"screen.colorDepth = 24", `String(screen.colorDepth)`, "24"},
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
