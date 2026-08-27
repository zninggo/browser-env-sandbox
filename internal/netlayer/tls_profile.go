package netlayer

// TLSProfile represents a TLS fingerprint configuration for a specific
// browser version. Used to match JA3/JA4 fingerprints with the UA.
//
// In live mode, this configures curl-impersonate to produce matching
// TLS handshakes. In replay mode, it's metadata only.
type TLSProfile struct {
	Browser     string `json:"browser"`      // "chrome"
	Version     string `json:"version"`      // "131"
	Impersonate string `json:"impersonate"`  // curl_cffi impersonate target
	JA3         string `json:"ja3"`          // JA3 fingerprint hash
	JA4         string `json:"ja4"`          // JA4 fingerprint hash
	H2Settings  []uint32 `json:"h2_settings"` // HTTP/2 SETTINGS frame values
	H2PseudoOrder []string `json:"h2_pseudo_order"` // HTTP/2 pseudo-header order
	HeaderOrder  []string `json:"header_order"`    // HTTP/1.1 header order
}

// DefaultTLSProfiles returns the built-in TLS profiles for known browsers.
// These match the curl_cffi impersonate targets.
func DefaultTLSProfiles() map[string]*TLSProfile {
	return map[string]*TLSProfile{
		"chrome131": {
			Browser:     "chrome",
			Version:     "131",
			Impersonate: "chrome131",
			JA3:         "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
			H2Settings:  []uint32{1, 65536, 0, 4, 3, 15663105, 0},
			H2PseudoOrder: []string{":method", ":authority", ":scheme", ":path"},
			HeaderOrder: []string{"user-agent", "accept", "accept-language", "accept-encoding", "content-type", "origin", "referer", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform"},
		},
		"chrome124": {
			Browser:     "chrome",
			Version:     "124",
			Impersonate: "chrome124",
			JA3:         "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
			H2Settings:  []uint32{1, 65536, 0, 4, 3, 15663105, 0},
			H2PseudoOrder: []string{":method", ":authority", ":scheme", ":path"},
			HeaderOrder: []string{"user-agent", "accept", "accept-language", "accept-encoding", "content-type", "origin", "referer", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform"},
		},
		"chrome120": {
			Browser:     "chrome",
			Version:     "120",
			Impersonate: "chrome120",
			JA3:         "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0",
			H2Settings:  []uint32{1, 65536, 0, 4, 3, 15663105, 0},
			H2PseudoOrder: []string{":method", ":authority", ":scheme", ":path"},
			HeaderOrder: []string{"user-agent", "accept", "accept-language", "accept-encoding", "content-type", "origin", "referer", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform"},
		},
	}
}

// GetTLSProfile returns the TLS profile for a given browser version.
// Falls back to chrome131 if the exact version isn't found.
func GetTLSProfile(browser, version string) *TLSProfile {
	profiles := DefaultTLSProfiles()
	key := browser + version
	if p, ok := profiles[key]; ok {
		return p
	}
	// Fallback: use the closest version
	return profiles["chrome131"]
}

// ProxyConfig holds per-session proxy configuration.
type ProxyConfig struct {
	URL      string `json:"url"`       // "http://user:pass@host:port" or "socks5://host:port"
	Type     string `json:"type"`      // "http", "socks5", "socks4"
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProxyPool manages a pool of proxies for session-unique assignment.
type ProxyPool struct {
	proxies []*ProxyConfig
	index   int
}

// NewProxyPool creates a proxy pool from a list of proxy URLs.
func NewProxyPool(urls []string) *ProxyPool {
	pool := &ProxyPool{}
	for _, u := range urls {
		pool.proxies = append(pool.proxies, &ProxyConfig{URL: u})
	}
	return pool
}

// Get returns the next proxy in round-robin order.
func (p *ProxyPool) Get() *ProxyConfig {
	if len(p.proxies) == 0 {
		return nil
	}
	proxy := p.proxies[p.index%len(p.proxies)]
	p.index++
	return proxy
}

// Size returns the number of proxies in the pool.
func (p *ProxyPool) Size() int {
	return len(p.proxies)
}
