package fpupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSafeJoin_RejectEscape 验证 tar/zip entry 路径逃逸被拒（tar slip 防护核心）。
func TestSafeJoin_RejectEscape(t *testing.T) {
	base := t.TempDir()
	blocked := []string{
		"../escape.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"C:\\windows\\win.ini",
		"sub/../../escape.txt",
		"//host/share/x",
	}
	for _, name := range blocked {
		if _, err := safeJoin(base, name); err == nil {
			t.Errorf("expected reject for %q, got allow", name)
		}
	}
}

// TestSafeJoin_AllowInTree 验证合法相对路径被接受且落在 base 内。
func TestSafeJoin_AllowInTree(t *testing.T) {
	base := t.TempDir()
	cases := []string{
		"network.json",
		"package/data_files/x.zip",
		"./a/b.js",
	}
	for _, name := range cases {
		got, err := safeJoin(base, name)
		if err != nil {
			t.Errorf("expected allow for %q, got %v", name, err)
			continue
		}
		rel, err := filepath.Rel(base, got)
		if err != nil || strings.HasPrefix(filepath.ToSlash(rel), "../") {
			t.Errorf("safeJoin(%q) escaped base: rel=%q err=%v", name, rel, err)
		}
	}
}

// TestExtractTarGz_RejectTarSlip 构造一个含逃逸 entry 的恶意 tar.gz，
// 验证 extractTarGz 拒绝它且不写入 base 之外（H13 验收：tar slip 逃逸拒识）。
func TestExtractTarGz_RejectTarSlip(t *testing.T) {
	base := t.TempDir()
	// 构造恶意 tar：含一个 ../evil.txt 逃逸 entry。
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name:     "../evil.txt",
		Typeflag: tar.TypeReg,
		Size:     int64(len("pwned")),
		Mode:     0644,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("pwned"))
	tw.Close()
	gw.Close()

	err := extractTarGz(buf.Bytes(), base)
	if err == nil {
		t.Fatal("expected extractTarGz to reject tar-slip entry, got nil")
	}
	// 确认逃逸文件未被写出。
	parent := filepath.Dir(base)
	evilPath := filepath.Join(parent, "evil.txt")
	if _, statErr := os.Stat(evilPath); statErr == nil {
		t.Fatalf("tar slip wrote outside base to %s", evilPath)
	}
}

// TestExtractTarGz_RejectSymlink 验证符号链接 entry 被拒（可指向 dest 外）。
func TestExtractTarGz_RejectSymlink(t *testing.T) {
	base := t.TempDir()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name:     "link.txt",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0777,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()

	if err := extractTarGz(buf.Bytes(), base); err == nil {
		t.Fatal("expected extractTarGz to reject symlink entry, got nil")
	}
}
