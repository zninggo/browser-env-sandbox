package executor

// Real-Chrome integration tests for the CDP executor. They skip when no
// local Chrome binary is found (CI containers) — the dual-executor
// constraint that Chrome is ALWAYS local makes these hermetic: no network,
// no remote endpoints, throwaway user-data-dir per run.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
)

func chromeIntegration(t *testing.T) *ChromeExecutor {
	t.Helper()
	bin := FindChrome()
	if bin == "" {
		t.Skip("no local Chrome binary found (set BES_CHROME_BIN to override)")
	}
	if os.Getenv("BES_TEST_NO_CHROME") != "" {
		t.Skip("BES_TEST_NO_CHROME set")
	}
	return NewChromeExecutorWithBinary(bin)
}

func TestChromeExecutorSessionIdentity(t *testing.T) {
	chrome := chromeIntegration(t)
	defer chrome.Dispose()

	eng := fpengine.New()
	snap, err := TakeSnapshot(eng, 424242, "chrome", "windows", "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}

	sess, err := chrome.NewSession(context.Background(), snap, SessionOptions{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Dispose()
	if sess.Executor() != "chrome" {
		t.Errorf("Executor() = %q", sess.Executor())
	}

	probe, err := IdentityProbe(sess, 30)
	if err != nil {
		t.Fatal(err)
	}
	expected := ExpectedIdentity(snap.Fingerprint)
	for field, want := range expected {
		if got := canon(probe[field]); got != canon(want) {
			t.Errorf("chrome identity %s = %q, want %q", field, got, canon(want))
		}
	}
}

func TestChromeExecutorRealCanvasAndWasm(t *testing.T) {
	chrome := chromeIntegration(t)
	defer chrome.Dispose()

	eng := fpengine.New()
	snap, err := TakeSnapshot(eng, 777, "chrome", "windows", "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := chrome.NewSession(context.Background(), snap, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Dispose()

	canvas, err := CanvasProbe(sess, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !realCanvas(canvas) {
		t.Errorf("canvas not real: %v", canvas)
	}
	wasm, err := WasmProbe(sess, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !realWasm(wasm) {
		t.Errorf("wasm not real: %v", wasm)
	}
}

func TestChromeExecutorRejectsNonLocalLocation(t *testing.T) {
	chrome := chromeIntegration(t)
	defer chrome.Dispose()
	snap := newTestSnapshot(t, "chrome", "windows", 1)
	_, err := chrome.NewSession(context.Background(), snap, SessionOptions{Location: "https://example.com/"})
	if err == nil {
		t.Fatal("expected rejection of non-inert location")
	}
}

func TestRouterEndToEndChrome(t *testing.T) {
	// Full dual-executor routing with real backends: canvas routes to the
	// real Chrome, js stays on V8, both under the SAME profile.
	chrome := chromeIntegration(t)
	defer chrome.Dispose()

	eng := fpengine.New()
	v8 := NewV8Executor(eng, 1)
	defer v8.Dispose()

	r, err := NewRouter(v8, chrome)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := TakeSnapshot(eng, 31337, "chrome", "windows", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	// js class → v8
	res, err := r.Eval(context.Background(), PointJS, snap, "1+1", SessionOptions{}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Executor != "v8" || res.Value != "2" {
		t.Errorf("js route: %+v", res)
	}

	// canvas class → chrome, real rasterization
	res, err = r.Eval(context.Background(), PointCanvas, snap, canvasProbeJS, SessionOptions{}, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Executor != "chrome" {
		t.Errorf("canvas route executor = %q, want chrome (fallback=%s)", res.Executor, res.Fallback)
	}
	m, err := parseProbeJSON(res.Value)
	if err != nil {
		t.Fatal(err)
	}
	if !realCanvas(m) {
		t.Errorf("canvas via router not real: %v", m)
	}

	// Both executors agree on the identity under this profile.
	rep, err := VerifyConsistency(context.Background(), snap, v8, chrome, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.AllPass {
		for _, f := range rep.Fields {
			if !f.Pass {
				t.Errorf("field %s: want=%q v8=%q chrome=%q note=%s", f.Field, f.Want, f.V8, f.Chrome, f.Note)
			}
		}
	}
	if !rep.RealPass {
		t.Errorf("real execution not confirmed: canvas=%v wasm=%v", rep.Canvas, rep.Wasm)
	}
}
