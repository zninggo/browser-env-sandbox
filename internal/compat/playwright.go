// Package compat provides a Playwright-compatible API layer.
// Allows existing Playwright/Puppeteer code to work with browser-env-sandbox
// by mapping Playwright concepts to sandbox sessions.
//
// Usage:
//   browser = await bes.launch({ browser: "chrome", os: "windows" })
//   page = await browser.newPage()
//   await page.goto("https://example.com")
//   result = await page.evaluate("navigator.userAgent")
//   await browser.close()
package compat

import (
	"fmt"
	"sync"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
)

// Launcher creates browser instances (Playwright-style).
type Launcher struct {
	fpEngine *fpengine.Engine
	sbEngine *sandbox.Engine
	mu       sync.Mutex
}

// NewLauncher creates a Playwright-compatible launcher.
func NewLauncher(poolSize int) *Launcher {
	fpEng := fpengine.New()
	return &Launcher{
		fpEngine: fpEng,
		sbEngine: sandbox.New(fpEng, poolSize),
	}
}

// LaunchOptions mirrors Playwright's BrowserType.launch options.
type LaunchOptions struct {
	Browser string `json:"browser"`
	OS      string `json:"os"`
	Seed    uint64 `json:"seed"`
	Headless bool  `json:"headless"` // ignored (always headless in V8)
}

// Browser represents a launched browser instance.
type Browser struct {
	launcher *Launcher
	session  *sandbox.Session
	pages    []*Page
	mu       sync.Mutex
}

// Page represents a browser page (maps to a sandbox session).
type Page struct {
	session *sandbox.Session
	url     string
}

// Launch creates a new browser instance (Playwright-style).
func (l *Launcher) Launch(opts LaunchOptions) (*Browser, error) {
	if opts.Browser == "" {
		opts.Browser = "chrome"
	}
	if opts.OS == "" {
		opts.OS = "windows"
	}

	sess, err := l.sbEngine.CreateSession(api.SessionOptions{
		Browser:  opts.Browser,
		OS:       opts.OS,
		Seed:     opts.Seed,
		Location: "about:blank",
	})
	if err != nil {
		return nil, fmt.Errorf("launch failed: %w", err)
	}

	browser := &Browser{
		launcher: l,
		session:  sess,
	}
	return browser, nil
}

// NewPage creates a new page in the browser.
func (b *Browser) NewPage() *Page {
	page := &Page{session: b.session}
	b.mu.Lock()
	b.pages = append(b.pages, page)
	b.mu.Unlock()
	return page
}

// Close closes the browser.
func (b *Browser) Close() {
	b.session.Dispose()
}

// Goto navigates to a URL (sets document.URL in sandbox).
func (p *Page) Goto(url string) error {
	p.url = url
	// In sandbox, "navigation" means updating location
	_, err := p.session.Eval(fmt.Sprintf(`
		(function() {
			// Update location-related properties
			try {
				Object.defineProperty(location, 'href', {value: %q, configurable: true});
				Object.defineProperty(document, 'URL', {value: %q, configurable: true});
			} catch(e) {}
		})()
	`, url, url))
	return err
}

// Evaluate executes JS in the page context.
func (p *Page) Evaluate(expression string) (string, error) {
	return p.session.Eval(expression)
}

// EvaluateAsync executes JS with timeout.
func (p *Page) EvaluateAsync(expression string, timeout time.Duration) (string, error) {
	return p.session.EvalWithTimeout(expression, timeout)
}

// Screenshot is a no-op (V8 sandbox has no visual rendering).
// Returns a 1x1 transparent PNG for API compatibility.
func (p *Page) Screenshot() string {
	return "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
}

// Content returns the page's document content (innerHTML).
func (p *Page) Content() string {
	result, _ := p.session.Eval("document.documentElement.innerHTML || ''")
	return result
}

// Title returns the page title.
func (p *Page) Title() string {
	result, _ := p.session.Eval("document.title || ''")
	return result
}

// Url returns the current page URL.
func (p *Page) Url() string {
	if p.url != "" {
		return p.url
	}
	result, _ := p.session.Eval("location.href || ''")
	return result
}

// SetCookie sets cookies on the page.
func (p *Page) SetCookie(name, value string) {
	p.session.SetCookie(name, value)
}

// Cookies returns all cookies.
func (p *Page) Cookies() string {
	return p.session.GetCookies()
}

// WaitForTimeout waits for the given duration (flushes timers).
func (p *Page) WaitForTimeout(d time.Duration) {
	p.session.FlushTimers(d)
}

// AddScriptTag injects a script into the page.
func (p *Page) AddScriptTag(name, content string) error {
	return p.session.LoadScript(name, content)
}

// Click simulates a click on an element matching the selector.
func (p *Page) Click(selector string) error {
	_, err := p.session.Eval(fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (el && el.humanClick) { el.humanClick(); }
			else if (el) { el.click(); }
			return el ? true : false;
		})()
	`, selector))
	return err
}

// Fill types text into an input matching the selector.
func (p *Page) Fill(selector, text string) error {
	_, err := p.session.Eval(fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (el && el.humanType) { el.humanType(%q); }
			else if (el) { el.value = %q; el.dispatchEvent({type:'input'}); el.dispatchEvent({type:'change'}); }
			return el ? true : false;
		})()
	`, selector, text, text))
	return err
}

// WaitForSelector waits for an element to appear (polls with setTimeout).
func (p *Page) WaitForSelector(selector string, timeout time.Duration) error {
	deadline := time.After(timeout)
	for {
		result, _ := p.session.Eval(fmt.Sprintf(`document.querySelector(%q) !== null`, selector))
		if result == "true" {
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for selector: %s", selector)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Dispose cleans up the launcher.
func (l *Launcher) Dispose() {
	l.sbEngine.Dispose()
}
