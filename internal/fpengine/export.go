package fpengine

import (
	"encoding/json"
	"fmt"
	"os"
)

// ExportJSON serializes a fingerprint to JSON for saving/sharing.
func ExportJSON(fp interface{}) ([]byte, error) {
	return json.MarshalIndent(fp, "", "  ")
}

// ExportToFile writes a fingerprint to a JSON file.
func ExportToFile(fp interface{}, path string) error {
	data, err := ExportJSON(fp)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ImportJSON deserializes a fingerprint from JSON bytes.
func ImportJSON(data []byte) (*importFingerprint, error) {
	var fp importFingerprint
	if err := json.Unmarshal(data, &fp); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}
	return &fp, nil
}

// ImportFromFile loads a fingerprint from a JSON file.
func ImportFromFile(path string) (*importFingerprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	return ImportJSON(data)
}

// importFingerprint is a loose structure for deserialization.
// Uses the same fields as api.Fingerprint but with relaxed types
// to handle hand-edited JSON.
type importFingerprint struct {
	Seed      uint64          `json:"seed"`
	Browser   json.RawMessage `json:"browser"`
	OS        json.RawMessage `json:"os"`
	GPU       json.RawMessage `json:"gpu"`
	Navigator json.RawMessage `json:"navigator"`
	Screen    json.RawMessage `json:"screen"`
	Canvas    json.RawMessage `json:"canvas"`
	WebGL     json.RawMessage `json:"webgl"`
	Audio     json.RawMessage `json:"audio"`
	Fonts     []string        `json:"fonts"`
	Timezone  string          `json:"timezone"`
	Languages []string        `json:"languages"`
	Window    json.RawMessage `json:"window"`
}

// ListSavedFingerprints scans a directory for saved fingerprint files.
func ListSavedFingerprints(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && hasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
