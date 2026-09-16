// Package executor implements the dual-executor architecture.
//
// Pure V8 cannot faithfully answer fingerprint points that require real
// execution: canvas rasterization, WebGL readbacks, AudioContext DSP and
// WebAssembly behavior all leak the mock when probed hard enough. This
// package builds two executors behind one interface:
//
//   - V8Executor    — the pure-V8 sandbox (default; fast, hostless, replayed
//     fingerprint points).
//   - ChromeExecutor — a real, locally launched, isolated Chrome driven over
//     the Chrome DevTools Protocol (canvas/wasm/etc. rendered by a real
//     engine).
//
// The core invariant is that ONE device profile drives every executor: a
// ProfileSnapshot (an api.Fingerprint plus its generation parameters) is
// applied to whichever executor a session is created on, so the identity
// surface (UA, platform, GPU strings, screen, hardwareConcurrency, timezone,
// languages, userAgentData) reads identically through either backend. The
// Router picks the backend per fingerprint-point class — V8 by default,
// Chrome for the points that need real rendering — and falls back to V8 when
// Chrome is unavailable or fails.
//
// Constraint: the Chrome executor only ever talks to a Chrome instance it
// launched itself on 127.0.0.1 with an isolated temp user-data-dir. The CDP
// dialer refuses non-local endpoints, and no remote/online browser can be
// configured through this package.
package executor

import (
	"context"
	"time"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// PointClass classifies a fingerprint point by how real its execution must
// be. The Router maps classes to executors: identity/plain-logic classes run
// on the V8 sandbox (the default), real-execution classes run on Chrome when
// it is available.
type PointClass string

const (
	// PointJS is plain script logic (signing algorithms, data munging).
	// No realism requirement beyond the browser environment itself.
	PointJS PointClass = "js"
	// PointNavigator covers navigator identity strings (UA, platform,
	// hardwareConcurrency, deviceMemory, userAgentData…). Mocked faithfully
	// by the V8 env — no real execution needed.
	PointNavigator PointClass = "navigator"
	// PointScreen covers screen/window geometry. Pure data — V8.
	PointScreen PointClass = "screen"
	// PointFonts covers font enumeration/metrics. The V8 env replays the
	// profile font list; a local Chrome would leak HOST fonts (Linux fonts
	// under a Windows profile), so this class stays on V8 by design.
	PointFonts PointClass = "fonts"
	// PointCanvas requires real rasterization (toDataURL/readback of drawn
	// text+geometry). Real rendering → Chrome.
	PointCanvas PointClass = "canvas"
	// PointWebGL requires a real GL pipeline for readbacks; only the identity
	// strings (vendor/renderer) are profile-driven, the rendering is real.
	PointWebGL PointClass = "webgl"
	// PointAudio requires real AudioContext DSP output.
	PointAudio PointClass = "audio"
	// PointWasm requires a real WebAssembly engine (features, threads, SIMD
	// surface). Real execution → Chrome.
	PointWasm PointClass = "wasm"
)

// realClasses are the point classes routed to the Chrome executor when it is
// available. Everything else defaults to V8.
var realClasses = map[PointClass]bool{
	PointCanvas: true,
	PointWebGL:  true,
	PointAudio:  true,
	PointWasm:   true,
}

// RealClass reports whether the class requires a real execution engine.
func RealClass(c PointClass) bool { return realClasses[c] }

// Executor is a fingerprint-point execution backend. Implementations must be
// safe for concurrent use.
type Executor interface {
	// Name is a short stable identifier ("v8", "chrome").
	Name() string
	// Available reports whether the executor can currently create sessions
	// (the V8 executor is always available; the Chrome executor requires a
	// local Chrome binary). It must not launch anything — it is a cheap check.
	Available() bool
	// NewSession creates a profile-driven execution context. The SAME
	// ProfileSnapshot handed to different executors must surface the same
	// identity points (this is what VerifyConsistency checks).
	NewSession(ctx context.Context, profile *ProfileSnapshot, opts SessionOptions) (Session, error)
	// Dispose releases executor-owned resources (Chrome process, isolate
	// pool). Sessions must be disposed separately.
	Dispose()
}

// Session is one profile-driven execution context. A Session is NOT safe for
// concurrent Eval — callers serialize evaluations (mirrors sandbox.Session).
type Session interface {
	// Executor names the backend this session runs on.
	Executor() string
	// Eval executes JavaScript and returns the stringified result. Promise
	// results are awaited. A zero timeout means the executor default.
	Eval(ctx context.Context, code string, timeout time.Duration) (string, error)
	// Dispose tears the session down.
	Dispose()
}

// SessionOptions tunes session creation across executors.
type SessionOptions struct {
	// Location is the document URL. Empty defaults to the executor's own
	// (V8: https://example.com/, Chrome: about:blank). The Chrome executor
	// only navigates to local/inert URLs (about:, data:) — its sessions are
	// for fingerprint-point execution, not web fetching.
	Location string
}

// ProfileProvider generates the fingerprint a snapshot is built from.
// fpengine.Engine satisfies it, as does any sandbox.FingerprintProvider.
type ProfileProvider interface {
	GenerateWithTimezone(seed uint64, browser, os, timezone string) (*api.Fingerprint, error)
}
