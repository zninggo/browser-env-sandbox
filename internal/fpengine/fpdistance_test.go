package fpengine

import (
	"testing"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

func mustGen(t *testing.T, seed uint64, browser, os string) *api.Fingerprint {
	t.Helper()
	fp, err := New().Generate(seed, browser, os)
	if err != nil {
		t.Fatalf("Generate(seed=%d): %v", seed, err)
	}
	return fp
}

// TestValidateWeights covers the weight-vector invariant (sum == 1.0).
func TestValidateWeights(t *testing.T) {
	if !ValidateWeights(DefaultWeights()) {
		t.Fatal("DefaultWeights must be valid (sum 1.0)")
	}
	// A weight vector with the remaining dimensions zeroed still sums to 1.0.
	if !ValidateWeights(WeightSet{Canvas: 0.5, WebGL: 0.5}) {
		t.Fatal("half/half weights must be valid (sum 1.0)")
	}
	if ValidateWeights(WeightSet{Canvas: 0.6, WebGL: 0.6}) {
		t.Fatal("weights summing to 1.2 must be rejected")
	}
	if ValidateWeights(WeightSet{Canvas: 0.5}) {
		t.Fatal("weights summing to 0.5 must be rejected")
	}
	if ValidateWeights(WeightSet{}) {
		t.Fatal("zero weights must be rejected")
	}
	if ValidateWeights(WeightSet{Canvas: -0.1, WebGL: 1.1}) {
		t.Fatal("negative weights must be rejected")
	}
}

// TestDistance_SelfIsZero verifies a fingerprint is distance 0 from itself.
func TestDistance_SelfIsZero(t *testing.T) {
	fp := mustGen(t, 42, "chrome", "windows")
	if d := Distance(fp, fp); d != 0 {
		t.Fatalf("Distance(fp, fp) = %v, want 0", d)
	}
	if d := DistanceWithWeights(fp, fp, DefaultWeights()); d != 0 {
		t.Fatalf("DistanceWithWeights(fp, fp) = %v, want 0", d)
	}
}

// TestDistance_IdenticalSeedsAreZero verifies two independently generated
// fingerprints from the same seed are distance 0 (deterministic generation).
func TestDistance_IdenticalSeedsAreZero(t *testing.T) {
	a := mustGen(t, 7, "chrome", "windows")
	b := mustGen(t, 7, "chrome", "windows")
	if d := Distance(a, b); d != 0 {
		t.Fatalf("same-seed fingerprints distance = %v, want 0", d)
	}
}

// TestDistance_Symmetric verifies the metric is symmetric across many pairs.
func TestDistance_Symmetric(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		a := mustGen(t, seed, "chrome", "windows")
		b := mustGen(t, seed+1000, "chrome", "windows")
		dab := Distance(a, b)
		dba := Distance(b, a)
		if dab != dba {
			t.Fatalf("asymmetric distance: D(a,b)=%v D(b,a)=%v (seeds %d,%d)", dab, dba, seed, seed+1000)
		}
		if dab < 0 || dab > 1 {
			t.Fatalf("distance out of range: %v", dab)
		}
	}
}

// TestDistance_NilHandling verifies nil fingerprints are handled sans panic.
func TestDistance_NilHandling(t *testing.T) {
	fp := mustGen(t, 1, "chrome", "windows")
	if d := Distance(fp, nil); d != 1 {
		t.Fatalf("Distance(fp, nil) = %v, want 1", d)
	}
	if d := Distance(nil, fp); d != 1 {
		t.Fatalf("Distance(nil, fp) = %v, want 1", d)
	}
	if d := Distance(nil, nil); d != 0 {
		t.Fatalf("Distance(nil, nil) = %v, want 0", d)
	}
}

// TestDistance_SameCoreCapped verifies that two fingerprints sharing the same
// canvas hash AND WebGL renderer (the "same device core") never exceed the
// hardLockCeil, however different their soft dimensions are.
func TestDistance_SameCoreCapped(t *testing.T) {
	a := mustGen(t, 1, "chrome", "windows")
	b := mustGen(t, 2, "chrome", "windows")

	// Force the same device core onto b.
	b.Canvas.ToDataURLHash = a.Canvas.ToDataURLHash
	b.Canvas.ToDataURL = a.Canvas.ToDataURL
	b.WebGL.Renderer = a.WebGL.Renderer
	b.GPU.Renderer = a.GPU.Renderer

	// Make b's soft dimensions maximally different.
	b.Fonts = []string{"A", "B", "C"}
	a.Fonts = []string{"X", "Y", "Z"}
	b.Screen["width"] = 800
	b.Screen["height"] = 600

	d := Distance(a, b)
	if d > hardLockCeil {
		t.Fatalf("same-core pair distance = %v, must be capped at %v", d, hardLockCeil)
	}
}

// TestDistance_DistinctIsPositive verifies genuinely distinct fingerprints have
// a non-trivial (positive) distance.
func TestDistance_DistinctIsPositive(t *testing.T) {
	a := mustGen(t, 1, "chrome", "linux")
	b := mustGen(t, 2, "chrome", "windows")
	if d := Distance(a, b); d <= 0 {
		t.Fatalf("distinct fingerprints distance = %v, want > 0", d)
	}
}

// TestReport_MatchesDistance verifies the per-dimension report totals to the
// same value as Distance.
func TestReport_MatchesDistance(t *testing.T) {
	a := mustGen(t, 1, "chrome", "windows")
	b := mustGen(t, 2, "chrome", "windows")
	r := Report(a, b)
	want := Distance(a, b)
	if r.Total != want {
		t.Fatalf("Report.Total = %v, Distance = %v", r.Total, want)
	}
	if r.Weights != DefaultWeights() {
		t.Fatalf("Report.Weights mismatch default")
	}
}

// TestFontsDistance_Jaccard verifies the Jaccard distance is computed correctly
// on hand-picked sets.
func TestFontsDistance_Jaccard(t *testing.T) {
	base := mustGen(t, 1, "chrome", "windows")
	a := *base
	b := *base
	a.Fonts = []string{"A", "B", "C", "D"}
	b.Fonts = []string{"A", "B", "E", "F"}
	// |A∩B|=2 ("A","B"), |A∪B|=6 ("A","B","C","D","E","F") → Jaccard distance = 1-2/6 = 2/3.
	got := fontsDistance(&a, &b)
	want := 1 - 2.0/6.0
	if got < want-1e-9 || got > want+1e-9 {
		t.Fatalf("fontsDistance = %v, want %v", got, want)
	}
}
