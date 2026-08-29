package netlayer

import "sync/atomic"

// ProxyConfig holds per-session proxy configuration.
type ProxyConfig struct {
	URL      string `json:"url"`  // "http://user:pass@host:port" or "socks5://host:port"
	Type     string `json:"type"` // "http", "socks5", "socks4"
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProxyPool manages a pool of proxies for session-unique assignment.
type ProxyPool struct {
	proxies []*ProxyConfig
	index   atomic.Int64
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
// The round-robin counter is atomically incremented so concurrent callers
// (e.g. parallel health checks) never race on the index.
func (p *ProxyPool) Get() *ProxyConfig {
	if len(p.proxies) == 0 {
		return nil
	}
	i := p.index.Add(1) - 1
	return p.proxies[i%int64(len(p.proxies))]
}

// Size returns the number of proxies in the pool.
func (p *ProxyPool) Size() int {
	return len(p.proxies)
}
