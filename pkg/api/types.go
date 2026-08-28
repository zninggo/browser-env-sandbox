// Package api defines public types shared across all modules.
package api

// Fingerprint represents a complete, self-consistent browser fingerprint.
type Fingerprint struct {
	Seed      uint64         `json:"seed"`
	Browser   BrowserProfile `json:"browser"`
	OS        OSProfile      `json:"os"`
	GPU       GPUProfile     `json:"gpu"`
	Navigator map[string]any `json:"navigator"`
	Screen    map[string]any `json:"screen"`
	Canvas    CanvasFP       `json:"canvas"`
	WebGL     WebGLFP        `json:"webgl"`
	Audio     AudioFP        `json:"audio"`
	Fonts     []string       `json:"fonts"`
	Timezone  string         `json:"timezone"`
	Languages []string       `json:"languages"`
	Window    WindowProps    `json:"window"`
}

// BrowserProfile describes a browser version profile.
type BrowserProfile struct {
	Name       string `json:"name"`       // "chrome", "firefox", "safari"
	Version    string `json:"version"`    // "131"
	MajorVer   int    `json:"major_ver"`  // 131
	UATemplate string `json:"ua_template"`
}

// OSProfile describes an operating system profile.
type OSProfile struct {
	Name        string   `json:"name"`         // "windows", "macos", "linux", "android"
	Version     string   `json:"version"`      // "11", "14", ""
	Platform    string   `json:"platform"`     // "Win32", "MacIntel", "Linux x86_64"
	UASegment   string   `json:"ua_segment"`   // "Windows NT 10.0; Win64; x64"
	DefaultFonts []string `json:"default_fonts"`
}

// GPUProfile describes a GPU and its WebGL fingerprint.
type GPUProfile struct {
	Vendor   string `json:"vendor"`   // "Google Inc. (NVIDIA)"
	Renderer string `json:"renderer"` // "ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 ...)"
}

// CanvasFP holds canvas fingerprint data.
type CanvasFP struct {
	ToDataURLHash string `json:"to_data_url_hash"`
	MeasureText   map[string]float64 `json:"measure_text"`
}

// WebGLFP holds WebGL fingerprint data.
type WebGLFP struct {
	Vendor     string   `json:"vendor"`
	Renderer   string   `json:"renderer"`
	Version    string   `json:"version"`
	Extensions []string `json:"extensions"`
	Params     map[int32]string `json:"params"`
}

// AudioFP holds AudioContext fingerprint data.
type AudioFP struct {
	Hash string `json:"hash"`
}

// WindowProps holds window dimension properties.
type WindowProps struct {
	InnerWidth      int     `json:"inner_width"`
	InnerHeight     int     `json:"inner_height"`
	OuterWidth      int     `json:"outer_width"`
	OuterHeight     int     `json:"outer_height"`
	DevicePixelRatio float64 `json:"device_pixel_ratio"`
	ScreenX         int     `json:"screen_x"`
	ScreenY         int     `json:"screen_y"`
}

// SessionOptions configures a sandbox session.
type SessionOptions struct {
	Seed       uint64  `json:"seed,omitempty"`       // 0 = random unique fingerprint
	Browser    string  `json:"browser,omitempty"`    // "chrome"
	OS         string  `json:"os,omitempty"`         // "windows"
	Timezone   string  `json:"timezone,omitempty"`   // e.g. "Asia/Tokyo"; empty = random from KB
	Location   string  `json:"location"`             // document.URL
	Cookies    map[string]string `json:"cookies,omitempty"`
	Proxy      string  `json:"proxy,omitempty"`
	NetMode    string  `json:"net_mode"`             // "replay" or "live"
	Recording  string  `json:"recording,omitempty"`  // path to recording file for replay

	// Session-level default request headers for netlayer (live mode).
	// Referer/Origin are forbidden header names in the fetch spec, so they
	// can only be set here (Go side) — sandbox JS cannot override them via
	// fetch/XHR options. Explicit values override the browser-default
	// derivation (full URL same-origin / origin cross-origin).
	Referer      string            `json:"referer,omitempty"`
	Origin       string            `json:"origin,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`      // empty = session fingerprint UA
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`   // appended after Chrome's canonical order
}
