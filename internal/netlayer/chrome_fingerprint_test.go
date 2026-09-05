package netlayer

import (
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestChromeVersionFromUA(t *testing.T) {
	cases := []struct {
		ua     string
		want   int
		wantOK bool
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36", 150, true},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.71 Safari/537.36", 120, true},
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36", 131, true},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36", 96, true},
		// A browser that is not Chrome has no Chrome/ token → not ok.
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0", 0, false},
	}
	for _, c := range cases {
		got, ok := chromeVersionFromUA(c.ua)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("chromeVersionFromUA(%q) = (%d, %v), want (%d, %v)", c.ua, got, ok, c.want, c.wantOK)
		}
	}
}

// TestChromeProfileFor_Rollback pins the version→preset rollback rule against
// the ladder. Each Chrome release between the ladder's named releases must
// reuse the nearest lower preset — the same rollback JA3/JA4 databases encode.
func TestChromeProfileForRollback(t *testing.T) {
	cases := []struct {
		major     int
		wantHello utls.ClientHelloID
	}{
		{57, utls.HelloChrome_58}, // clamp up to oldest
		{58, utls.HelloChrome_58}, // 58–61
		{61, utls.HelloChrome_58},
		{62, utls.HelloChrome_62}, // 62–69
		{69, utls.HelloChrome_62},
		{70, utls.HelloChrome_70}, // 70–71
		{71, utls.HelloChrome_70},
		{72, utls.HelloChrome_72}, // 72–82
		{82, utls.HelloChrome_72},
		{83, utls.HelloChrome_83},   // 83–86
		{87, utls.HelloChrome_87},   // 87–95
		{96, utls.HelloChrome_96},   // 96–99
		{100, utls.HelloChrome_100}, // 100–101
		{102, utls.HelloChrome_102}, // 102–105
		{105, utls.HelloChrome_102},
		{106, utls.HelloChrome_106_Shuffle}, // 106–114
		{114, utls.HelloChrome_106_Shuffle},
		{115, utls.HelloChrome_115_PQ}, // 115–119
		{119, utls.HelloChrome_115_PQ},
		{120, utls.HelloChrome_120_PQ}, // 120–130
		{130, utls.HelloChrome_120_PQ},
		{131, utls.HelloChrome_131}, // 131–132
		{132, utls.HelloChrome_131},
		{133, utls.HelloChrome_133}, // 133+
		{150, utls.HelloChrome_133},
	}
	for _, c := range cases {
		got := chromeProfileFor(c.major).hello
		if got != c.wantHello {
			t.Errorf("chromeProfileFor(%d).hello = %v, want %v", c.major, got, c.wantHello)
		}
	}
}

// TestChromeProfileFor_PQSigAlgs verifies the post-quantum signature schemes
// join only at Chrome 140+, independent of the ClientHello preset.
func TestChromeProfileForPQSigAlgs(t *testing.T) {
	for major := 58; major <= 250; major++ {
		got := chromeProfileFor(major).pqSigAlgs
		want := major >= pqSigAlgMinChromeVersion
		if got != want {
			t.Errorf("chromeProfileFor(%d).pqSigAlgs = %v, want %v", major, got, want)
		}
	}
}

// TestChromeProfileForDeterministic and TestChromeFingerprintForUADeterministic
// assert there is no randomness: the same version (or UA) always resolves to an
// identical profile. This is the "不再随机生成" acceptance criterion.
func TestChromeProfileForDeterministic(t *testing.T) {
	a := chromeProfileFor(150)
	for i := 0; i < 100; i++ {
		b := chromeProfileFor(150)
		if a != b {
			t.Fatalf("chromeProfileFor(150) is non-deterministic: %+v vs %+v", a, b)
		}
	}
}

func TestChromeFingerprintForUADeterministic(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	a, ok := chromeFingerprintForUA(ua)
	if !ok {
		t.Fatalf("chromeFingerprintForUA(%q) not ok", ua)
	}
	for i := 0; i < 100; i++ {
		b, _ := chromeFingerprintForUA(ua)
		if a != b {
			t.Fatalf("chromeFingerprintForUA(%q) is non-deterministic", ua)
		}
	}
}

// TestChromeFingerprintForUA_Binding verifies the end-to-end invariant: a UA
// declaring Chrome/150 resolves to the same profile as version 150 directly.
func TestChromeFingerprintForUABinding(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	got, ok := chromeFingerprintForUA(ua)
	if !ok {
		t.Fatalf("chromeFingerprintForUA not ok")
	}
	want := chromeProfileFor(150)
	if got != want {
		t.Errorf("UA→fingerprint = %+v, want %+v (version binding broken)", got, want)
	}
	if !got.pqSigAlgs {
		t.Errorf("Chrome 150 UA should resolve to pqSigAlgs=true (140+)")
	}
}

// TestChromeFingerprintLadderMonotonic ensures the table is ordered by
// ascending minMajor — a misordered ladder silently corrupts the rollback.
func TestChromeFingerprintLadderMonotonic(t *testing.T) {
	for i := 1; i < len(chromeFingerprintLadder); i++ {
		if chromeFingerprintLadder[i].minMajor <= chromeFingerprintLadder[i-1].minMajor {
			t.Fatalf("ladder not monotonic at index %d: %d -> %d",
				i, chromeFingerprintLadder[i-1].minMajor, chromeFingerprintLadder[i].minMajor)
		}
	}
}
