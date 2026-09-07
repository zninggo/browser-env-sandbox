package session

import (
	"sync"
	"testing"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

func genFp(t *testing.T, eng *fpengine.Engine, seed uint64) *api.Fingerprint {
	t.Helper()
	fp, err := eng.Generate(seed, "chrome", "windows")
	if err != nil {
		t.Fatalf("Generate(seed=%d): %v", seed, err)
	}
	return fp
}

// TestCorrelator_AdmitDuplicateRejected verifies the same fingerprint cannot be
// admitted twice.
func TestCorrelator_AdmitDuplicateRejected(t *testing.T) {
	c := NewCorrelator(CorrelatorOptions{})
	eng := fpengine.New()
	fp := genFp(t, eng, 1)
	if err := c.Admit(fp); err != nil {
		t.Fatalf("first Admit failed: %v", err)
	}
	if err := c.Admit(fp); err != ErrTooSimilar {
		t.Fatalf("duplicate Admit = %v, want ErrTooSimilar", err)
	}
	if c.Count() != 1 {
		t.Fatalf("Count = %d, want 1", c.Count())
	}
}

// TestCorrelator_NilRejected verifies a nil fingerprint is rejected.
func TestCorrelator_NilRejected(t *testing.T) {
	c := NewCorrelator(CorrelatorOptions{})
	if err := c.Admit(nil); err == nil {
		t.Fatal("Admit(nil) must fail")
	}
}

// TestCorrelator_CheckDoesNotRecord verifies Check is a pure predicate.
func TestCorrelator_CheckDoesNotRecord(t *testing.T) {
	c := NewCorrelator(CorrelatorOptions{})
	eng := fpengine.New()
	fp := genFp(t, eng, 1)
	if !c.Check(fp) {
		t.Fatal("Check on empty correlator must pass")
	}
	if err := c.Admit(fp); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if c.Check(fp) {
		t.Fatal("Check on duplicate must fail")
	}
	if c.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (Check must not record)", c.Count())
	}
}

// TestCorrelator_MinDistanceTo verifies the reported distance to the nearest
// admitted fingerprint is 0 for an exact duplicate and positive for a distant one.
func TestCorrelator_MinDistanceTo(t *testing.T) {
	c := NewCorrelator(CorrelatorOptions{})
	eng := fpengine.New()
	fp := genFp(t, eng, 1)
	if err := c.Admit(fp); err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if d := c.MinDistanceTo(fp); d != 0 {
		t.Fatalf("MinDistanceTo(duplicate) = %v, want 0", d)
	}
	other := genFp(t, eng, 2)
	if d := c.MinDistanceTo(other); !(d >= 0 && d <= 1) {
		t.Fatalf("MinDistanceTo(other) = %v out of [0,1]", d)
	}
}

// TestCorrelator_RespectsThreshold builds a set via sequential GenerateDistinct
// and verifies every admitted pair is at least MinDistance apart.
func TestCorrelator_RespectsThreshold(t *testing.T) {
	eng := fpengine.New()
	c := NewCorrelator(CorrelatorOptions{})
	const n = 40
	for i := 0; i < n; i++ {
		if _, err := GenerateDistinct(eng, c, api.SessionOptions{Browser: "chrome", OS: "windows"}, 50); err != nil {
			t.Fatalf("GenerateDistinct #%d failed: %v", i, err)
		}
	}
	admitted := c.Admitted()
	if len(admitted) != n {
		t.Fatalf("admitted %d, want %d", len(admitted), n)
	}
	for i := 0; i < len(admitted); i++ {
		for j := i + 1; j < len(admitted); j++ {
			d := fpengine.Distance(admitted[i], admitted[j])
			if d < c.MinDistance() {
				t.Fatalf("admitted pair (%d,%d) distance %v < threshold %v", i, j, d, c.MinDistance())
			}
		}
	}
}

// TestGenerateDistinct_Concurrent is the acceptance test: many concurrent
// GenerateDistinct calls (the batch-concurrent-session scenario) produce a set
// whose pairwise distance never falls below the threshold.
func TestGenerateDistinct_Concurrent(t *testing.T) {
	eng := fpengine.New()
	c := NewCorrelator(CorrelatorOptions{})
	const workers = 16

	var wg sync.WaitGroup
	errs := make([]error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, errs[id] = GenerateDistinct(eng, c, api.SessionOptions{Browser: "chrome", OS: "windows"}, 50)
		}(w)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d GenerateDistinct failed: %v", i, err)
		}
	}
	admitted := c.Admitted()
	if len(admitted) != workers {
		t.Fatalf("admitted %d, want %d", len(admitted), workers)
	}
	min := 1.0
	for i := 0; i < len(admitted); i++ {
		for j := i + 1; j < len(admitted); j++ {
			d := fpengine.Distance(admitted[i], admitted[j])
			if d < min {
				min = d
			}
			if d < c.MinDistance() {
				t.Fatalf("concurrent pair (%d,%d) distance %v < threshold %v", i, j, d, c.MinDistance())
			}
		}
	}
	t.Logf("min pairwise distance across %d concurrent sessions: %.3f (threshold %.3f)", workers, min, c.MinDistance())
}

// TestGenerateDistinct_ExhaustsWhenPinned verifies that when the generator can
// only produce an already-admitted fingerprint, GenerateDistinct fails cleanly
// after maxAttempts instead of looping forever.
func TestGenerateDistinct_ExhaustsWhenPinned(t *testing.T) {
	eng := fpengine.New()
	c := NewCorrelator(CorrelatorOptions{})
	opts := api.SessionOptions{Seed: 1, Browser: "chrome", OS: "windows"}
	if _, err := GenerateDistinct(eng, c, opts, 1); err != nil {
		t.Fatalf("first GenerateDistinct failed: %v", err)
	}
	// seed 1 is now admitted; a single attempt (maxAttempts=1) re-generates
	// seed 1 and must fail against it.
	if _, err := GenerateDistinct(eng, c, opts, 1); err == nil {
		t.Fatal("second GenerateDistinct with maxAttempts=1 must fail against the admitted fingerprint")
	}
}

// TestGenerateDistinct_NilGuard verifies a nil generator is rejected cleanly.
func TestGenerateDistinct_NilGuard(t *testing.T) {
	if _, err := GenerateDistinct(nil, NewCorrelator(CorrelatorOptions{}), api.SessionOptions{}, 1); err == nil {
		t.Fatal("GenerateDistinct with nil generator must fail")
	}
}
