// Reproducible threshold basis: Shannon entropy and expected pairwise distance.
//
// The deduplication threshold must not be a hand-waved constant — it has to be
// recomputable from the same distributions the fingerprint engine samples from.
// Two numbers are exposed here and both are pure functions of the loaded data
// (KnowledgeBase + apify fingerprint suite):
//
//	FingerprintEntropy()       — Shannon entropy (bits) of each dimension's
//	                            sampling distribution. H_d = -Σ p_i log2(p_i).
//	ExpectedPairwiseDistance() — the weighted expected distance between two
//	                            independent samples, built from each dimension's
//	                            Simpson diversity index 1 - Σ p_i² (the probability
//	                            two independent draws pick different categories).
//
// The default threshold (DefaultMinDistance) is the midpoint between the
// same-device-core ceiling and the minimum distinct-device separation. See
// RecommendedMinDistance for the full derivation.
package fpengine

import (
	"math"
	"sort"
	"strconv"
)

// EntropyStats is the Shannon entropy (in bits) of each fingerprint dimension's
// sampling distribution. Total is the sum (an upper bound on the identifying
// information a single fingerprint carries, assuming dimension independence).
type EntropyStats struct {
	Canvas     float64 `json:"canvas_bits"`
	WebGL      float64 `json:"webgl_bits"`
	Fonts      float64 `json:"fonts_bits"`
	Resolution float64 `json:"resolution_bits"`
	UA         float64 `json:"ua_bits"`
	TLS        float64 `json:"tls_bits"`
	Total      float64 `json:"total_bits"`
}

// FingerprintEntropy computes the Shannon entropy of each dimension from the
// distributions the engine actually samples: WebGL renderer and screen geometry
// from the apify data (CurrentFpRealGPUs / CurrentFpRealScreens), fonts from the
// per-OS default font sets, UA from the browser×OS matrix, TLS from the browser
// versions, and canvas from the GPU vendor-bucket distribution (the component
// the synthetic canvas hash is keyed on).
func FingerprintEntropy() EntropyStats {
	kb := DefaultKnowledgeBase()

	webgl := entropyRenderers(CurrentFpRealGPUs())
	resolution := entropyScreens(CurrentFpRealScreens())
	fonts := entropyFontSets(kb)
	ua := entropyUA(kb)
	tls := entropyBrowserVersions(kb)
	canvas := entropyCanvasBuckets(CurrentFpRealGPUs(), kb)

	return EntropyStats{
		Canvas:     canvas,
		WebGL:      webgl,
		Fonts:      fonts,
		Resolution: resolution,
		UA:         ua,
		TLS:        tls,
		Total:      canvas + webgl + fonts + resolution + ua + tls,
	}
}

// ExpectedPairwiseDistance returns the weighted expected distance between two
// independent samples, computed as Σ_d w_d · (1 − Σ_i p_di²) over each
// dimension's category frequencies (Simpson diversity index). This is the
// "center of mass" of the distinct-device distance distribution: two unrelated
// devices are, on average, this far apart, while a same-device duplicate is 0
// apart (and is additionally hard-capped at hardLockCeil).
//
// Continuous dimensions (resolution) are reduced to their discrete screen
// buckets so they participate in the Simpson computation on equal footing.
func ExpectedPairwiseDistance() float64 {
	w := DefaultWeights()
	kb := DefaultKnowledgeBase()
	gpus := CurrentFpRealGPUs()
	screens := CurrentFpRealScreens()

	webgl := 1 - simpson(rendererCounts(gpus))
	resolution := 1 - simpson(screenCounts(screens))
	fonts := 1 - simpson(fontSetCounts(kb))
	ua := 1 - simpson(uaCounts(kb))
	tls := 1 - simpson(browserVersionCounts(kb))
	canvas := 1 - simpson(canvasBucketCounts(gpus, kb))

	return w.Canvas*canvas + w.WebGL*webgl + w.Fonts*fonts +
		w.Resolution*resolution + w.UA*ua + w.TLS*tls
}

// RecommendedMinDistance derives the deduplication threshold from the entropy
// basis. It is the midpoint of the distance gap between:
//
//  1. the same-device-core ceiling (hardLockCeil = 0.25) — the maximum distance
//     a "same GPU + same canvas" pair can reach, and
//  2. the minimum distinct-device separation (the WebGL weight) — how far apart
//     two devices that differ only in GPU model are.
//
// The empirical distance distribution of distinct samples is bimodal with an
// empty gap in between (near-duplicates cluster ≤ 0.25, distinct devices ≥ WebGL
// weight), so the midpoint cleanly separates the two modes. It recomputes to
// DefaultMinDistance (0.30); as the data-driven profile raises the entropy of
// the soft dimensions the gap widens and the midpoint shifts up with it.
func RecommendedMinDistance() float64 {
	return (hardLockCeil + DefaultWeights().WebGL) / 2
}

