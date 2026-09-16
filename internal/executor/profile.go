package executor

import (
	"fmt"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// ProfileSnapshot is the single device profile that drives every executor.
//
// It wraps the fpengine output (api.Fingerprint — the complete, self-consistent
// joint realization: UA ↔ platform ↔ GPU ↔ screen ↔ fonts ↔ cores ↔ memory ↔
// timezone) together with the generation parameters that produced it, so any
// executor can re-derive or re-apply the same identity. Generating a snapshot
// once and handing it to both the V8 and the Chrome executor is the whole
// point of the dual-executor architecture: identity points come from the
// profile, only the execution engine differs.
type ProfileSnapshot struct {
	Fingerprint *api.Fingerprint
	Seed        uint64
	Browser     string
	OS          string
	Timezone    string
}

// NewSnapshot wraps an existing fingerprint (plus its generation parameters)
// as a ProfileSnapshot. The fingerprint is used as-is; nil is rejected.
func NewSnapshot(fp *api.Fingerprint, seed uint64, browser, osName, timezone string) (*ProfileSnapshot, error) {
	if fp == nil {
		return nil, fmt.Errorf("executor: fingerprint required")
	}
	return &ProfileSnapshot{
		Fingerprint: fp,
		Seed:        seed,
		Browser:     browser,
		OS:          osName,
		Timezone:    timezone,
	}, nil
}

// TakeSnapshot generates a fresh device profile through the provider and
// wraps it as a ProfileSnapshot. seed 0 = random (provider semantics).
func TakeSnapshot(provider ProfileProvider, seed uint64, browser, osName, timezone string) (*ProfileSnapshot, error) {
	if provider == nil {
		return nil, fmt.Errorf("executor: profile provider required")
	}
	fp, err := provider.GenerateWithTimezone(seed, browser, osName, timezone)
	if err != nil {
		return nil, fmt.Errorf("executor: profile generation failed: %w", err)
	}
	snap, err := NewSnapshot(fp, fp.Seed, browser, osName, timezone)
	if err != nil {
		return nil, err
	}
	// A random seed was drawn inside the provider; record the realized seed
	// so the snapshot is reproducible.
	snap.Seed = fp.Seed
	return snap, nil
}

// UserAgent returns the profile's User-Agent string.
func (s *ProfileSnapshot) UserAgent() string {
	if s.Fingerprint == nil {
		return ""
	}
	ua, _ := s.Fingerprint.Navigator["userAgent"].(string)
	return ua
}

// Describe returns a one-line human-readable summary of the profile.
func (s *ProfileSnapshot) Describe() string {
	if s.Fingerprint == nil {
		return "profile(<nil>)"
	}
	fp := s.Fingerprint
	return fmt.Sprintf("%s/%s @ %s (platform %s, %dx%d, seed %d)",
		fp.Browser.Name, fp.Browser.Version, fp.OS.Name, fp.OS.Platform,
		fp.Window.OuterWidth, fp.Window.OuterHeight, fp.Seed)
}
