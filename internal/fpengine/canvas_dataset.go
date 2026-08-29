// Package fpengine — canvas dataset loader.
//
// Loads pre-collected canvas.toDataURL() output from real Chrome browsers,
// keyed by "chrome<ver>_<os>_<gpuVendor>" (e.g. "chrome151_windows_nvidia").
// When a dataset entry exists for the current fingerprint's key, the sandbox's
// toDataURL() returns the real pre-collected value instead of a synthetic PNG.
//
// Dataset file: data/canvas_dataset.json
// Format: { "chrome151_windows_nvidia": "data:image/png;base64,iVBOR...", ... }
//
// The file is .gitignored (rendered output may contain device-identifying
// data). When absent or empty, the engine falls back to synthetic generation.
package fpengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// CanvasDataset maps fingerprint keys to pre-collected toDataURL() values.
type CanvasDataset map[string]string

var (
	canvasDataset     CanvasDataset
	canvasDatasetOnce sync.Once
	canvasDatasetErr  error
	canvasDatasetPath = defaultCanvasDatasetPath()
)

func defaultCanvasDatasetPath() string {
	if p := os.Getenv("BES_CANVAS_DATA"); p != "" {
		return p
	}
	candidates := []string{
		"data/canvas_dataset.json",
		filepath.Join(execDir(), "data", "canvas_dataset.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "data/canvas_dataset.json"
}

// LoadCanvasDataset loads the canvas dataset JSON (lazy, once).
// Returns nil (no error) when the file is missing — callers fall back to
// synthetic generation. Thread-safe.
func LoadCanvasDataset() (CanvasDataset, error) {
	canvasDatasetOnce.Do(func() {
		raw, err := os.ReadFile(canvasDatasetPath)
		if err != nil {
			// File missing — not an error, just no dataset.
			canvasDataset = nil
			return
		}
		var ds CanvasDataset
		if err := json.Unmarshal(raw, &ds); err != nil {
			canvasDatasetErr = err
			canvasDataset = nil
			return
		}
		canvasDataset = ds
	})
	return canvasDataset, canvasDatasetErr
}

// LookupCanvasDataset returns the pre-collected toDataURL() value for the
// given key, or "" if not found / dataset unavailable.
func LookupCanvasDataset(key string) string {
	ds, _ := LoadCanvasDataset()
	if ds == nil {
		return ""
	}
	return ds[key]
}

// ReloadCanvasDataset forces a reload (used after `bes update-fp`).
func ReloadCanvasDataset() (CanvasDataset, error) {
	canvasDatasetOnce = sync.Once{}
	return LoadCanvasDataset()
}
