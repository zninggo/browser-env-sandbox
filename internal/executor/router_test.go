package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stub executors for routing/fallback semantics (no V8, no Chrome).

type stubExecutor struct {
	name      string
	available bool
	evalOut   string
	evalErr   error
	sessErr   error
	disposed  int
}

func (s *stubExecutor) Name() string    { return s.name }
func (s *stubExecutor) Available() bool { return s.available }
func (s *stubExecutor) Dispose()        { s.disposed++ }

func (s *stubExecutor) NewSession(ctx context.Context, profile *ProfileSnapshot, opts SessionOptions) (Session, error) {
	if s.sessErr != nil {
		return nil, s.sessErr
	}
	return &stubSession{owner: s}, nil
}

type stubSession struct {
	owner *stubExecutor
}

func (s *stubSession) Executor() string { return s.owner.name }
func (s *stubSession) Eval(ctx context.Context, code string, timeout time.Duration) (string, error) {
	if s.owner.evalErr != nil {
		return "", s.owner.evalErr
	}
	return s.owner.evalOut, nil
}
func (s *stubSession) Dispose() {}

func TestRouteDefaultV8(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true}
	chrome := &stubExecutor{name: "chrome", available: true}
	r, err := NewRouter(v8, chrome)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []PointClass{PointJS, PointNavigator, PointScreen, PointFonts} {
		if got := r.Route(c); got != v8 {
			t.Errorf("Route(%s) = %s, want v8", c, got.Name())
		}
	}
	for _, c := range []PointClass{PointCanvas, PointWebGL, PointAudio, PointWasm} {
		if got := r.Route(c); got != chrome {
			t.Errorf("Route(%s) = %s, want chrome", c, got.Name())
		}
	}
}

func TestRouteChromeUnavailable(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true}
	// Chrome missing entirely.
	r, _ := NewRouter(v8, nil)
	if got := r.Route(PointCanvas); got != v8 {
		t.Errorf("Route(canvas) with nil chrome = %s, want v8", got.Name())
	}
	// Chrome present but unavailable (no binary).
	chrome := &stubExecutor{name: "chrome", available: false}
	r2, _ := NewRouter(v8, chrome)
	if got := r2.Route(PointWasm); got != v8 {
		t.Errorf("Route(wasm) with unavailable chrome = %s, want v8", got.Name())
	}
}

func TestRouterEvalRealClass(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true, evalOut: "mock"}
	chrome := &stubExecutor{name: "chrome", available: true, evalOut: "real"}
	r, _ := NewRouter(v8, chrome)

	res, err := r.Eval(context.Background(), PointCanvas, nil, "code", SessionOptions{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != "real" || res.Executor != "chrome" {
		t.Errorf("got %+v, want real/chrome", res)
	}
	if res.Fallback != "" {
		t.Errorf("unexpected fallback: %s", res.Fallback)
	}
	routed, _ := r.Stats()
	if routed[PointCanvas] != 1 {
		t.Errorf("routed[canvas] = %d, want 1", routed[PointCanvas])
	}
}

func TestRouterEvalFallbackOnChromeFailure(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true, evalOut: "mock"}
	chrome := &stubExecutor{name: "chrome", available: true, evalErr: errors.New("chrome crashed")}
	var fbClass PointClass
	var fbFrom string
	r, _ := NewRouter(v8, chrome)
	r.OnFallback = func(c PointClass, from string, err error) {
		fbClass, fbFrom = c, from
	}

	res, err := r.Eval(context.Background(), PointWasm, nil, "code", SessionOptions{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != "mock" {
		t.Errorf("fallback value = %q, want mock", res.Value)
	}
	if res.Executor != "v8(fallback:chrome)" {
		t.Errorf("executor = %q, want v8(fallback:chrome)", res.Executor)
	}
	if res.Fallback == "" {
		t.Error("Fallback reason is empty")
	}
	if fbClass != PointWasm || fbFrom != "chrome" {
		t.Errorf("OnFallback got %s/%s, want wasm/chrome", fbClass, fbFrom)
	}
	_, fell := r.Stats()
	if fell[PointWasm] != 1 {
		t.Errorf("fellBack[wasm] = %d, want 1", fell[PointWasm])
	}
}

func TestRouterEvalFallbackOnSessionFailure(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true, evalOut: "mock"}
	chrome := &stubExecutor{name: "chrome", available: true, sessErr: errors.New("launch failed")}
	r, _ := NewRouter(v8, chrome)
	res, err := r.Eval(context.Background(), PointCanvas, nil, "code", SessionOptions{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Executor != "v8(fallback:chrome)" || res.Value != "mock" {
		t.Errorf("got %+v, want v8 fallback with mock value", res)
	}
}

func TestRouterEvalBothFail(t *testing.T) {
	v8 := &stubExecutor{name: "v8", available: true, evalErr: errors.New("v8 error")}
	chrome := &stubExecutor{name: "chrome", available: true, evalErr: errors.New("chrome error")}
	r, _ := NewRouter(v8, chrome)
	_, err := r.Eval(context.Background(), PointCanvas, nil, "code", SessionOptions{}, time.Second)
	if err == nil {
		t.Fatal("expected error when both executors fail")
	}
}

func TestRouterRequiresDefault(t *testing.T) {
	if _, err := NewRouter(nil, nil); err == nil {
		t.Fatal("expected error for nil default executor")
	}
}
