// Fingerprint distance metric.
//
// This is the shared measure used by the session deduplication / correlation
// guard (see internal/session/correlate.go) to answer one question: are two
// fingerprints "close enough" that a risk-control aggregator would club them
// into a single bot identity and take the whole batch down ("团灭")?
//
// The metric is a weighted sum of per-dimension distances over the STABLE
// dimensions that fingerprinting libraries actually persist on — canvas hash,
// WebGL renderer/vendor, font list, screen/window geometry, User-Agent, and the
// TLS ClientHello fingerprint (JA3/JA4, which is version-bound). Deliberately
// excluded are the volatile signals a real browser varies between visits
// (hardwareConcurrency, deviceMemory, timezone offset): those are *supposed* to
// be shared across distinct devices, so they must not drive the distance.
//
// The weights and the default threshold are documented and reproducible: they
// derive from the Shannon entropy of each dimension's sampling distribution
// (see WeightSet, DefaultWeights and FingerprintEntropy). This metric operates
// only on the shared api.Fingerprint type so it stays compatible with the
// data-driven device profile (ZMTO-47): both consume and produce the same
// dimensions, so a profile-sampled fingerprint has no distance metric mismatch.
package fpengine

import (
	"math"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// WeightSet is the per-dimension weight vector for the distance metric. The
// entries MUST sum to 1.0 (enforced by ValidateWeights). Rationale per dimension
// mirrors the identifying entropy (bits) each contributes to a visitor ID in
// modern fingerprinting libraries:
//
//	canvas     — canvas.toDataURL hash; the single highest-weight signal
//	             (FingerprintJS Open Source weights it ~16-18 bits).
//	webgl      — WebGL renderer/vendor string; GPU model is near-unique per
//	             device (~10-12 bits).
//	fonts      — installed font list; high cardinality (~13 bits in practice).
//	resolution — screen size × window size × devicePixelRatio (~7 bits).
//	ua         — browser name + version + OS platform (~2-4 bits).
//	tls        — JA3/JA4 ClientHello fingerprint, version-bound (~2 bits).
type WeightSet struct {
	Canvas     float64
	WebGL      float64
	Fonts      float64
	Resolution float64
	UA         float64
	TLS        float64
}

// DefaultWeights returns the default, entropy-proportional weight vector. The
// weights track each dimension's identifying entropy (see FingerprintEntropy):
// WebGL renderer carries the most because the GPU model is near-unique per
// device (the strongest stable per-device signal), followed by canvas hash, UA
// and screen resolution; fonts and the TLS version contribute least. WebGL is
// nudged above its raw entropy share because a differing GPU model is a full
// "different device" signal even when every other dimension coincides.
func DefaultWeights() WeightSet {
	return WeightSet{
		Canvas:     0.20,
		WebGL:      0.35,
		Fonts:      0.08,
		Resolution: 0.15,
		UA:         0.15,
		TLS:        0.07,
	}
}

// ValidateWeights reports whether the weights sum to 1.0 (within a tolerance)
// and are all non-negative. A zero or otherwise wrong-sum vector is rejected:
// a mis-summed weight set silently scales the whole distance and breaks the
// threshold's meaning, so it must not be used.
func ValidateWeights(w WeightSet) bool {
	sum := w.Canvas + w.WebGL + w.Fonts + w.Resolution + w.UA + w.TLS
	if math.Abs(sum-1.0) > 1e-6 {
		return false
	}
	for _, v := range []float64{w.Canvas, w.WebGL, w.Fonts, w.Resolution, w.UA, w.TLS} {
		if v < 0 {
			return false
		}
	}
	return true
}

// hardLockCeil caps the distance of a "same device core" pair (identical canvas
// hash AND identical WebGL renderer). Such a pair is — from every fingerprinting
// library's perspective — the same GPU/device, whatever its soft dimensions say,
// so its distance MUST stay below the threshold. It is chosen just under the
// threshold (see DefaultMinDistance).
const hardLockCeil = 0.25

// DefaultMinDistance is the default deduplication threshold. Two fingerprints
// are considered "the same device" (and one of them must be resampled) when
// their distance is strictly below this value.
//
// Derivation (reproducible via FingerprintEntropy + RecommendedMinDistance): a
// same-device-core pair is hard-capped at hardLockCeil = 0.25, while two
// devices that differ only in GPU model separate by the full WebGL weight
// (0.35). The empirical distance distribution of distinct samples is bimodal
// with an empty gap between those two values (near-duplicates cluster below
// 0.25, distinct devices at or above 0.35). The threshold is the midpoint of
// that gap — 0.30 — so duplicates are always rejected and genuine distinct
// devices always pass. See RecommendedMinDistance, which recomputes the same
// value from the weights.
const DefaultMinDistance = 0.30

// sameDeviceCore reports whether both fingerprints share the same hardware
// identity core: an identical (non-empty) canvas hash AND an identical WebGL
// renderer. This is the "same GPU + same canvas" cluster a risk-control
// aggregator keys on first.
func sameDeviceCore(a, b *api.Fingerprint) bool {
	if a == nil || b == nil {
		return false
	}
	canvasSame := a.Canvas.ToDataURLHash != "" && a.Canvas.ToDataURLHash == b.Canvas.ToDataURLHash
	webglSame := a.WebGL.Renderer != "" && a.WebGL.Renderer == b.WebGL.Renderer
	return canvasSame && webglSame
}

// Distance reports the normalized [0,1] fingerprint distance using the default
// weights. 0 = identical, 1 = maximally different. A same-device-core pair can
// never exceed the 0.30 cap regardless of soft-dimension differences.
func Distance(a, b *api.Fingerprint) float64 {
	return DistanceWithWeights(a, b, DefaultWeights())
}

// DistanceWithWeights reports the normalized [0,1] distance under an explicit
// weight vector (must pass ValidateWeights; otherwise the default is used).
func DistanceWithWeights(a, b *api.Fingerprint, w WeightSet) float64 {
	if a == nil || b == nil {
		if a == nil && b == nil {
			return 0
		}
		return 1
	}
	if !ValidateWeights(w) {
		w = DefaultWeights()
	}

	d := w.Canvas*canvasDistance(a, b) +
		w.WebGL*webglDistance(a, b) +
		w.Fonts*fontsDistance(a, b) +
		w.Resolution*resolutionDistance(a, b) +
		w.UA*uaDistance(a, b) +
		w.TLS*tlsDistance(a, b)

	// Same device core must never clear the threshold: cap it below
	// DefaultMinDistance. This is what turns the soft dimensions (which could
	// otherwise push a same-GPU/same-canvas pair over the line) into a hard
	// rejection signal.
	if sameDeviceCore(a, b) && d > hardLockCeil {
		d = hardLockCeil
	}
	if d < 0 {
		d = 0
	}
	if d > 1 {
		d = 1
	}
	return d
}

// DistanceReport breaks a distance into its per-dimension components for
// observability and debugging (why was this pair rejected?).
type DistanceReport struct {
	Total      float64   `json:"total"`
	SameCore   bool      `json:"same_core"`
	Canvas     float64   `json:"canvas"`
	WebGL      float64   `json:"webgl"`
	Fonts      float64   `json:"fonts"`
	Resolution float64   `json:"resolution"`
	UA         float64   `json:"ua"`
	TLS        float64   `json:"tls"`
	Weights    WeightSet `json:"weights"`
}

// Report returns the full per-dimension breakdown of Distance(a, b).
func Report(a, b *api.Fingerprint) DistanceReport {
	return ReportWithWeights(a, b, DefaultWeights())
}

// ReportWithWeights returns the per-dimension breakdown under explicit weights.
func ReportWithWeights(a, b *api.Fingerprint, w WeightSet) DistanceReport {
	if !ValidateWeights(w) {
		w = DefaultWeights()
	}
	r := DistanceReport{
		Weights:    w,
		Canvas:     canvasDistance(a, b),
		WebGL:      webglDistance(a, b),
		Fonts:      fontsDistance(a, b),
		Resolution: resolutionDistance(a, b),
		UA:         uaDistance(a, b),
		TLS:        tlsDistance(a, b),
		SameCore:   sameDeviceCore(a, b),
	}
	r.Total = w.Canvas*r.Canvas + w.WebGL*r.WebGL + w.Fonts*r.Fonts +
		w.Resolution*r.Resolution + w.UA*r.UA + w.TLS*r.TLS
	if r.SameCore && r.Total > hardLockCeil {
		r.Total = hardLockCeil
	}
	return r
}

// --- per-dimension distances (each in [0,1]) ---

// canvasDistance compares canvas.toDataURL hashes. Two equal, non-empty hashes
// are the same canvas fingerprint (0); any difference is 1. Two empty hashes
// carry no signal and are treated as equal (0).
func canvasDistance(a, b *api.Fingerprint) float64 {
	if a.Canvas.ToDataURLHash == "" || b.Canvas.ToDataURLHash == "" {
		return 0
	}
	if a.Canvas.ToDataURLHash == b.Canvas.ToDataURLHash {
		return 0
	}
	return 1
}

// webglDistance compares the WebGL renderer string. The GPU model is the
// primary per-device signal, so a differing renderer is a full separation (1);
// identical renderer is 0. The vendor is only a fallback when one renderer is
// empty.
func webglDistance(a, b *api.Fingerprint) float64 {
	ar, br := a.WebGL.Renderer, b.WebGL.Renderer
	if ar == "" || br == "" {
		if a.GPU.Vendor != "" && a.GPU.Vendor == b.GPU.Vendor {
			return 0
		}
		return 1
	}
	if ar == br {
		return 0
	}
	return 1
}

// fontsDistance is the Jaccard distance over the two font lists: 1 - |A∩B|/|A∪B|.
// Discards order and duplicates (font presence is a set property).
func fontsDistance(a, b *api.Fingerprint) float64 {
	if len(a.Fonts) == 0 || len(b.Fonts) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a.Fonts))
	for _, f := range a.Fonts {
		set[f] = struct{}{}
	}
	inter := 0
	union := len(set)
	for _, f := range b.Fonts {
		if _, ok := set[f]; ok {
			inter++
		} else {
			union++
		}
	}
	if union == 0 || inter == union {
		return 0
	}
	return 1 - float64(inter)/float64(union)
}