// --- entropy / diversity helpers ---

func shannonBits(counts map[string]int) float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	// Sort keys so the float sum is deterministic (map iteration order is random
	// and floating-point addition is not associative).
	keys := sortedKeys(counts)
	var h float64
	for _, k := range keys {
		c := counts[k]
		if c == 0 {
			continue
		}
		p := float64(c) / float64(total)
		h -= p * math.Log2(p)
	}
	return h
}

// simpson returns Σ p_i² (the probability two independent draws coincide).
func simpson(counts map[string]int) float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	keys := sortedKeys(counts)
	var s float64
	for _, k := range keys {
		p := float64(counts[k]) / float64(total)
		s += p * p
	}
	return s
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// rendererCounts buckets GPUs by exact renderer string (near-unique per device).
func rendererCounts(gpus []FpRealGPU) map[string]int {
	m := make(map[string]int, len(gpus))
	for _, g := range gpus {
		if g.Renderer != "" {
			m[g.Renderer]++
		}
	}
	return m
}

func entropyRenderers(gpus []FpRealGPU) float64 {
	return shannonBits(rendererCounts(gpus))
}

// screenCounts buckets screens by their (width, height) geometry.
func screenCounts(screens []FpRealScreen) map[string]int {
	m := make(map[string]int, len(screens))
	for _, s := range screens {
		m[screenKey(s.Width, s.Height)]++
	}
	return m
}

func entropyScreens(screens []FpRealScreen) float64 {
	return shannonBits(screenCounts(screens))
}

func screenKey(w, h int) string {
	return strconv.Itoa(w) + "x" + strconv.Itoa(h)
}

// fontSetCounts buckets by each OS's default font set (fonts are per-OS
// constant today; the data-driven profile will widen this distribution).
func fontSetCounts(kb *KnowledgeBase) map[string]int {
	m := make(map[string]int, len(kb.OSes))
	for _, o := range kb.OSes {
		m[fontSetKey(o.DefaultFonts)]++
	}
	return m
}

func entropyFontSets(kb *KnowledgeBase) float64 {
	return shannonBits(fontSetCounts(kb))
}

// uaCounts buckets the browser-name × OS-name matrix that composes the UA.
func uaCounts(kb *KnowledgeBase) map[string]int {
	m := make(map[string]int, len(kb.Browsers)*len(kb.OSes))
	for _, b := range kb.Browsers {
		for _, o := range kb.OSes {
			m[b.Name+"/"+b.Version+"::"+o.Name]++
		}
	}
	return m
}

func entropyUA(kb *KnowledgeBase) float64 {
	return shannonBits(uaCounts(kb))
}

// browserVersionCounts buckets the browser major versions (what the TLS/JA3/JA4
// preset is bound to).
func browserVersionCounts(kb *KnowledgeBase) map[string]int {
	m := make(map[string]int, len(kb.Browsers))
	for _, b := range kb.Browsers {
		m[b.Version]++
	}
	return m
}

func entropyBrowserVersions(kb *KnowledgeBase) float64 {
	return shannonBits(browserVersionCounts(kb))
}

// canvasBucketCounts buckets GPUs by their canvas category: the vendor bucket
// the synthetic canvas hash is keyed on (nvidia/intel/amd/apple), expanded by
// OS count and browser-major count to reflect the full hash key.
func canvasBucketCounts(gpus []FpRealGPU, kb *KnowledgeBase) map[string]int {
	m := make(map[string]int, 16)
	for _, g := range gpus {
		v := canvasVendorBucket(g.Vendor)
		for _, o := range kb.OSes {
			for _, b := range kb.Browsers {
				m[v+"::"+o.Name+"::"+b.Version]++
			}
		}
	}
	return m
}

func entropyCanvasBuckets(gpus []FpRealGPU, kb *KnowledgeBase) float64 {
	return shannonBits(canvasBucketCounts(gpus, kb))
}

// canvasVendorBucket mirrors canvasHashKey's coarse GPU classification which
// the synthetic canvas hash is keyed on.
func canvasVendorBucket(vendor string) string {
	switch {
	case contains(vendor, "Intel"):
		return "intel"
	case contains(vendor, "AMD"):
		return "amd"
	case contains(vendor, "Apple"):
		return "apple"
	default:
		return "nvidia"
	}
}

func fontSetKey(fonts []string) string {
	// FNV-1a over the sorted-by-construction font list, separator-guarded so
	// {"A","BC"} and {"AB","C"} cannot collide. Equality is the only contract.
	const offset = uint64(1469598103934665603)
	h := offset
	for _, f := range fonts {
		for i := 0; i < len(f); i++ {
			h ^= uint64(f[i])
			h *= 1099511628211
		}
		h ^= 0x1f // font separator
	}
	return strconv.FormatUint(h, 10)
}
