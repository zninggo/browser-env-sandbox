// Package fpupdate — automatic fingerprint data updater.
//
// Downloads the latest apify/fingerprint-suite network.json from npm,
// parses it into bes's JSON format, and writes to data/fp_real_data.json.
// Also checks the npm registry for version changes to enable auto-updates.
package fpupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	npmRegistryURL = "https://registry.npmjs.org/fingerprint-generator/latest"
	networkZipPath = "package/data_files/fingerprint-network-definition.zip"
)

// NpmPackageInfo is the subset of npm registry response we need.
type NpmPackageInfo struct {
	Version string `json:"version"`
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

// CheckLatestVersion queries npm registry for the latest fingerprint-generator
// version. Returns the version string and tarball URL.
func CheckLatestVersion() (version, tarballURL string, err error) {
	resp, err := http.Get(npmRegistryURL)
	if err != nil {
		return "", "", fmt.Errorf("npm registry query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("npm registry returned %d", resp.StatusCode)
	}
	var info NpmPackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", fmt.Errorf("parse npm response: %w", err)
	}
	return info.Version, info.Dist.Tarball, nil
}

// GetInstalledVersion reads the version from the local JSON data file.
func GetInstalledVersion(dataPath string) string {
	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return ""
	}
	var meta struct {
		SourceVersion string `json:"source_version"`
	}
	json.Unmarshal(raw, &meta)
	return meta.SourceVersion
}

// Update downloads the latest network.json from npm, parses it, and writes
// the bes JSON data file. Returns the new version and data stats.
func Update(dataPath string) (version string, stats map[string]int, err error) {
	ver, tarball, err := CheckLatestVersion()
	if err != nil {
		return "", nil, fmt.Errorf("check latest: %w", err)
	}
	fmt.Printf("Latest fingerprint-generator version: %s\n", ver)

	fmt.Printf("Downloading %s...\n", tarball)
	tgzData, err := download(tarball)
	if err != nil {
		return "", nil, fmt.Errorf("download tarball: %w", err)
	}

	fmt.Println("Extracting network.json...")
	networkJSON, err := extractNetworkJSON(tgzData)
	if err != nil {
		return "", nil, fmt.Errorf("extract network.json: %w", err)
	}

	fmt.Println("Parsing fingerprint data...")
	besJSON, err := parseNetworkToBesJSON(networkJSON, ver)
	if err != nil {
		return "", nil, fmt.Errorf("parse network.json: %w", err)
	}

	os.MkdirAll(filepath.Dir(dataPath), 0755)
	if err := os.WriteFile(dataPath, besJSON, 0644); err != nil {
		return "", nil, fmt.Errorf("write %s: %w", dataPath, err)
	}

	var result struct {
		Stats map[string]int `json:"stats"`
	}
	json.Unmarshal(besJSON, &result)

	fmt.Printf("Updated %s (%d bytes)\n", dataPath, len(besJSON))
	return ver, result.Stats, nil
}

// CheckAndUpdate compares installed version with latest npm version.
// If different (or installed version is empty), performs update.
func CheckAndUpdate(dataPath string) (updated bool, version string, err error) {
	latest, _, err := CheckLatestVersion()
	if err != nil {
		return false, "", err
	}
	installed := GetInstalledVersion(dataPath)
	if installed == latest {
		fmt.Printf("Already up-to-date (version %s)\n", installed)
		return false, installed, nil
	}
	fmt.Printf("Update available: %s → %s\n", installed, latest)
	ver, _, err := Update(dataPath)
	return err == nil, ver, err
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func extractNetworkJSON(tgzData []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "bes-fp-update")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tgzPath := filepath.Join(tmpDir, "fp.tgz")
	if err := os.WriteFile(tgzPath, tgzData, 0644); err != nil {
		return nil, err
	}

	pyScript := fmt.Sprintf(`
import tarfile, zipfile, os, sys
tgz = "%s"
outdir = "%s"
with tarfile.open(tgz, "r:gz") as tar:
    tar.extractall(outdir)
zippath = os.path.join(outdir, "package", "data_files", "fingerprint-network-definition.zip")
if not os.path.exists(zippath):
    print("ERROR: zip not found", file=sys.stderr)
    sys.exit(1)
with zipfile.ZipFile(zippath) as z:
    z.extractall(outdir)
jsonpath = os.path.join(outdir, "network.json")
with open(jsonpath) as f:
    sys.stdout.write(f.read())
`, tgzPath, tmpDir)

	pythonBin := "python3"
	if runtime.GOOS == "windows" {
		pythonBin = "python"
	}

	cmd := exec.Command(pythonBin, "-c", pyScript)
	var out, stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python extract: %w (%s)", err, stderr.String())
	}
	return []byte(out.String()), nil
}
