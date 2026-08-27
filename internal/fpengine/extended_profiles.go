package fpengine

import (
	"github.com/zninggo/bes/pkg/api"
)

// ExtendedKnowledgeBase returns a knowledge base with Firefox, Safari,
// and Android profiles in addition to Chrome desktop.
func ExtendedKnowledgeBase() *KnowledgeBase {
	kb := DefaultKnowledgeBase()

	// ── Firefox profiles ──
	kb.Browsers = append(kb.Browsers,
		BrowserEntry{
			Name:       "firefox",
			Version:    "140",
			MajorVer:   140,
			UATemplate: "Mozilla/5.0 (%s; rv:140.0) Gecko/20100101 Firefox/140.0",
		},
		BrowserEntry{
			Name:       "firefox",
			Version:    "135",
			MajorVer:   135,
			UATemplate: "Mozilla/5.0 (%s; rv:135.0) Gecko/20100101 Firefox/135.0",
		},
	)

	// ── Safari profiles (macOS only) ──
	kb.Browsers = append(kb.Browsers,
		BrowserEntry{
			Name:       "safari",
			Version:    "18",
			MajorVer:   18,
			UATemplate: "Mozilla/5.0 (%s) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
		},
	)

	// ── Android OS profile ──
	kb.OSes = append(kb.OSes, OSEntry{
		Name:      "android",
		Version:   "13",
		Platform:  "Linux armv8l",
		UASegment: "Linux; Android 13; Pixel 7",
		DefaultFonts: []string{
			"Roboto", "Noto Sans", "Noto Sans CJK", "Noto Serif",
			"Droid Sans", "Droid Sans Mono", "Cutive Mono", "Coming Soon",
			" Dancing Script", "Carrois Gothic", "Cousine", "Cutive Mono",
			"Inter", "Source Sans Pro", "Source Code Pro",
		},
	})

	// ── Android GPUs ──
	kb.GPUs["android"] = []GPUEntry{
		{Vendor: "Qualcomm", Renderer: "Adreno (TM) 730"},
		{Vendor: "Qualcomm", Renderer: "Adreno (TM) 740"},
		{Vendor: "ARM", Renderer: "Mali-G710"},
	}

	// ── Android screens ──
	kb.Screens["android"] = []ScreenEntry{
		{Width: 1080, Height: 2400, AvailWidth: 1080, AvailHeight: 2280, ColorDepth: 24},
		{Width: 1440, Height: 3200, AvailWidth: 1440, AvailHeight: 3080, ColorDepth: 24},
	}

	// ── Android window props ──
	kb.WindowProps["android"] = []api.WindowProps{
		{InnerWidth: 412, InnerHeight: 915, OuterWidth: 412, OuterHeight: 915, DevicePixelRatio: 2.625, ScreenX: 0, ScreenY: 0},
		{InnerWidth: 360, InnerHeight: 780, OuterWidth: 360, OuterHeight: 780, DevicePixelRatio: 3, ScreenX: 0, ScreenY: 0},
	}

	// ── Chrome versions 137-154 (between 136 and 155, for broader coverage) ──
	for _, v := range []struct{ ver string; major int }{
		{"137", 137}, {"138", 138}, {"139", 139}, {"140", 140},
		{"141", 141}, {"143", 143}, {"144", 144}, {"147", 147},
		{"149", 149}, {"151", 151}, {"153", 153}, {"154", 154},
		// Older versions (for compatibility with older TLS profiles)
		{"131", 131}, {"124", 124}, {"120", 120},
	} {
		kb.Browsers = append(kb.Browsers, BrowserEntry{
			Name:       "chrome",
			Version:    v.ver,
			MajorVer:   v.major,
			UATemplate: "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + v.ver + " Safari/537.36",
		})
	}

	return kb
}

// FirefoxNavigatorAdjustments adjusts navigator properties for Firefox.
// Firefox has different vendor, no window.chrome, no userAgentData.
func FirefoxNavigatorAdjustments(nav map[string]any, version string) {
	nav["vendor"] = ""
	nav["productSub"] = "20100101"
	nav["product"] = "Gecko"
	// Firefox has no userAgentData
	delete(nav, "userAgentData")
	// Firefox has no window.chrome
}

// SafariNavigatorAdjustments adjusts navigator properties for Safari.
func SafariNavigatorAdjustments(nav map[string]any) {
	nav["vendor"] = "Apple Computer, Inc."
	nav["productSub"] = "20030107"
	nav["product"] = "Gecko"
	// Safari has no userAgentData
	delete(nav, "userAgentData")
}

// AndroidNavigatorAdjustments adjusts navigator for Android.
func AndroidNavigatorAdjustments(nav map[string]any) {
	nav["maxTouchPoints"] = 5
	// Android UA includes mobile
}
