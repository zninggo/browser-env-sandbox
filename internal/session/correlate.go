// Correlation constraint: keep concurrently-created session fingerprints spread
// apart so a risk-control aggregator cannot club a batch of near-identical
// fingerprints into one bot identity ("团灭").
//
// The Correlator is a concurrency-safe registry of already-admitted
// fingerprints. Before admitting a new fingerprint it checks the distance
// (fpengine.Distance) against every admitted one and rejects the candidate when
// it is too close to any of them — the caller (or GenerateDistinct) then
// resamples with a fresh seed. Admit is check-then-insert under one lock, so
// two concurrent creators can never both slip in a fingerprint that is too
// close to the other.
//
// This lives in the session module but consumes the shared fpengine distance
// metric and the shared api.Fingerprint type, so it is compatible with the
// data-driven device profile (ZMTO-47): both produce fingerprints over the same
// dimensions, and the constraint keys on the same stable dimensions the profile
// will sample.
package session

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// ErrTooSimilar is returned by Admit / GenerateDistinct when a candidate
// fingerprint is too close (distance < threshold) to an already-admitted one.
var ErrTooSimilar = errors.New("fingerprint too similar to an existing session")

// CorrelatorOptions configures a Correlator.
type CorrelatorOptions struct {
	// MinDistance is the minimum acceptable distance to every already-admitted
	// fingerprint. Defaults to fpengine.DefaultMinDistance when <= 0.
	MinDistance float64
	// Weights overrides the distance metric weights. Zero value uses
	// fpengine.DefaultWeights.
	Weights fpengine.WeightSet
}

// Correlator is a concurrency-safe fingerprint deduplication guard.
type Correlator struct {
	mu       sync.RWMutex
	minDist  float64
	weights  fpengine.WeightSet
	admitted []*api.Fingerprint
}

// NewCorrelator builds a Correlator from opts (zero-value safe).
func NewCorrelator(opts CorrelatorOptions) *Correlator {
	minDist := opts.MinDistance
	if minDist <= 0 {
		minDist = fpengine.DefaultMinDistance
	}
	weights := opts.Weights
	valid := fpengine.ValidateWeights(weights)
	if !valid {
		weights = fpengine.DefaultWeights()
	}
	return &Correlator{minDist: minDist, weights: weights}
}

// MinDistance returns the configured threshold.
func (c *Correlator) MinDistance() float64 {
	return c.minDist
}

// Count returns the number of admitted fingerprints.
func (c *Correlator) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.admitted)
}

// Admitted returns a copy of the admitted fingerprints (for inspection/tests).
func (c *Correlator) Admitted() []*api.Fingerprint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*api.Fingerprint, len(c.admitted))
	copy(out, c.admitted)
	return out
}

// distance computes the weighted distance between two fingerprints under
// c.weights.
func (c *Correlator) distance(a, b *api.Fingerprint) float64 {
	return fpengine.DistanceWithWeights(a, b, c.weights)
}

// MinDistanceTo returns the smallest distance from fp to any admitted
// fingerprint, or 0 if none are admitted yet.
func (c *Correlator) MinDistanceTo(fp *api.Fingerprint) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if fp == nil {
		return 0
	}
	best := math.MaxFloat64
	for _, a := range c.admitted {
		if d := c.distance(fp, a); d < best {
			best = d
		}
	}
	if best == math.MaxFloat64 {
		return 0
	}
	return best
}

// Check reports whether fp satisfies the constraint against every admitted
// fingerprint without recording it.
func (c *Correlator) Check(fp *api.Fingerprint) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.passesLocked(fp)
}

// passesLocked reports whether fp is at least minDist away from every admitted
// fingerprint. Caller holds the lock.
func (c *Correlator) passesLocked(fp *api.Fingerprint) bool {
	if fp == nil {
		return false
	}
	for _, a := range c.admitted {
		if c.distance(fp, a) < c.minDist {
			return false
		}
	}
	return true
}

// Admit records fp as admitted if it satisfies the constraint against all
// existing fingerprints, atomically (check + insert under one lock). Returns
// ErrTooSimilar (or ErrNilFingerprint) without recording when it fails.
func (c *Correlator) Admit(fp *api.Fingerprint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fp == nil {
		return errors.New("session: cannot admit a nil fingerprint")
	}
	if !c.passesLocked(fp) {
		return ErrTooSimilar
	}
	c.admitted = append(c.admitted, fp)
	return nil
}

// Reset clears the admitted set (e.g. for a new batch or after sessions close).
func (c *Correlator) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.admitted = c.admitted[:0]
}

// fingerprintGenerator is the subset of fpengine.Engine the correlator needs.
type fingerprintGenerator interface {
	GenerateWithTimezone(seed uint64, browser, os, timezone string) (*api.Fingerprint, error)
}

// GenerateDistinct generates a fresh fingerprint and admits it, resampling up
// to maxAttempts times until it is far enough from everything already admitted.
// It is safe to call concurrently — Admit's check-then-insert is atomic, so a
// failed admission means another goroutine beat us to an identical (or too
// close) fingerprint and we resample.
//
// opts.Seed seeds the first attempt; a zero seed draws a fresh random seed per
// attempt. A non-zero seed is bumped per attempt so a deterministic caller
// still gets a *different* fingerprint on each retry rather than re-generating
// the same one. maxAttempts <= 0 is treated as 1.
func GenerateDistinct(g fingerprintGenerator, c *Correlator, opts api.SessionOptions, maxAttempts int) (*api.Fingerprint, error) {
	if g == nil {
		return nil, errors.New("session: nil fingerprint generator")
	}
	if c == nil {
		return g.GenerateWithTimezone(opts.Seed, opts.Browser, opts.OS, opts.Timezone)
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	base := opts.Seed
	for attempt := 0; attempt < maxAttempts; attempt++ {
		seed := base
		if base == 0 {
			seed = uint64(time.Now().UnixNano()) + uint64(attempt)
		} else {
			seed = base + uint64(attempt)
		}
		fp, err := g.GenerateWithTimezone(seed, opts.Browser, opts.OS, opts.Timezone)
		if err != nil {
			return nil, fmt.Errorf("generate fingerprint: %w", err)
		}
		if err := c.Admit(fp); err == nil {
			return fp, nil
		}
	}
	return nil, fmt.Errorf("%w after %d attempts", ErrTooSimilar, maxAttempts)
}
