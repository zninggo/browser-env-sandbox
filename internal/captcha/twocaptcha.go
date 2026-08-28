package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwoCaptchaSolver integrates with the 2captcha.com API (and compatible
// services like anti-captcha that use the same in.php/res.php protocol).
//
// Usage:
//
//	captcha.SetSolver(&captcha.TwoCaptchaSolver{APIKey: "..."})
//	sol, err := captcha.Solve(captcha.Challenge{Type: captcha.TypeImage, ImageB64: "..."})
type TwoCaptchaSolver struct {
	APIKey    string        // 2captcha API key
	Endpoint  string        // override API base (default: https://2captcha.com)
	Timeout   time.Duration // total wait (default: 120s)
	UserAgent string        // optional UA for API calls
}

// Name returns the solver identifier.
func (s *TwoCaptchaSolver) Name() string { return "2captcha" }

// Solve submits the challenge to 2captcha and polls for the result.
func (s *TwoCaptchaSolver) Solve(ctx context.Context, challenge Challenge) (*Solution, error) {
	if s.APIKey == "" {
		return nil, errors.New("2captcha: API key required")
	}
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = "https://2captcha.com"
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	// Submit the challenge (in.php)
	submitURL := endpoint + "/in.php"
	form := url.Values{}
	form.Set("key", s.APIKey)
	form.Set("json", "1")

	switch challenge.Type {
	case TypeImage:
		form.Set("method", "base64")
		form.Set("body", challenge.ImageB64)
	case TypeReCaptcha:
		form.Set("method", "userrecaptcha")
		form.Set("googlekey", challenge.SiteKey)
		form.Set("pageurl", challenge.PageURL)
		if challenge.Action != "" {
			form.Set("action", challenge.Action)
		}
		if challenge.MinScore > 0 {
			form.Set("min_score", fmt.Sprintf("%v", challenge.MinScore))
		}
	case TypeHCaptcha:
		form.Set("method", "hcaptcha")
		form.Set("sitekey", challenge.SiteKey)
		form.Set("pageurl", challenge.PageURL)
	case TypeTurnstile:
		form.Set("method", "turnstile")
		form.Set("sitekey", challenge.SiteKey)
		form.Set("pageurl", challenge.PageURL)
	case TypeGeeTest:
		return nil, errors.New("2captcha: GeeTest requires gt/challenge params, use SolveGeeTest")
	default:
		return nil, fmt.Errorf("2captcha: unsupported challenge type: %s", challenge.Type)
	}

	captionID, err := s.submit(ctx, submitURL, form)
	if err != nil {
		return nil, fmt.Errorf("2captcha submit failed: %w", err)
	}

	// Poll for result (res.php)
	resultURL := endpoint + "/res.php"
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, errors.New("2captcha: timed out waiting for solution")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
		token, err := s.pollResult(ctx, resultURL, s.APIKey, captionID)
		if err != nil {
			if errors.Is(err, errCAPCHA_NOT_READY) {
				continue
			}
			return nil, fmt.Errorf("2captcha poll failed: %w", err)
		}
		return &Solution{Solved: true, Token: token}, nil
	}
}

var errCAPCHA_NOT_READY = errors.New("CAPCHA_NOT_READY")

func (s *TwoCaptchaSolver) submit(ctx context.Context, submitURL string, form url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", submitURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Status  int    `json:"status"`
		Request string `json:"request"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid JSON response: %s", string(body))
	}
	if result.Status != 1 {
		return "", fmt.Errorf("submit error: %s", result.Request)
	}
	return result.Request, nil
}

func (s *TwoCaptchaSolver) pollResult(ctx context.Context, resultURL, apiKey, id string) (string, error) {
	params := url.Values{
		"key":    {apiKey},
		"action": {"get"},
		"id":     {id},
		"json":   {"1"},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", resultURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Status  int    `json:"status"`
		Request string `json:"request"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid JSON response: %s", string(body))
	}
	if result.Status != 1 {
		if result.Request == "CAPCHA_NOT_READY" {
			return "", errCAPCHA_NOT_READY
		}
		return "", fmt.Errorf("poll error: %s", result.Request)
	}
	return result.Request, nil
}
