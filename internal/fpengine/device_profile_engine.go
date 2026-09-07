package fpengine

import (
	"fmt"

	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

// sampleDeviceProfile draws a coherent device profile for the given browser/os
// constraints, or ok=false when no joint data is available (or no UA matches
// the constraint), in which case the caller falls back to the legacy sampler.
func (e *Engine) sampleDeviceProfile(rng *seededRNG, browser, os string) (DeviceProfile, bool) {
	s, err := loadDeviceProfileStore()
	if err != nil || s == nil {
		return DeviceProfile{}, false
	}
	return s.sampleDeviceProfile(rng, browser, os)
}

// buildFromDeviceProfile assembles an api.Fingerprint from a sampled
// DeviceProfile. It reuses the same pure builders (buildNavigator, buildWebGL,
// canvas/audio hash lookup) as the legacy path, but feeds them the joint
// values so no dimension is re-sampled independently.
func (e *Engine) buildFromDeviceProfile(seed uint64, rng *seededRNG, dpp DeviceProfile, timezone string) (*api.Fingerprint, error) {
	bp := api.BrowserProfile{
		Name:       dpp.Browser,
		Version:    dpp.BrowserVersion,
		MajorVer:   dpp.BrowserMajor,
		UATemplate: "",
	}
	osp := buildDeviceOSProfile(e.kb, dpp)
	gpu := api.GPUProfile{Vendor: dpp.Vendor, Renderer: dpp.Renderer}
	screen := buildDeviceScreen(dpp.Screen)

	tz, langs := "", []string(nil)
	if timezone != "" {
		if n, l, ok := e.kb.SampleTimezoneNamed(timezone); ok {
			tz, langs = n, l
		}
	}
	if tz == "" {
		tz, langs = e.kb.SampleTimezone(rng)
	}

	ov := &jointOverrides{
		ua:       dpp.UserAgent,
		hwConc:   dpp.HardwareConcurrency,
		devMem:   dpp.DeviceMemory,
		maxTouch: dpp.MaxTouchPoints,
	}
	nav := buildNavigator(bp, osp, gpu, screen, rng, langs, ov)

	canvas := e.kb.LookupCanvasHash(bp.MajorVer, osp.Name, gpu.Vendor)
	webgl := buildWebGL(gpu)
	audio := e.kb.LookupAudioHash(bp.MajorVer, osp.Name)
	win := buildDeviceWindow(dpp.Screen)

	fp := &api.Fingerprint{
		Seed:      seed,
		Browser:   bp,
		OS:        osp,
		GPU:       gpu,
		Navigator: nav,
		Screen:    screen,
		Canvas:    canvas,
		WebGL:     webgl,
		Audio:     audio,
		Fonts:     osp.DefaultFonts,
		Timezone:  tz,
		Languages: langs,
		Window:    win,
	}

	if err := ValidateConsistency(fp); err != nil {
		return nil, fmt.Errorf("device fingerprint consistency check failed: %w", err)
	}
	return fp, nil
}

// buildDeviceScreen maps a DeviceScreen onto the screen map shape the sandbox
// injection expects (mirrors the legacy SampleScreen output).
func buildDeviceScreen(d DeviceScreen) map[string]any {
	w, h := d.Width, d.Height
	if w == 0 {
		w = 1280
	}
	if h == 0 {
		h = 720
	}
	aw, ah := d.AvailWidth, d.AvailHeight
	if aw == 0 {
		aw = w
	}
	if ah == 0 {
		ah = h
	}
	cd := d.ColorDepth
	if cd == 0 {
		cd = 24
	}
	pd := d.PixelDepth
	if pd == 0 {
		pd = cd
	}
	return map[string]any{
		"width":       w,
		"height":      h,
		"availWidth":  aw,
		"availHeight": ah,
		"colorDepth":  cd,
		"pixelDepth":  pd,
		"orientation": map[string]any{"type": "landscape-primary", "angle": 0},
		"availLeft":   0,
		"availTop":    0,
	}
}

// buildDeviceWindow derives window dimensions from the sampled screen plus its
// DPR (which, for Retina/Android, is a per-device value the legacy path could
// not capture). Invariants preserved: screen.height >= availHeight >=
// outerHeight >= innerHeight (the source outerHeight can exceed availHeight by
// a few px of window border, so it is clamped rather than trusted).
func buildDeviceWindow(d DeviceScreen) api.WindowProps {
	w, h := d.Width, d.Height
	if w == 0 {
		w = 1280
	}
	if h == 0 {
		h = 720
	}
	aw, ah := d.AvailWidth, d.AvailHeight
	if aw == 0 {
		aw = w
	}
	if ah == 0 {
		ah = h
	}
	ow, oh := d.OuterWidth, d.OuterHeight
	if ow == 0 || ow > aw {
		ow = aw
	}
	if oh == 0 || oh > ah {
		oh = ah
	}
	ih := oh - 100 // browser chrome (tabs + address bar)
	if ih < 200 {
		ih = oh * 9 / 10
	}
	dpr := d.DevicePixelRatio
	if dpr <= 0 {
		dpr = 1.0
	}
	return api.WindowProps{
		InnerWidth:       ow,
		InnerHeight:      ih,
		OuterWidth:       ow,
		OuterHeight:      oh,
		DevicePixelRatio: dpr,
		ScreenX:          0,
		ScreenY:          0,
	}
}