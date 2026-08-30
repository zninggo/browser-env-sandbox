// Package fpupdate — automatic fingerprint data updater.
//
// Downloads the latest apify/fingerprint-suite network.json from npm,
// parses it into bes's JSON format, and writes to data/fp_real_data.json.
// Also checks the npm registry for version changes to enable auto-updates.
package fpupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	npmRegistryURL = "https://registry.npmjs.org/fingerprint-generator/latest"
	networkZipPath = "package/data_files/fingerprint-network-definition.zip"

	// downloadTimeout 限制单次 HTTP 请求时长，防止慢速攻击挂起更新流程。
	downloadTimeout = 60 * time.Second
	// maxTarballBytes 是允许下载的 tarball 上限，防止无界 io.ReadAll 造成 DoS。
	maxTarballBytes = 64 * 1024 * 1024 // 64 MiB
	// maxRegistryBytes 是 npm registry JSON 响应上限。
	maxRegistryBytes = 1 << 20 // 1 MiB
	// allowedTarballHost 是 tarball 下载唯一允许的主机，防 SSRF/恶意源。
	allowedTarballHost = "registry.npmjs.org"
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
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(npmRegistryURL)
	if err != nil {
		return "", "", fmt.Errorf("npm registry query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("npm registry returned %d", resp.StatusCode)
	}
	var info NpmPackageInfo
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxRegistryBytes))
	if err := dec.Decode(&info); err != nil {
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

// assertAllowedTarball 校验 tarball URL 只指向白名单官方源，防 SSRF。
func assertAllowedTarball(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid tarball url: %w", err)
	}
	if strings.ToLower(u.Scheme) != "https" {
		return fmt.Errorf("tarball scheme must be https, got %q", u.Scheme)
	}
	if u.Hostname() != allowedTarballHost {
		return fmt.Errorf("tarball host %q not whitelisted", u.Hostname())
	}
	return nil
}

func download(rawURL string) ([]byte, error) {
	if err := assertAllowedTarball(rawURL); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTarballBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxTarballBytes {
		return nil, fmt.Errorf("tarball exceeds %d bytes", maxTarballBytes)
	}
	return data, nil
}

// safeJoin 将 name 解析到 base 目录内，拒绝绝对路径、盘符、UNC、.. 穿越，
// 防止 tar/zip entry 逃逸到目标目录外（tar slip / zip slip）。
// 反斜杠统一归一化为正斜杠后再判，确保跨平台一致拦截 Windows 风格逃逸。
func safeJoin(base, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty path")
	}
	slash := strings.ReplaceAll(name, "\\", "/")
	slash = filepath.ToSlash(slash)
	// 拒绝绝对路径（Unix 风格 / 开头 或 Windows 盘符 C:）。
	if strings.HasPrefix(slash, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	if len(slash) >= 2 && slash[1] == ':' {
		return "", fmt.Errorf("drive letter not allowed")
	}
	// 拒绝 UNC 路径。
	if strings.HasPrefix(slash, "//") {
		return "", fmt.Errorf("unc path not allowed")
	}
	// 强制相对化：剥掉所有前导分隔符。
	slash = strings.TrimLeft(slash, "/")
	if slash == "" || slash == "." {
		return "", fmt.Errorf("empty relative path")
	}
	cleaned := filepath.ToSlash(filepath.Clean(slash))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("escapes base dir")
	}
	abs := filepath.Join(base, cleaned)
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return "", err
	}
	relSlash := filepath.ToSlash(rel)
	if relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return "", fmt.Errorf("escapes base dir")
	}
	return abs, nil
}

// extractTarGz 解压 tar.gz 到 dest，逐 entry 用 safeJoin 校验不逃逸，并
// 拒绝符号链接/硬链接 entry（可指向 dest 外构成逃逸）。
func extractTarGz(tgzData []byte, dest string) error {
	zr, err := gzip.NewReader(bytes.NewReader(tgzData))
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return fmt.Errorf("tar entry %q: %w", hdr.Name, err)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)&0777); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode)&0777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("symlink/link entry %q not allowed", hdr.Name)
		default:
			// 跳过 FIFO/字符设备等非常规 entry。
		}
	}
	return nil
}

func extractNetworkJSON(tgzData []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "bes-fp-update")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(tgzData, tmpDir); err != nil {
		return nil, err
	}

	zipPath := filepath.Join(tmpDir, networkZipPath)
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open inner zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		target, err := safeJoin(tmpDir, f.Name)
		if err != nil {
			return nil, fmt.Errorf("zip entry %q: %w", f.Name, err)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return nil, err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return nil, err
		}
		rc.Close()
		out.Close()
	}

	jsonPath := filepath.Join(tmpDir, "network.json")
	return os.ReadFile(jsonPath)
}
