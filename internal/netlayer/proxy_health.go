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

