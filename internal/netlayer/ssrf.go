// ssrf.go — SSRF 防护：阻止 sandbox 在 live 模式下被当作出站代理，代发
// 请求到内网/环回/链路本地（含云元数据 169.254.169.254）地址。
//
// 持 token 的客户端可提交任意 urlStr；若不过滤目标，bes 即可被用来探测
// 169.254.169.254（云元数据）、127.0.0.1、RFC1918 内网段。CheckBlockedURL
// 在 handleLive 入口统一拦截：仅放行 http/https，拒绝保留段 IP（IP 字面量
// 直接判，域名经 DNS 解析后逐条复核，防 DNS rebinding 落到内网）。

package netlayer

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockedV4CIDRs 是拒绝的 IPv4 保留/内网段。
var blockedV4CIDRs = []string{
	"0.0.0.0/8",      // 本网络
	"10.0.0.0/8",     // RFC1918 私有
	"100.64.0.0/10",  // CGNAT (RFC6598)
	"127.0.0.0/8",    // 环回
	"169.254.0.0/16", // 链路本地（含云元数据 169.254.169.254）
	"172.16.0.0/12",  // RFC1918 私有
	"192.0.0.0/24",   // IETF 协议分配
	"192.168.0.0/16", // RFC1918 私有
	"198.18.0.0/15",  // 基准测试
	"224.0.0.0/4",    // 组播
	"240.0.0.0/4",    // 保留
}

// blockedV6CIDRs 是拒绝的 IPv6 保留/内网段。
var blockedV6CIDRs = []string{
	"::1/128",       // 环回
	"fc00::/7",      // ULA 私有
	"fe80::/10",     // 链路本地
	"ff00::/8",      // 组播
	"::/128",        // 未指定
	"::ffff:0:0/96", // IPv4 映射（按 v4 段判定，此处兜底）
}

var blockedV4Nets []*net.IPNet
var blockedV6Nets []*net.IPNet

func init() {
	for _, c := range blockedV4CIDRs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			blockedV4Nets = append(blockedV4Nets, n)
		}
	}
	for _, c := range blockedV6CIDRs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			blockedV6Nets = append(blockedV6Nets, n)
		}
	}
}

// isMetadataHost 报告主机名是否为已知云元数据端点（IP 形式由 CIDR 段覆盖）。
func isMetadataHost(host string) bool {
	h := strings.ToLower(host)
	switch h {
	case "metadata.google.internal", "metadata", "instance-data":
		return true
	}
	if strings.HasSuffix(h, ".metadata.aws") || strings.HasSuffix(h, ".metadata") {
		return true
	}
	return false
}

// isBlockedIP 报告 IP 是否落在拒绝的保留/内网段。
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		for _, n := range blockedV4Nets {
			if n.Contains(v4) {
				return true
			}
		}
		return false
	}
	for _, n := range blockedV6Nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckBlockedURL 校验 URL 是否命中 SSRF 拦截。
//   - 仅允许 http/https；
//   - 拒绝空 host、云元数据主机名；
//   - IP 字面量直接判保留段；
//   - 域名经 net.LookupIP 解析后逐条复核（任一命中内网即拒，防 rebinding）；
//   - DNS 解析失败不阻塞：放行交由拨号阶段处理，避免无 DNS 环境误杀。
func CheckBlockedURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("ssrf: invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("ssrf: scheme %q not allowed", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("ssrf: empty host")
	}
	if isMetadataHost(host) {
		return fmt.Errorf("ssrf: metadata host %q blocked", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("ssrf: ip %s blocked", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("ssrf: host %q resolves to blocked %s", host, ip)
		}
	}
	return nil
}
