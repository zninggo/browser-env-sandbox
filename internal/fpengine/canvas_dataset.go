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
	"sync/atomic"
)

// CanvasDataset maps fingerprint keys to pre-collected toDataURL() values.
type CanvasDataset map[string]string

// canvasState is an immutable snapshot of a loaded (or attempted) dataset.
// Once constructed it is never mutated, so it is safe to read concurrently
// without locks after publication via canvasDatasetState.
type canvasState struct {
	dataset CanvasDataset
	err     error
}

// canvasDatasetState holds the current snapshot. nil means "not loaded yet".
// All access goes through atomic loads/stores — no field of a live snapshot
// is ever mutated, which is what makes reload race-free under -race.
var canvasDatasetState atomic.Pointer[canvasState]

// canvasDatasetOnce coordinates the *first* lazy load (only one goroutine
// reads the file). Reload replaces the whole coordination unit atomically by
// swapping canvasDatasetLoader, so a concurrent Do never races a field
// overwrite — it races at most on reading the *pointer* to the current Once,
// which is itself atomic.
var canvasDatasetLoader atomic.Pointer[sync.Once]

// canvasDatasetPath is resolved once at package init.
var canvasDatasetPath = defaultCanvasDatasetPath()

func init() {
	canvasDatasetLoader.Store(&sync.Once{})
}

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

// loadCanvasDatasetFromFile reads & parses the dataset file. Missing file is
// not an error: it yields a nil dataset with nil error (callers fall back to
// synthetic generation).
func loadCanvasDatasetFromFile() canvasState {
	raw, err := os.ReadFile(canvasDatasetPath)
	if err != nil {
		// File missing — not an error, just no dataset.
		return canvasState{dataset: nil, err: nil}
	}
	var ds CanvasDataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		return canvasState{dataset: nil, err: err}
	}
	return canvasState{dataset: ds, err: nil}
}

// LoadCanvasDataset loads the canvas dataset JSON (lazy, once).
// Returns nil (no error) when the file is missing — callers fall back to
// synthetic generation. Thread-safe; safe to call concurrently with Reload.
func LoadCanvasDataset() (CanvasDataset, error) {
	st := loadSnapshot()
	if st == nil {
		return nil, nil
	}
	return st.dataset, st.err
}

// loadSnapshot returns the current snapshot, triggering a lazy load if none
// exists yet. It is the single consolidated lazy-load path.
//
// The trailing fallback re-reads the file directly when a concurrent Reload
// cleared the published snapshot in the window between Do's return and the
// final Load. Without it, a Load sharing a sync.Once with an in-flight first
// loader (which does not re-run the func) would observe nil even though the
// dataset file exists. The fallback returns a fresh immutable snapshot without
// publishing it, which is safe — all snapshots are read-only after creation.
func loadSnapshot() *canvasState {
	if st := canvasDatasetState.Load(); st != nil {
		return st
	}
	loader := canvasDatasetLoader.Load()
	loader.Do(func() {
		if canvasDatasetState.Load() != nil {
			return
		}
		s := loadCanvasDatasetFromFile()
		canvasDatasetState.Store(&s)
	})
	if st := canvasDatasetState.Load(); st != nil {
		return st
	}
	// A concurrent Reload cleared the snapshot after Do returned (the Once
	// was already used, so the func did not re-run to republish). Read the
	// file directly rather than return nil.
	s := loadCanvasDatasetFromFile()
	return &s
}

// LookupCanvasDataset returns the pre-collected toDataURL() value for the
// given key, or "" if not found / dataset unavailable.
func LookupCanvasDataset(key string) string {
	st := loadSnapshot()
	if st == nil || st.dataset == nil {
		return ""
	}
	return st.dataset[key]
}

// ReloadCanvasDataset forces a reload (used after `bes update-fp`).
//
// The swap is two non-atomic steps: (1) clear the published snapshot, then
// (2) install a fresh sync.Once so the new generation coordinates its own
// first loader. Between the two steps there is a brief window where the
// snapshot is nil. A concurrent Load sharing the *old* Once (already Do'd,
// so it will not re-run the loader func) would observe nil there even though
// the file exists. loadSnapshot closes that window with a trailing fallback
// that re-reads the file directly instead of returning nil, so callers never
// see a spurious nil. A concurrent Do on the old Once only reads the atomic
// Pointer — it never mutates a live snapshot — so the two-step swap is safe
// once the fallback is in place.
func ReloadCanvasDataset() (CanvasDataset, error) {
	canvasDatasetState.Store(nil)
	canvasDatasetLoader.Store(&sync.Once{})
	st := loadSnapshot()
	if st == nil {
		return nil, nil
	}
	return st.dataset, st.err
}
