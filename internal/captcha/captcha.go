// Package captcha provides CAPTCHA recognition integration for the sandbox.
//
// It exposes a Solver interface with a pluggable backend architecture:
// callers register a solver (e.g. 2captcha, anti-captcha, local OCR) and the
// sandbox/bridge can invoke it when a page returns a CAPTCHA challenge.
//
// The default NoOpSolver does nothing — real deployments inject a solver via
// SetSolver.
package captcha

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CaptchaType identifies the kind of CAPTCHA challenge.
type CaptchaType string

const (
	TypeImage   CaptchaType = "image"    // image-based text CAPTCHA
	TypeReCaptcha CaptchaType = "recaptcha" // Google reCAPTCHA v2/v3
	TypeHCaptcha CaptchaType = "hcaptcha"   // hCaptcha
	TypeTurnstile CaptchaType = "turnstile" // Cloudflare Turnstile
	TypeGeeTest  CaptchaType = "geetest"  // GeeTest slider/click
)

// Challenge represents a CAPTCHA challenge to solve.
type Challenge struct {
	Type     CaptchaType `json:"type"`
	ImageB64 string      `json:"image_b64,omitempty"` // base64-encoded image (for TypeImage)
	SiteKey  string      `json:"site_key,omitempty"`  // reCAPTCHA/hCaptcha/Turnstile site key
	PageURL  string      `json:"page_url"`            // the page where the CAPTCHA appears
	Action   string      `json:"action,omitempty"`    // reCAPTCHA v3 action
	MinScore float64     `json:"min_score,omitempty"` // reCAPTCHA v3 minimum score (0-1)
}

// Solution is the result of solving a CAPTCHA.
type Solution struct {
	Token    string `json:"token,omitempty"`    // g-recaptcha-response / h-captcha-response / turnstile token
	Text     string `json:"text,omitempty"`     // decoded text (for TypeImage)
	Cookies  map[string]string `json:"cookies,omitempty"` // cookies set after solving
	Score    float64 `json:"score,omitempty"`   // reCAPTCHA v3 score
	Reason   string  `json:"reason,omitempty"`  // error reason if solved=false
	Solved   bool    `json:"solved"`
}

// Solver solves CAPTCHA challenges. Implementations may call external APIs
// (2captcha, anti-captcha), local OCR (tesseract), or ML models.
type Solver interface {
	Solve(ctx context.Context, challenge Challenge) (*Solution, error)
	Name() string
}

// NoOpSolver is the default no-op solver. It returns an error indicating no
// solver is configured. Deployments inject a real solver via SetSolver.
type NoOpSolver struct{}

func (n *NoOpSolver) Solve(ctx context.Context, challenge Challenge) (*Solution, error) {
	return &Solution{Solved: false, Reason: "no captcha solver configured"}, nil
}
func (n *NoOpSolver) Name() string { return "noop" }

// Registry holds the active solver. It is safe for concurrent use.
var (
	mu     sync.RWMutex
	solver Solver = &NoOpSolver{}
)

// SetSolver registers a CAPTCHA solver. Pass nil to revert to the no-op solver.
func SetSolver(s Solver) {
	mu.Lock()
	defer mu.Unlock()
	if s == nil {
		solver = &NoOpSolver{}
	} else {
		solver = s
	}
}

// GetSolver returns the currently registered solver.
func GetSolver() Solver {
	mu.RLock()
	defer mu.RUnlock()
	return solver
}

// Solve is a convenience wrapper that calls the registered solver with a
// 120-second timeout (typical for 2captcha-style APIs).
func Solve(challenge Challenge) (*Solution, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return GetSolver().Solve(ctx, challenge)
}

// ErrNoSolver is returned when no solver is configured.
var ErrNoSolver = errors.New("no captcha solver configured")
