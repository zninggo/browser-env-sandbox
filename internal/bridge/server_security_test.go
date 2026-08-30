package bridge

import (
	"testing"
)

// TestResolvePreloadPath_RejectTraversal 验证 preload 白名单拒绝 .. 穿越、
// 绝对路径、盘符、UNC（H1 验收：preload 穿越拒识）。
func TestResolvePreloadPath_RejectTraversal(t *testing.T) {
	blocked := []string{
		"../secret.txt",
		"../../etc/passwd",
		"..\\..\\windows\\win.ini",
		"/etc/passwd",
		"/windows/system32/config/sam",
		"C:\\windows\\win.ini",
		"c:/users/admin/secret",
		"//server/share/secret",   // UNC
		`\\server\share\secret`,   // UNC backslash
		"sub/../../escape.txt",
	}
	for _, name := range blocked {
		_, err := resolvePreloadPath(name)
		if err == nil {
			t.Errorf("expected reject for %q, got allow", name)
		}
	}
}

// TestResolvePreloadPath_AllowInTree 验证白名单目录内的相对名被接受，
// 且解析结果落在 preloadDir 子树内（避免回归）。
func TestResolvePreloadPath_AllowInTree(t *testing.T) {
	cases := map[string]string{
		"init.js":                "data/preload/init.js",
		"stealth/anti-bot.js":    "data/preload/stealth/anti-bot.js",
		"./normalize.js":         "data/preload/normalize.js",
	}
	for name, wantPrefix := range cases {
		got, err := resolvePreloadPath(name)
		if err != nil {
			t.Errorf("expected allow for %q, got %v", name, err)
			continue
		}
		if got != wantPrefix {
			t.Errorf("resolvePreloadPath(%q) = %q, want %q", name, got, wantPrefix)
		}
	}
}
