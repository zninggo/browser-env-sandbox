package fpengine

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"
)

// TestHardwareConcurrency_RangeBuildNavigator covers the buildNavigator path:
// many generated fingerprints must never report a hardwareConcurrency above
// 64. The sanitized dataset no longer contains 640/384/96/128, and buildNavigator
// clamps any out-of-range sample to a safe default (engine.go).
func TestHardwareConcurrency_RangeBuildNavigator(t *testing.T) {
	eng := New()
	osList := []string{"windows", "macos", "linux", "android"}
	const samples = 500
	for _, osName := range osList {
		for i := 1; i <= samples; i++ {
			fp, err := eng.Generate(uint64(i), "chrome", osName)
			if err != nil {
				t.Fatalf("Generate(seed=%d, os=%s): %v", i, osName, err)
			}
			hwConc, ok := fp.Navigator["hardwareConcurrency"]
			if !ok {
				t.Fatalf("os=%s seed=%d: hardwareConcurrency missing from navigator", osName, i)
			}
			v, ok := hwConc.(int)
			if !ok {
				t.Fatalf("os=%s seed=%d: hardwareConcurrency not int, got %T (%v)", osName, i, hwConc, hwConc)
			}
			if v < minHardwareConcurrency || v > maxHardwareConcurrency {
				t.Fatalf("os=%s seed=%d: hardwareConcurrency=%d out of range [%d,%d]",
					osName, i, v, minHardwareConcurrency, maxHardwareConcurrency)
			}
		}
	}
}

// TestHardwareConcurrency_RangeValidateConsistency covers the ValidateConsistency
// path: a fingerprint with an out-of-range value must be rejected, and every
// generated fingerprint must pass validation (no false rejection).
func TestHardwareConcurrency_RangeValidateConsistency(t *testing.T) {
	eng := New()
	osList := []string{"windows", "macos", "linux", "android"}
	for _, osName := range osList {
		for i := 1; i <= 200; i++ {
			fp, err := eng.Generate(uint64(i), "chrome", osName)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if err := ValidateConsistency(fp); err != nil {
				t.Fatalf("ValidateConsistency rejected generated fp (os=%s seed=%d): %v", osName, i, err)
			}
		}
	}

	// A hand-crafted fingerprint with hwConc=640 must be rejected.
	over, err := eng.Generate(1, "chrome", "windows")
	if err != nil {
		t.Fatalf("Generate baseline: %v", err)
	}
	over.Navigator["hardwareConcurrency"] = 640
	if err := ValidateConsistency(over); err == nil {
		t.Fatalf("ValidateConsistency accepted hwConc=640 (should reject)")
	}

	// hwConc=0 must also be rejected (below min).
	over.Navigator["hardwareConcurrency"] = 0
	if err := ValidateConsistency(over); err == nil {
		t.Fatalf("ValidateConsistency accepted hwConc=0 (should reject)")
	}
}

// TestSyntheticCanvasDataURL_PNGDecode verifies the synthetic canvas PNG is a
// valid, decodable PNG — the IDAT chunk must be a proper deflate stream. The
// old implementation wrote raw pseudo-random bytes as IDAT, which fails
// png.Decode. Several seed combinations are exercised.
func TestSyntheticCanvasDataURL_PNGDecode(t *testing.T) {
	cases := []struct {
		majorVer  int
		osName    string
		gpuVendor string
	}{
		{131, "windows", "Google Inc. (NVIDIA)"},
		{150, "macos", "Apple"},
		{124, "linux", "Mesa/X.org"},
		{120, "android", "Qualcomm"},
		{131, "windows", ""},
	}
	for _, tc := range cases {
		b64 := syntheticCanvasDataURL(tc.majorVer, tc.osName, tc.gpuVendor)
		if b64 == "" {
			t.Fatalf("syntheticCanvasDataURL returned empty for %+v", tc)
		}
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("base64 decode failed for %+v: %v", tc, err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("png.Decode failed for %+v (IDAT not deflate-compressed?): %v", tc, err)
		}
		if img == nil {
			t.Fatalf("png.Decode returned nil image for %+v", tc)
		}
		bounds := img.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			t.Fatalf("decoded PNG has non-positive dimensions for %+v: %v", tc, bounds)
		}
	}
}

// TestFilterGPUsByOS_AndroidExcludesD3D11MetalApple verifies that the android
// GPU filter never returns Direct3D11, Metal, or Apple entries — including via
// the fallback path. The test feeds a deliberately tainted GPU list and also
// checks the real runtime dataset.
func TestFilterGPUsByOS_AndroidExcludesD3D11MetalApple(t *testing.T) {
	tainted := []FpRealGPU{
		{Vendor: "Qualcomm", Renderer: "Adreno (TM) 730"},
		{Vendor: "ARM", Renderer: "Mali-G78 MP24"},
		{Vendor: "Google Inc. (Intel)", Renderer: "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)"},
		{Vendor: "Apple", Renderer: "Apple GPU"},
		{Vendor: "Google Inc. (Apple)", Renderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M1)"},
		{Vendor: "Imagination Technologies", Renderer: "PowerVR Rogue GE8320"},
	}
	got := filterGPUsByOS(tainted, "android")
	if len(got) == 0 {
		t.Fatalf("filterGPUsByOS(android) returned empty from tainted list")
	}
	for _, g := range got {
		r := strings.ToLower(g.Renderer)
		v := strings.ToLower(g.Vendor)
		if strings.Contains(r, "direct3d11") {
			t.Fatalf("android filter leaked Direct3D11 GPU: %+v", g)
		}
		if strings.Contains(r, "metal") {
			t.Fatalf("android filter leaked Metal GPU: %+v", g)
		}
		if strings.Contains(v, "apple") {
			t.Fatalf("android filter leaked Apple GPU: %+v", g)
		}
	}

	realGPUs := CurrentFpRealGPUs()
	if len(realGPUs) == 0 {
		t.Skip("CurrentFpRealGPUs empty, skipping real-data check")
	}
	realFiltered := filterGPUsByOS(realGPUs, "android")
	for _, g := range realFiltered {
		r := strings.ToLower(g.Renderer)
		v := strings.ToLower(g.Vendor)
		if strings.Contains(r, "direct3d11") || strings.Contains(r, "metal") || strings.Contains(v, "apple") {
			t.Fatalf("android filter leaked impossible GPU from real dataset: %+v", g)
		}
	}

	eng := New()
	for i := 1; i <= 300; i++ {
		fp, err := eng.Generate(uint64(i), "chrome", "android")
		if err != nil {
			t.Fatalf("Generate android seed=%d: %v", i, err)
		}
		r := strings.ToLower(fp.GPU.Renderer)
		v := strings.ToLower(fp.GPU.Vendor)
		if strings.Contains(r, "direct3d11") {
			t.Fatalf("android fingerprint sampled Direct3D11 GPU (seed=%d): %s", i, fp.GPU.Renderer)
		}
		if strings.Contains(r, "metal") {
			t.Fatalf("android fingerprint sampled Metal GPU (seed=%d): %s", i, fp.GPU.Renderer)
		}
		if strings.Contains(v, "apple") {
			t.Fatalf("android fingerprint sampled Apple GPU (seed=%d): %s", i, fp.GPU.Vendor)
		}
	}
}