package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ChromeExecutor runs fingerprint points in a real, locally launched,
// isolated Chrome over CDP. Canvas/WebGL readbacks, AudioContext DSP and
// WebAssembly execute in a genuine engine; the identity surface is pinned to
// the device profile (Emulation overrides + injected script), so the same
// ProfileSnapshot reads identically here and in the V8 sandbox.
//
// One Chrome process is shared by all sessions (each session gets its own
// page target). The process is started lazily on first use and killed on
// Dispose — by exact PID, never by executable name.
type ChromeExecutor struct {
	bin     string
	process *chromeProcess
}

// NewChromeExecutor returns a Chrome executor using the discovered local
// Chrome binary. When no binary is found the executor still exists but
// reports Available() == false (the Router then stays on V8).
func NewChromeExecutor() *ChromeExecutor {
	bin := FindChrome()
	return &ChromeExecutor{bin: bin, process: newChromeProcess(bin)}
}

// NewChromeExecutorWithBinary is NewChromeExecutor with an explicit binary
// path (used by tests). The path must exist.
func NewChromeExecutorWithBinary(bin string) *ChromeExecutor {
	return &ChromeExecutor{bin: bin, process: newChromeProcess(bin)}
}

// Name implements Executor.
func (e *ChromeExecutor) Name() string { return "chrome" }

// Available implements Executor — cheap check that a Chrome binary was found.
// It does not launch anything.
func (e *ChromeExecutor) Available() bool { return e.bin != "" }

// NewSession implements Executor.
//
// Location handling: the Chrome executor evaluates fingerprint points on an
// inert document — only about: and data: URLs are accepted (empty →
// about:blank). Fetching real pages through this executor is out of scope by
// design: the sandbox + netlayer own live traffic.
func (e *ChromeExecutor) NewSession(ctx context.Context, profile *ProfileSnapshot, opts SessionOptions) (Session, error) {
	if profile == nil || profile.Fingerprint == nil {
		return nil, fmt.Errorf("executor: profile required")
	}
	loc := opts.Location
	if loc == "" {
		loc = "about:blank"
	}
	if err := validateInertLocation(loc); err != nil {
		return nil, err
	}

	wsURL, targetID, err := e.process.newPageTarget(ctx)
	if err != nil {
		return nil, fmt.Errorf("chrome: new page target: %w", err)
	}
	conn, err := cdpDial(ctx, wsURL)
	if err != nil {
		e.closeTarget(context.WithoutCancel(ctx), targetID)
		return nil, err
	}

	sess := &chromeSession{exec: e, conn: conn, targetID: targetID}
	if err := sess.applyProfile(ctx, profile); err != nil {
		sess.Dispose()
		return nil, err
	}
	if loc != "about:blank" {
		if err := conn.call(ctx, "Page.navigate", map[string]any{"url": loc}, nil); err != nil {
			sess.Dispose()
			return nil, fmt.Errorf("chrome: navigate to %q: %w", loc, err)
		}
	}
	return sess, nil
}

// Dispose implements Executor — kills the Chrome process (exact PID) and
// removes its throwaway user-data-dir.
func (e *ChromeExecutor) Dispose() { e.process.dispose() }

// validateInertLocation enforces the inert-document policy.
func validateInertLocation(loc string) error {
	switch {
	case loc == "about:blank" || loc == "about:":
		return nil
	case len(loc) > 5 && loc[:5] == "data:":
		return nil
	default:
		return fmt.Errorf("chrome executor: location %q rejected — only about:/data: inert documents are allowed (fingerprint-point execution, not page fetching)", loc)
	}
}

// closeTarget closes a page target via the DevTools HTTP endpoint.
func (e *ChromeExecutor) closeTarget(ctx context.Context, targetID string) {
	base, err := e.process.ensure(ctx)
	if err != nil {
		return
	}
	_, _ = devToolsHTTP(ctx, "GET", base+"/json/close/"+targetID)
}

