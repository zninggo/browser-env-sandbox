package executor

// V8-side integration tests: the real fpengine + the real V8 sandbox, no
// Chrome required. These pin the "profile drives the executor" contract on
// the default path.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
)

// TestMain moves the test cwd to the repo root: the fingerprint engine
// resolves its data files (data/device_profiles.json, data/fp_real_data.json)
// relative to the cwd, and from the package dir they are missing — the
// legacy-embedded fallback then cannot produce firefox/safari profiles at
// all. Running against the real data keeps the tests on the production path.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		// Package ran from an unexpected location; keep going — tests that
		// need the real data will fail loudly rather than silently pass.
		os.Stderr.WriteString("executor tests: chdir to repo root failed: " + err.Error() + "\n")
	} else {
		fpengine.ReloadDeviceProfiles()
	}
	os.Exit(m.Run())
}

func newTestSnapshot(t *testing.T, browser, osName string, seed uint64) *ProfileSnapshot {
	t.Helper()
	eng := fpengine.New()
	snap, err := TakeSnapshot(eng, seed, browser, osName, "")
	if err != nil {
		t.Fatalf("TakeSnapshot: %v", err)
	}
	return snap
}

func TestBuildInjectionJSChromeProfile(t *testing.T) {
	snap := newTestSnapshot(t, "chrome", "windows", 42)
	js, err := BuildInjectionJS(snap.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}

	// The payload must carry the profile identity.
	for _, want := range []string{
		snap.UserAgent(),
		snap.Fingerprint.OS.Platform,
		`"webglParams"`,
		`"37445"`,
		`"37446"`,
		"userAgentData",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("injection JS missing %q", want)
		}
	}

	// The embedded payload must be valid JSON: extract between "var P = " and
	// the following semicolon.
	i := strings.Index(js, "var P = ")
	j := strings.Index(js[i:], ";\n")
	if i < 0 || j < 0 {
		t.Fatal("cannot locate payload in injection JS")
	}
	var payload injectionPayload
	if err := json.Unmarshal([]byte(js[i+len("var P = "):i+j]), &payload); err != nil {
		t.Fatalf("embedded payload is not valid JSON: %v", err)
	}
	if payload.UserAgent != snap.UserAgent() {
		t.Errorf("payload UA = %q, want %q", payload.UserAgent, snap.UserAgent())
	}
	if payload.WebGLParams["37446"] != snap.Fingerprint.WebGL.Renderer {
		t.Errorf("payload renderer param mismatch")
	}
}

