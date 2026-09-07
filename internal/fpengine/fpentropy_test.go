package fpengine

import "testing"

// TestFingerprintEntropy_Positive verifies the entropy basis is computable and
// positive on the embedded data (the numbers a reviewer recomputes to substantiate
// the threshold).
func TestFingerprintEntropy_Positive(t *testing.T) {
	e := FingerprintEntropy()
	for name, bits := range map[string]float64{
		"canvas":     e.Canvas,
		"webgl":      e.WebGL,
		"fonts":      e.Fonts,
		"resolution": e.Resolution,
		"ua":         e.UA,
		"tls":        e.TLS,
	} {
		if bits < 0 {
			t.Fatalf("%s entropy negative: %v", name, bits)
		}
	}
	if e.Total <= 0 {
		t.Fatalf("total identifying entropy is non-positive: %v", e.Total)
	}
	// The GPU renderer list is near-unique per device, so WebGL entropy should
	// be meaningfully above 1 bit.
	if e.WebGL < 1.0 {
		t.Fatalf("webgl entropy unexpectedly low: %v bits", e.WebGL)
	}
}

// TestFingerprintEntropy_Reproducible verifies the entropy computation is a
// pure function of the loaded data (deterministic across calls).
func TestFingerprintEntropy_Reproducible(t *testing.T) {
	a := FingerprintEntropy()
	b := FingerprintEntropy()
	if a != b {
		t.Fatalf("entropy not deterministic: %+v vs %+v", a, b)
	}
}

// TestRecommendedMinDistance_InGap verifies the derived threshold sits strictly
// between the same-device-core ceiling and the minimum distinct-device separation,
// and matches the DefaultMinDistance constant.
func TestRecommendedMinDistance_InGap(t *testing.T) {
	tau := RecommendedMinDistance()
	if !(tau > hardLockCeil) {
		t.Fatalf("RecommendedMinDistance = %v, must exceed the hard cap %v", tau, hardLockCeil)
	}
	if !(tau < DefaultWeights().WebGL) {
		t.Fatalf("RecommendedMinDistance = %v, must be below the WebGL weight %v", tau, DefaultWeights().WebGL)
	}
	if tau != DefaultMinDistance {
		t.Fatalf("RecommendedMinDistance = %v does not match DefaultMinDistance = %v", tau, DefaultMinDistance)
	}
	// The expected distinct-device distance (the mean pairwise separation of two
	// unrelated samples) must clear the threshold by a wide margin.
	expected := ExpectedPairwiseDistance()
	if expected <= tau {
		t.Fatalf("ExpectedPairwiseDistance = %v is not above the threshold %v (threshold would reject genuine devices)", expected, tau)
	}
}
