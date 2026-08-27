package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/pkg/api"
)

// Profile represents a complete saved browser profile.
// This is the unit of persistence — a profile can be saved, shared,
// and loaded across machines. It captures everything needed to
// resume a session with the same identity.
type Profile struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Fingerprint *api.Fingerprint  `json:"fingerprint"`
	Cookies     map[string]string `json:"cookies"`
	Proxy       string            `json:"proxy,omitempty"`
	Location    string            `json:"location"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Tags        []string          `json:"tags,omitempty"`
}

// ProfileStore manages profile persistence on disk.
type ProfileStore struct {
	baseDir string
}

// NewProfileStore creates a profile store rooted at the given directory.
func NewProfileStore(baseDir string) *ProfileStore {
	if baseDir == "" {
		baseDir = "data/profiles"
	}
	os.MkdirAll(baseDir, 0755)
	return &ProfileStore{baseDir: baseDir}
}

// Save writes a profile to disk.
func (ps *ProfileStore) Save(p *Profile) error {
	p.UpdatedAt = time.Now()
	path := filepath.Join(ps.baseDir, p.ID+".json")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a profile from disk by ID.
func (ps *ProfileStore) Load(id string) (*Profile, error) {
	path := filepath.Join(ps.baseDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}
	return &p, nil
}

// Delete removes a profile from disk.
func (ps *ProfileStore) Delete(id string) error {
	path := filepath.Join(ps.baseDir, id+".json")
	return os.Remove(path)
}

// List returns all saved profile IDs and names.
func (ps *ProfileStore) List() ([]Profile, error) {
	entries, err := os.ReadDir(ps.baseDir)
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ps.baseDir, entry.Name()))
		if err != nil {
			continue
		}
		var p Profile
		if json.Unmarshal(data, &p) == nil {
			profiles = append(profiles, p)
		}
	}
	return profiles, nil
}

// CreateFromSession creates a Profile from an active session.
func (ps *ProfileStore) CreateFromSession(ms *ManagedSession, name string) *Profile {
	return &Profile{
		ID:          ms.ID,
		Name:        name,
		Fingerprint: ms.Fingerprint,
		Cookies:     ms.Cookies.GetAll(),
		Proxy:       "",
		Location:    "https://example.com/",
		CreatedAt:   ms.CreatedAt,
		UpdatedAt:   time.Now(),
	}
}

// ExportJSON exports a profile as JSON bytes (for sharing).
func (p *Profile) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ImportJSON imports a profile from JSON bytes.
func ImportProfileJSON(data []byte) (*Profile, error) {
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GenerateNewProfile creates a new profile with a fresh fingerprint.
func GenerateNewProfile(fpEng *fpengine.Engine, name, browser, os string) (*Profile, error) {
	fp, err := fpEng.Generate(0, browser, os)
	if err != nil {
		return nil, err
	}
	return &Profile{
		ID:          fmt.Sprintf("profile-%d", time.Now().UnixNano()),
		Name:        name,
		Fingerprint: fp,
		Cookies:     make(map[string]string),
		Location:    "https://example.com/",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}