func TestBuildInjectionJSFirefoxHasNoUserAgentData(t *testing.T) {
	snap := newTestSnapshot(t, "firefox", "windows", 5)
	if snap.Fingerprint.Browser.Name != "firefox" {
		t.Fatalf("expected a firefox profile, got %s (device profile data missing?)", snap.Fingerprint.Browser.Name)
	}
	js, err := BuildInjectionJS(snap.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	// Non-Chromium profile: payload userAgentData must be null so the script
	// pins navigator.userAgentData to undefined instead of leaking Chrome's.
	if !strings.Contains(js, `"userAgentData":null`) {
		t.Error("firefox profile payload should carry userAgentData:null")
	}
	if !strings.Contains(js, "P.userAgentData === null ? undefined : P.userAgentData") {
		t.Error("script should map null userAgentData to undefined")
	}
}

func TestV8ExecutorIdentityMatchesProfile(t *testing.T) {
	snap := newTestSnapshot(t, "chrome", "windows", 42)
	v8 := NewV8Executor(fpengine.New(), 1)
	defer v8.Dispose()

	sess, err := v8.NewSession(context.Background(), snap, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Dispose()
	if sess.Executor() != "v8" {
		t.Errorf("Executor() = %q, want v8", sess.Executor())
	}

	probe, err := IdentityProbe(sess, 15)
	if err != nil {
		t.Fatal(err)
	}
	expected := ExpectedIdentity(snap.Fingerprint)
	for field, want := range expected {
		got := canon(probe[field])
		if got != canon(want) {
			t.Errorf("v8 identity %s = %q, want %q", field, got, canon(want))
		}
	}
}

func TestV8ExecutorSameProfileSameFingerprint(t *testing.T) {
	// Two V8 sessions from ONE snapshot must produce identical probe
	// readouts (deterministic, profile-driven — not re-sampled per session).
	snap := newTestSnapshot(t, "chrome", "macos", 7)
	v8 := NewV8Executor(fpengine.New(), 2)
	defer v8.Dispose()

	readouts := make([]map[string]any, 2)
	for i := range readouts {
		sess, err := v8.NewSession(context.Background(), snap, SessionOptions{})
		if err != nil {
			t.Fatal(err)
		}
		readouts[i], err = IdentityProbe(sess, 15)
		if err != nil {
			sess.Dispose()
			t.Fatal(err)
		}
		sess.Dispose()
	}
	for field := range readouts[0] {
		if canon(readouts[0][field]) != canon(readouts[1][field]) {
			t.Errorf("non-deterministic identity field %s: %v vs %v",
				field, readouts[0][field], readouts[1][field])
		}
	}
}

func TestVerifyConsistencyV8Only(t *testing.T) {
	// Chrome executor without a binary: the report must still verify the V8
	// side and record Chrome's unavailability instead of failing.
	snap := newTestSnapshot(t, "chrome", "windows", 1)
	v8 := NewV8Executor(fpengine.New(), 1)
	defer v8.Dispose()

	chrome := &unavailableChrome{}
	rep, err := VerifyConsistency(context.Background(), snap, v8, chrome, 15)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.AllPass {
		for _, f := range rep.Fields {
			if !f.Pass {
				t.Errorf("field %s failed: note=%s", f.Field, f.Note)
			}
		}
	}
	if rep.ChromeRan {
		t.Error("ChromeRan should be false")
	}
	if rep.RealPass {
		t.Error("RealPass should be false without Chrome")
	}
}

type unavailableChrome struct{}

func (u *unavailableChrome) Name() string    { return "chrome" }
func (u *unavailableChrome) Available() bool { return false }
func (u *unavailableChrome) Dispose()        {}

func (u *unavailableChrome) NewSession(ctx context.Context, p *ProfileSnapshot, o SessionOptions) (Session, error) {
	return nil, errors.New("chrome unavailable")
}

func TestParseProbeJSONDoubleStringified(t *testing.T) {
	// A session adapter that JSON-stringifies a string result produces a
	// quoted-JSON payload; parseProbeJSON must unwrap it.
	raw := `"{\"ua\":\"x\"}"`
	m, err := parseProbeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m["ua"] != "x" {
		t.Errorf("ua = %v, want x", m["ua"])
	}
}

func TestSnapshotWrapsFingerprint(t *testing.T) {
	eng := fpengine.New()
	fp, err := eng.Generate(99, "chrome", "windows")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := NewSnapshot(fp, fp.Seed, "chrome", "windows", "")
	if err != nil {
		t.Fatal(err)
	}
	if snap.UserAgent() == "" {
		t.Error("snapshot UA empty")
	}
	if snap.Describe() == "" {
		t.Error("snapshot Describe empty")
	}
	if _, err := NewSnapshot(nil, 0, "", "", ""); err == nil {
		t.Error("nil fingerprint should be rejected")
	}
}

func TestEvalTimedRespectsTimeout(t *testing.T) {
	v8 := NewV8Executor(fpengine.New(), 1)
	defer v8.Dispose()
	snap := newTestSnapshot(t, "chrome", "windows", 3)
	sess, err := v8.NewSession(context.Background(), snap, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Dispose()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// An infinite loop must be cut short by the context deadline.
	_, err = sess.Eval(ctx, "while(true){}", 30*time.Second)
	if err == nil {
		t.Error("expected deadline error for infinite loop")
	}
}