// chromeSession is one page target driven by one profile.
type chromeSession struct {
	exec     *ChromeExecutor
	conn     *cdpConn
	targetID string

	mu       sync.Mutex
	disposed bool
}

// applyProfile pins the identity surface to the profile: CDP-native for the
// pieces the browser must own end-to-end (UA → HTTP headers + iframes,
// timezone → Date/Intl), injected script for the rest (navigator extras,
// screen/window geometry, WebGL identity constants).
func (s *chromeSession) applyProfile(ctx context.Context, profile *ProfileSnapshot) error {
	fp := profile.Fingerprint

	if err := s.conn.call(ctx, "Page.enable", nil, nil); err != nil {
		return fmt.Errorf("chrome: Page.enable: %w", err)
	}

	ua := profile.UserAgent()
	if ua == "" {
		return fmt.Errorf("chrome: profile has no user agent")
	}
	acceptLang := "en"
	if len(fp.Languages) > 0 {
		acceptLang = fp.Languages[0]
	}
	platform := fp.OS.Platform
	if p, ok := fp.Navigator["platform"].(string); ok && p != "" {
		platform = p
	}
	if err := s.conn.call(ctx, "Emulation.setUserAgentOverride", map[string]any{
		"userAgent":      ua,
		"platform":       platform,
		"acceptLanguage": acceptLang,
	}, nil); err != nil {
		return fmt.Errorf("chrome: setUserAgentOverride: %w", err)
	}

	if fp.Timezone != "" {
		if err := s.conn.call(ctx, "Emulation.setTimezoneOverride", map[string]any{
			"timezoneId": fp.Timezone,
		}, nil); err != nil {
			return fmt.Errorf("chrome: setTimezoneOverride(%s): %w", fp.Timezone, err)
		}
	}

	injectJS, err := BuildInjectionJS(fp)
	if err != nil {
		return err
	}
	// Register for every future document, then apply to the already-loaded
	// about:blank directly so the session is consistent immediately.
	if err := s.conn.call(ctx, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": injectJS,
	}, nil); err != nil {
		return fmt.Errorf("chrome: addScriptToEvaluateOnNewDocument: %w", err)
	}
	if _, err := s.evalNow(ctx, injectJS); err != nil {
		return fmt.Errorf("chrome: apply injection: %w", err)
	}
	return nil
}

// Executor implements Session.
func (s *chromeSession) Executor() string { return "chrome" }

// Eval implements Session — Runtime.evaluate with awaitPromise, bounded by
// ctx (plus the explicit timeout when shorter).
func (s *chromeSession) Eval(ctx context.Context, code string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	disposed := s.disposed
	s.mu.Unlock()
	if disposed {
		return "", fmt.Errorf("chrome: session disposed")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return s.evalNow(ctx, code)
}

func (s *chromeSession) evalNow(ctx context.Context, code string) (string, error) {
	params := map[string]any{
		"expression":   code,
		"returnByValue": true,
		"awaitPromise": true,
	}
	var out struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text       string `json:"text"`
			Exception  *struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := s.conn.call(ctx, "Runtime.evaluate", params, &out); err != nil {
		return "", err
	}
	if out.ExceptionDetails != nil {
		msg := out.ExceptionDetails.Text
		if out.ExceptionDetails.Exception != nil && out.ExceptionDetails.Exception.Description != "" {
			msg = out.ExceptionDetails.Exception.Description
		}
		return "", fmt.Errorf("chrome: JS exception: %s", msg)
	}
	switch out.Result.Type {
	case "string":
		var s string
		if err := json.Unmarshal(out.Result.Value, &s); err != nil {
			return string(out.Result.Value), nil
		}
		return s, nil
	case "undefined":
		return "undefined", nil
	case "null":
		return "null", nil
	default:
		return string(out.Result.Value), nil
	}
}

// Dispose implements Session — closes the CDP connection and the page target.
func (s *chromeSession) Dispose() {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	s.disposed = true
	s.mu.Unlock()
	s.conn.close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.exec.closeTarget(ctx, s.targetID)
}
