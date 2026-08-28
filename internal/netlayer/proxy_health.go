package netlayer

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyHealthChecker checks proxy availability and latency.
type ProxyHealthChecker struct {
	testURL string
	timeout time.Duration
	mu      sync.Mutex
	results map[string]*ProxyHealth
}

// ProxyHealth holds the health status of a proxy.
type ProxyHealth struct {
	URL       string    `json:"url"`
	Alive     bool      `json:"alive"`
	Latency   int64     `json:"latency_ms"` // milliseconds
	LastCheck time.Time `json:"last_check"`
	Error     string    `json:"error,omitempty"`
}

// NewProxyHealthChecker creates a health checker.
func NewProxyHealthChecker(testURL string) *ProxyHealthChecker {
	if testURL == "" {
		testURL = "https://httpbin.org/ip"
	}
	return &ProxyHealthChecker{
		testURL: testURL,
		timeout: 10 * time.Second,
		results: make(map[string]*ProxyHealth),
	}
}

// Check tests a single proxy.
func (c *ProxyHealthChecker) Check(proxyURL string) *ProxyHealth {
	c.mu.Lock()
	defer c.mu.Unlock()

	start := time.Now()
	health := &ProxyHealth{URL: proxyURL, LastCheck: time.Now()}

	// Parse proxy URL
	pu, err := url.Parse(proxyURL)
	if err != nil {
		health.Error = fmt.Sprintf("invalid proxy URL: %v", err)
		c.results[proxyURL] = health
		return health
	}

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(pu),
		DialContext: (&net.Dialer{
			Timeout: c.timeout,
		}).DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   c.timeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.testURL, nil)
	if err != nil {
		health.Error = err.Error()
		c.results[proxyURL] = health
		return health
	}

	resp, err := client.Do(req)
	if err != nil {
		health.Error = err.Error()
		c.results[proxyURL] = health
		return health
	}
	defer resp.Body.Close()

	health.Latency = time.Since(start).Milliseconds()
	health.Alive = resp.StatusCode == 200
	c.results[proxyURL] = health
	return health
}

// CheckAll tests all proxies in a pool concurrently.
func (c *ProxyHealthChecker) CheckAll(pool *ProxyPool) []*ProxyHealth {
	size := pool.Size()
	if size == 0 {
		return nil
	}

	results := make([]*ProxyHealth, size)
	var wg sync.WaitGroup

	for i := 0; i < size; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Get proxy by index (round-robin through the pool)
			proxy := pool.Get()
			if proxy != nil {
				results[idx] = c.Check(proxy.URL)
			}
		}(i)
	}

	wg.Wait()
	return results
}

// GetHealth returns cached health for a proxy.
func (c *ProxyHealthChecker) GetHealth(proxyURL string) *ProxyHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.results[proxyURL]
}

// GetAllHealth returns cached health for all checked proxies.
func (c *ProxyHealthChecker) GetAllHealth() []*ProxyHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]*ProxyHealth, 0, len(c.results))
	for _, h := range c.results {
		result = append(result, h)
	}
	return result
}

// SetTimeout configures the health check timeout.
func (c *ProxyHealthChecker) SetTimeout(d time.Duration) {
	c.timeout = d
}

// --- Enhanced ProxyPool with health-aware routing ---

// HealthyProxyPool wraps ProxyPool with health checking.
// Dead proxies are skipped; alive proxies are sorted by latency.
type HealthyProxyPool struct {
	pool    *ProxyPool
	checker *ProxyHealthChecker
	mu      sync.Mutex
	healthy []*ProxyConfig
	index   int
}

// NewHealthyProxyPool creates a health-aware proxy pool.
func NewHealthyProxyPool(urls []string) *HealthyProxyPool {
	return &HealthyProxyPool{
		pool:    NewProxyPool(urls),
		checker: NewProxyHealthChecker(""),
	}
}

// Refresh re-checks all proxies and rebuilds the healthy list.
func (h *HealthyProxyPool) Refresh() {
	results := h.checker.CheckAll(h.pool)
	h.mu.Lock()
	defer h.mu.Unlock()

	h.healthy = nil
	for _, r := range results {
		if r != nil && r.Alive {
			h.healthy = append(h.healthy, &ProxyConfig{URL: r.URL})
		}
	}
}

// Get returns the next healthy proxy (round-robin).
// Falls back to any proxy if no healthy ones available.
func (h *HealthyProxyPool) Get() *ProxyConfig {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.healthy) == 0 {
		return h.pool.Get()
	}
	proxy := h.healthy[h.index%len(h.healthy)]
	h.index++
	return proxy
}

// GetBest returns the proxy with the lowest latency.
func (h *HealthyProxyPool) GetBest() *ProxyConfig {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.healthy) == 0 {
		return h.pool.Get()
	}

	var best *ProxyConfig
	var bestLatency int64 = -1
	for _, p := range h.healthy {
		health := h.checker.GetHealth(p.URL)
		if health != nil && health.Alive {
			if bestLatency < 0 || health.Latency < bestLatency {
				best = p
				bestLatency = health.Latency
			}
		}
	}
	if best != nil {
		return best
	}
	return h.healthy[0]
}

// Size returns the number of healthy proxies.
func (h *HealthyProxyPool) Size() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.healthy)
}

// TotalSize returns the total number of proxies (including unhealthy).
func (h *HealthyProxyPool) TotalSize() int {
	return h.pool.Size()
}
