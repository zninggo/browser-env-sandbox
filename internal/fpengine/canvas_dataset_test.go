package fpengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// withTempDataset writes ds to a temp file and repoints canvasDatasetPath at
// it, returning a restore func. It calls ReloadCanvasDataset first so the new
// path takes effect for subsequent loads.
func withTempDataset(t *testing.T, ds CanvasDataset) func() {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "canvas_dataset.json")
	if ds != nil {
		raw, err := json.Marshal(ds)
		if err != nil {
			t.Fatalf("marshal dataset: %v", err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatalf("write dataset: %v", err)
		}
	}
	origPath := canvasDatasetPath
	canvasDatasetPath = path
	// Drop any cached snapshot from a prior generation so the next load reads
	// the new path.
	ReloadCanvasDataset()
	return func() {
		canvasDatasetPath = origPath
		ReloadCanvasDataset()
	}
}

// TestReloadCanvasDataset_ConcurrentRace hammers Load + Reload concurrently.
// Under `go test -race` this must report no data race and no panic. It
// exercises the exact hazard the fix addresses: Reload must not overwrite a
// sync.Once while a concurrent Do is in flight.
func TestReloadCanvasDataset_ConcurrentRace(t *testing.T) {
	const goroutines = 32
	const iterations = 50

	ds := CanvasDataset{"chrome151_windows_nvidia": "data:image/png;base64,AAA"}
	restore := withTempDataset(t, ds)
	defer restore()

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines repeatedly Load (read path, the old race victim).
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				got, err := LoadCanvasDataset()
				if err != nil {
					t.Errorf("LoadCanvasDataset error: %v", err)
					return
				}
				// Dataset is non-nil in this fixture; a nil result would mean
				// a stale snapshot leaked across reloads.
				if got == nil {
					t.Errorf("LoadCanvasDataset returned nil dataset")
					return
				}
				_ = LookupCanvasDataset("chrome151_windows_nvidia")
			}
		}()
	}
	// The other half repeatedly Reload (the old race offender).
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if _, err := ReloadCanvasDataset(); err != nil {
					t.Errorf("ReloadCanvasDataset error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestReloadCanvasDataset_RefreshesAfterFileChange verifies the update-fp
// semantics: after the dataset file changes on disk, a Reload must make
// LoadCanvasDataset return the NEW content, not a stale cached snapshot.
// This is the second acceptance criterion — update-fp must actually refresh
// canvas data.
func TestReloadCanvasDataset_RefreshesAfterFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "canvas_dataset.json")

	origPath := canvasDatasetPath
	canvasDatasetPath = path
	defer func() {
		canvasDatasetPath = origPath
		ReloadCanvasDataset()
	}()

	// Phase 1: write v1, load it.
	v1 := CanvasDataset{"chrome151_windows_nvidia": "data:image/png;base64,V1"}
	writeDataset(t, path, v1)
	ReloadCanvasDataset() // ensure clean load of v1
	got, err := LoadCanvasDataset()
	if err != nil {
		t.Fatalf("load v1: %v", err)
	}
	if got["chrome151_windows_nvidia"] != "data:image/png;base64,V1" {
		t.Fatalf("v1 mismatch: got %q", got["chrome151_windows_nvidia"])
	}

	// Phase 2: simulate `bes update-fp` rewriting the file with v2.
	v2 := CanvasDataset{"chrome151_windows_nvidia": "data:image/png;base64,V2"}
	writeDataset(t, path, v2)

	// Without reload, the cached snapshot must still show v1 (proves the cache
	// exists and the refresh is not a no-op re-read).
	if cached := LookupCanvasDataset("chrome151_windows_nvidia"); cached != "data:image/png;base64,V1" {
		t.Fatalf("expected stale v1 before reload, got %q", cached)
	}

	// The actual fix path: update-fp now calls ReloadCanvasDataset.
	ReloadCanvasDataset()
	got, err = LoadCanvasDataset()
	if err != nil {
		t.Fatalf("load v2: %v", err)
	}
	if got["chrome151_windows_nvidia"] != "data:image/png;base64,V2" {
		t.Fatalf("after reload expected v2, got %q — canvas not refreshed", got["chrome151_windows_nvidia"])
	}
}

// TestUpdateFP_BothReloadsCalled documents the server-side fix contract: the
// update-fp flow must call BOTH ReloadFpRealData and ReloadCanvasDataset. Since
// the cmd packages are hard to drive end-to-end, this test pins the
// fpengine-level invariant that ReloadCanvasDataset is callable and returns the
// refreshed data — the building block the two cmd entry points now invoke.
func TestUpdateFP_BothReloadsCalled(t *testing.T) {
	ds := CanvasDataset{"chrome150_macos_apple": "data:image/png;base64,UPD"}
	restore := withTempDataset(t, ds)
	defer restore()

	// Mirror the cmd/bes + cmd/bes-server update-fp sequence.
	_, _ = ReloadFpRealData()
	got, err := ReloadCanvasDataset()
	if err != nil {
		t.Fatalf("ReloadCanvasDataset during update-fp: %v", err)
	}
	if got == nil || got["chrome150_macos_apple"] != "data:image/png;base64,UPD" {
		t.Fatalf("update-fp did not refresh canvas: got %v", got)
	}
}

func writeDataset(t *testing.T, path string, ds CanvasDataset) {
	t.Helper()
	raw, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write dataset: %v", err)
	}
}
