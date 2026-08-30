package netlayer

import (
	"testing"
)

// TestCheckBlockedURL_RejectInternal 验证 SSRF 防护拒绝内网/环回/链路本地/
// 云元数据地址（H2 验收：SSRF 内网拒识）。
func TestCheckBlockedURL_RejectInternal(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/",
		"http://127.0.0.1:8080/",
		"http://localhost/",        // 解析到 127.0.0.1/::1
		"http://169.254.169.254/",  // 云元数据
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://10.255.255.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://172.31.255.1/",
		"http://[::1]/",
		"http://[fc00::1]/",
		"http://[fe80::1]/",
		"http://metadata.google.internal/",
		"ftp://127.0.0.1/",         // 非 http/https 协议
		"http:///path",             // 空 host
	}
	for _, u := range blocked {
		if err := CheckBlockedURL(u); err == nil {
			t.Errorf("expected block for %q, got allow", u)
		}
	}
}

// TestCheckBlockedURL_AllowPublic 验证公网地址不被误杀（避免回归）。
func TestCheckBlockedURL_AllowPublic(t *testing.T) {
	// 仅校验 IP 字面量与协议/host 形态；不依赖真实 DNS（公网域名解析在
	// 无网络环境会走 nil 返回放行，符合"DNS 失败不阻塞"设计）。
	cases := []string{
		"http://8.8.8.8/",
		"https://1.1.1.1/",
		"http://8.8.8.8:80/path?q=1",
	}
	for _, u := range cases {
		if err := CheckBlockedURL(u); err != nil {
			t.Errorf("expected allow for %q, got %v", u, err)
		}
	}
}