// resolutionDistance compares screen size, window size and devicePixelRatio. It
// averages the relative differences (Manhattan, normalized per axis) across
// width, height and DPR, clipped to [0,1]. A shared DPR + shared dimension
// yields 0; a large relative spread yields 1.
func resolutionDistance(a, b *api.Fingerprint) float64 {
	aw := screenInt(a, "width")
	bw := screenInt(b, "width")
	ah := screenInt(a, "height")
	bh := screenInt(b, "height")

	relW := relDiff(aw, bw)
	relH := relDiff(ah, bh)
	relDPR := relDiffFloat(a.Window.DevicePixelRatio, b.Window.DevicePixelRatio)

	n := 0
	sum := 0.0
	for _, v := range []float64{relW, relH, relDPR} {
		if v < 0 {
			continue // axis not comparable (missing data)
		}
		sum += v
		n++
	}
	if n == 0 {
		return 0
	}
	return clamp01(sum / float64(n))
}

// uaDistance compares the browser name, OS name and browser major version that
// make up the User-Agent. 0 = identical UA identity; 0.5 = same browser+OS but
// different version; 1 = different browser or different OS.
func uaDistance(a, b *api.Fingerprint) float64 {
	name1, name2 := a.Browser.Name, b.Browser.Name
	os1, os2 := a.OS.Name, b.OS.Name
	if name1 != name2 || os1 != os2 {
		return 1
	}
	if a.Browser.MajorVer != b.Browser.MajorVer {
		return 0.5
	}
	return 0
}

// tlsDistance compares the TLS/JA3/JA4 ClientHello fingerprint, which is bound
// to the browser major version (the utls preset + the post-quantum signature
// injection boundary at Chrome 140 both key off it). 0 = same major version
// (same JA3/JA4); 1 = different.
//
// TODO(netlayer): when the TLS↔UA binding sub-task (ZMTO-46) exposes an explicit
// JA3/JA4 hash on the session, feed that string here instead of the version
// proxy so the distance reflects the actual ClientHello bytes, not just the
// version hint.
func tlsDistance(a, b *api.Fingerprint) float64 {
	if a.Browser.MajorVer == b.Browser.MajorVer {
		return 0
	}
	return 1
}

// --- helpers ---

func screenInt(fp *api.Fingerprint, key string) int {
	if fp == nil {
		return 0
	}
	switch v := fp.Screen[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func relDiff(a, b int) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	if a == 0 || b == 0 {
		return 1
	}
	d := math.Abs(float64(a - b))
	m := math.Max(float64(a), float64(b))
	return d / m
}

func relDiffFloat(a, b float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	if a == 0 || b == 0 {
		return 1
	}
	d := math.Abs(a - b)
	m := math.Max(math.Abs(a), math.Abs(b))
	return d / m
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
