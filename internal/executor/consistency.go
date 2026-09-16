package executor

// Profile consistency verification: drive BOTH executors from one
// ProfileSnapshot, read the identity surface back through each, and compare
// field-by-field against the profile (and therefore against each other).
// Also collects real-execution evidence from Chrome (canvas rasterization,
// WebAssembly) to show the realism side of the dual-executor contract.

import (
	"context"
	"fmt"
	"sort"
)

// FieldCheck is one identity point compared across the profile and both
// executors. Pass means: the value read from V8 equals the profile AND the
// value read from Chrome equals the profile (hence V8 == Chrome).
type FieldCheck struct {
	Field  string `json:"field"`
	Want   string `json:"want"`   // expected (from the device profile)
	V8     string `json:"v8"`     // V8 executor readout
	Chrome string `json:"chrome"` // Chrome executor readout ("" when unavailable)
	Pass   bool   `json:"pass"`
	Note   string `json:"note,omitempty"` // failure diagnosis
}

// ConsistencyReport is the outcome of VerifyConsistency.
type ConsistencyReport struct {
	Profile string `json:"profile"`

	// Fields is the identity comparison table.
	Fields []FieldCheck `json:"fields"`
	// AllPass: every identity field matched the profile in every executor
	// that ran.
	AllPass bool `json:"all_pass"`

	// V8Error / ChromeError carry session-level failures ("" when fine).
	V8Error    string `json:"v8_error,omitempty"`
	ChromeError string `json:"chrome_error,omitempty"`
	// ChromeRan reports whether the Chrome executor produced a readout.
	ChromeRan bool `json:"chrome_ran"`

	// Real-execution evidence (best-effort, never gates AllPass):
	// Canvas / Wasm are the Chrome executor readouts; V8Canvas / V8Wasm are
	// the V8 (mocked) counterparts, kept for contrast.
	Canvas   map[string]any `json:"canvas,omitempty"`
	Wasm     map[string]any `json:"wasm,omitempty"`
	V8Canvas map[string]any `json:"v8_canvas,omitempty"`
	V8Wasm   map[string]any `json:"v8_wasm,omitempty"`

	// RealPass: Chrome rendered a real PNG canvas AND executed real wasm.
	RealPass bool `json:"real_pass"`
}

// VerifyConsistency runs the full dual-executor check for one profile:
//
//  1. V8 session under the snapshot → identity probe.
//  2. Chrome session under the SAME snapshot → identity probe (skipped,
//     with ChromeError note, when Chrome is unavailable).
//  3. Every identity field is compared against the profile.
//  4. Canvas/wasm probes run in both executors as realism evidence.
//
// The returned error is reserved for setup failures (nil profile, no
// executors); probe mismatches land in the report, not the error.
func VerifyConsistency(ctx context.Context, profile *ProfileSnapshot, v8, chrome Executor, timeoutSec int) (*ConsistencyReport, error) {
	if profile == nil || profile.Fingerprint == nil {
		return nil, fmt.Errorf("executor: profile required")
	}
	if v8 == nil {
		return nil, fmt.Errorf("executor: v8 executor required")
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	rep := &ConsistencyReport{Profile: profile.Describe()}
	expected := ExpectedIdentity(profile.Fingerprint)

	// --- V8 readout --------------------------------------------------------
	var v8Map map[string]any
	sess, err := v8.NewSession(ctx, profile, SessionOptions{})
	if err != nil {
		rep.V8Error = err.Error()
	} else {
		v8Map, err = IdentityProbe(sess, timeoutSec)
		if err != nil {
			rep.V8Error = err.Error()
		}
		if m, err := CanvasProbe(sess, timeoutSec); err == nil {
			rep.V8Canvas = m
		}
		if m, err := WasmProbe(sess, timeoutSec); err == nil {
			rep.V8Wasm = m
		}
		sess.Dispose()
	}

	// --- Chrome readout ----------------------------------------------------
	var chromeMap map[string]any
	chromeRan := false
	if chrome != nil && chrome.Available() {
		csess, err := chrome.NewSession(ctx, profile, SessionOptions{})
		if err != nil {
			rep.ChromeError = err.Error()
		} else {
			chromeRan = true
			chromeMap, err = IdentityProbe(csess, timeoutSec)
			if err != nil {
				rep.ChromeError = err.Error()
				chromeMap = nil
			}
			if m, err := CanvasProbe(csess, timeoutSec); err == nil {
				rep.Canvas = m
			}
			if m, err := WasmProbe(csess, timeoutSec); err == nil {
				rep.Wasm = m
			}
			csess.Dispose()
		}
	} else if chrome != nil {
		rep.ChromeError = "chrome executor unavailable (no local Chrome binary?)"
	}

	// --- field-by-field comparison ----------------------------------------
	fields := make([]string, 0, len(expected))
	for k := range expected {
		fields = append(fields, k)
	}
	sort.Strings(fields)

	rep.AllPass = true
	rep.ChromeRan = chromeRan
	for _, f := range fields {
		want := canon(expected[f])
		fc := FieldCheck{Field: f, Want: want}
		v8Val, v8OK := v8Map[f]
		fc.V8 = canon(v8Val)
		if v8Map == nil {
			fc.Note = "v8 probe failed"
		} else if !v8OK {
			fc.Note = "v8: field missing"
		}
		if chromeRan {
			cVal, cOK := chromeMap[f]
			fc.Chrome = canon(cVal)
			if chromeMap == nil {
				fc.Note += " chrome probe failed"
			} else if !cOK {
				fc.Note += " chrome: field missing"
			}
		}
		fc.Pass = fc.Note == "" && fc.V8 == want && (!chromeRan || fc.Chrome == want)
		if !fc.Pass {
			rep.AllPass = false
		}
		rep.Fields = append(rep.Fields, fc)
	}

	// --- real-execution evidence ------------------------------------------
	rep.RealPass = chromeRan && realCanvas(rep.Canvas) && realWasm(rep.Wasm)
	return rep, nil
}

// canon renders a probe value (JSON-decoded) and its expected counterpart
// (Go native) in one comparable string. JSON numbers arrive as float64;
// expected values are int/float64/string — %v gives identical text for the
// same number in both routes.
func canon(v any) string {
	switch t := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return t
	case bool:
		return fmt.Sprintf("%t", t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// realCanvas: the Chrome canvas probe produced a genuine PNG of plausible
// size (the mocked V8 dataURL is far shorter than a real raster of the same
// scene; both must be PNG, only Chrome's needs real bulk).
func realCanvas(m map[string]any) bool {
	if m == nil {
		return false
	}
	png, _ := m["png"].(bool)
	length := 0
	switch l := m["len"].(type) {
	case float64:
		length = int(l)
	}
	return png && length > 2000
}

// realWasm: the Chrome executor compiled and ran the real wasm module.
func realWasm(m map[string]any) bool {
	if m == nil {
		return false
	}
	ok, _ := m["ok"].(bool)
	return ok
}

// Summary renders a compact human-readable verdict line.
func (r *ConsistencyReport) Summary() string {
	pass, total := 0, len(r.Fields)
	for _, f := range r.Fields {
		if f.Pass {
			pass++
		}
	}
	s := fmt.Sprintf("identity %d/%d pass, all_pass=%t", pass, total, r.AllPass)
	if r.RealPass {
		s += ", real canvas/wasm OK"
	}
	return s
}
