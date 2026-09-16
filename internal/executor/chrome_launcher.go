package executor

// Local Chrome discovery + isolated launch.
//
// Every Chrome the executor drives is launched HERE, on this machine, with a
// fresh throwaway user-data-dir and the debugging port bound to localhost.
// There is deliberately no way to point the executor at an already-running or
// remote browser: the dual-executor constraint (local isolated real Chrome
// only) is enforced by construction.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

// chromeCandidates is the binary search order (env override first).
var chromeCandidates = []string{
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
}

// FindChrome returns the path to a local Chrome binary, or "" when none is
// found. BES_CHROME_BIN overrides the search.
func FindChrome() string {
	if p := os.Getenv("BES_CHROME_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	for _, name := range chromeCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// devToolsListeningRe matches Chrome's startup banner, e.g.
// "DevTools listening on ws://127.0.0.1:43567/devtools/browser/<uuid>".
var devToolsListeningRe = regexp.MustCompile(`DevTools listening on (ws://[^\s]+)`)

// chromeLaunchTimeout bounds the wait for the DevTools banner.
const chromeLaunchTimeout = 15 * time.Second

// chromeProcess is one locally launched, isolated Chrome instance.
type chromeProcess struct {
	bin         string
	cmd         *exec.Cmd
	debugHTTP   string // http://127.0.0.1:<port>
	userDataDir string

	mu      sync.Mutex
	started bool
	closed  bool
}

// newChromeProcess creates the launcher handle (nothing started yet).
func newChromeProcess(bin string) *chromeProcess {
	return &chromeProcess{bin: bin}
}

// ensure starts Chrome on first use and returns the DevTools HTTP base URL
// (http://127.0.0.1:<port>). Idempotent and safe for concurrent use.
func (p *chromeProcess) ensure(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return "", fmt.Errorf("chrome: process disposed")
	}
	if p.started {
		return p.debugHTTP, nil
	}

	if err := p.launchLocked(ctx); err != nil {
		return "", err
	}
	p.started = true
	return p.debugHTTP, nil
}

// launchLocked starts Chrome and waits for the DevTools banner. p.mu held.
func (p *chromeProcess) launchLocked(ctx context.Context) error {
	debugHTTP, userDataDir, cmd, err := startChrome(p.bin, ctx)
	if err != nil {
		return err
	}
	p.debugHTTP = debugHTTP
	p.userDataDir = userDataDir
	p.cmd = cmd
	return nil
}

// startChrome launches one isolated headless Chrome with an ephemeral
// debugging port and blocks until the DevTools endpoint answers. When the
// normal sandboxed launch fails (common when running as root/in a container),
// it retries once with --no-sandbox.
func startChrome(bin string, ctx context.Context) (debugHTTP, userDataDir string, cmd *exec.Cmd, err error) {
	userDataDir, err = os.MkdirTemp("", "bes-chrome-")
	if err != nil {
		return "", "", nil, fmt.Errorf("chrome: temp user-data-dir: %w", err)
	}

	run := func(extra []string) (string, *exec.Cmd, error) {
		args := chromeArgs(userDataDir)
		args = append(args, extra...)
		c := exec.Command(bin, args...)
		stderr, err := c.StderrPipe()
		if err != nil {
			return "", nil, err
		}
		if err := c.Start(); err != nil {
			return "", nil, err
		}
		wsURL, err := waitDevTools(stderr)
		if err != nil {
			// Best-effort cleanup of the failed attempt.
			_ = c.Process.Kill()
			_, _ = c.Process.Wait()
			return "", nil, err
		}
		u, err := url.Parse(wsURL)
		if err != nil {
			return "", nil, err
		}
		return "http://" + u.Host, c, nil
	}

	base, c, err := run(nil)
	if err != nil {
		// Retry without the Chrome sandbox (containers / root). The browser is
		// still a local, isolated, short-lived instance.
		base2, c2, err2 := run([]string{"--no-sandbox"})
		if err2 != nil {
			os.RemoveAll(userDataDir)
			return "", "", nil, fmt.Errorf("chrome: launch failed: %v (retry with --no-sandbox failed: %v)", err, err2)
		}
		return base2, userDataDir, c2, nil
	}
	return base, userDataDir, c, nil
}

// chromeArgs builds the launch flags for an isolated headless Chrome:
//   - --headless + --remote-debugging-port=0 (ephemeral port, bound to
//     127.0.0.1 by Chrome itself)
//   - fresh --user-data-dir (also required for remote debugging since
//     Chrome 136)
//   - --disable-blink-features=AutomationControlled keeps the automation
//     surface (navigator.webdriver etc.) at its default "clean" state
//   - --enable-unsafe-swiftshader so software WebGL works on GPU-less hosts
//     (canvas/WebGL points must actually render)
//   - no --enable-automation, no first-run noise
func chromeArgs(userDataDir string) []string {
	return []string{
		"--headless",
		"--remote-debugging-port=0",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-client-side-phishing-detection",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-hang-monitor",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-sync",
		"--disable-blink-features=AutomationControlled",
		"--enable-unsafe-swiftshader",
		"--mute-audio",
		"about:blank",
	}
}

// waitDevTools scans Chrome's stderr for the DevTools banner.
func waitDevTools(stderr io.Reader) (string, error) {
	br := bufio.NewReader(stderr)
	deadline := time.After(chromeLaunchTimeout)
	found := make(chan string, 1)
	go func() {
		for {
			line, err := br.ReadString('\n')
			if m := devToolsListeningRe.FindStringSubmatch(line); m != nil {
				found <- m[1]
				return
			}
			if err != nil {
				close(found)
				return
			}
		}
	}()
	select {
	case wsURL, ok := <-found:
		if !ok || wsURL == "" {
			return "", fmt.Errorf("chrome: exited before the DevTools endpoint came up")
		}
		// The banner host must be local — enforced centrally, belt-and-braces
		// with cdpDial's own validation.
		if _, err := validateLocalWSURL(wsURL); err != nil {
			return "", err
		}
		return wsURL, nil
	case <-deadline:
		return "", fmt.Errorf("chrome: DevTools endpoint did not come up within %s", chromeLaunchTimeout)
	}
}

// dispose kills the exact Chrome process this handle started (never by
// executable name — a name match could catch an unrelated long-lived
// browser), waits for it, and removes the throwaway user-data-dir.
func (p *chromeProcess) dispose() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	if !p.started {
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
	if p.userDataDir != "" {
		os.RemoveAll(p.userDataDir)
	}
}

// newPageTarget creates a fresh page target on the running Chrome and
// returns its webSocketDebuggerUrl plus target id. /json/new requires PUT
// since Chrome 111.
func (p *chromeProcess) newPageTarget(ctx context.Context) (wsURL, targetID string, err error) {
	base, err := p.ensure(ctx)
	if err != nil {
		return "", "", err
	}
	data, err := devToolsHTTP(ctx, "PUT", base+"/json/new?about:blank")
	if err != nil {
		return "", "", err
	}
	var target struct {
		ID                   string `json:"id"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		Type                 string `json:"type"`
	}
	if err := json.Unmarshal(data, &target); err != nil {
		return "", "", fmt.Errorf("chrome: decode /json/new: %w", err)
	}
	if target.WebSocketDebuggerURL == "" {
		return "", "", fmt.Errorf("chrome: /json/new returned no webSocketDebuggerUrl")
	}
	return target.WebSocketDebuggerURL, target.ID, nil
}
