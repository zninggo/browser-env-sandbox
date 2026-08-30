package sandbox

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/zninggo/v8go"
)

// Default V8 heap limits. The initial heap is kept small for fast startup;
// the max prevents runaway scripts from OOM-killing the process. Override
// via BES_POOL_MEM_MB (max heap in MB; initial is always 8MB).
const (
	defaultInitialHeapMB = 8
	defaultMaxHeapMB     = 512
)

// maxHeapMB returns the configured max heap size in MB.
func maxHeapMB() uint64 {
	if v := os.Getenv("BES_POOL_MEM_MB"); v != "" {
		if mb, err := strconv.ParseUint(v, 10, 64); err == nil && mb > 0 {
			return mb
		}
	}
	return defaultMaxHeapMB
}

// newIsolateWithLimits creates a V8 Isolate with resource constraints.
func newIsolateWithLimits() *v8go.Isolate {
	initial := uint64(defaultInitialHeapMB) * 1024 * 1024
	max := maxHeapMB() * 1024 * 1024
	return v8go.NewIsolate(v8go.WithResourceConstraints(initial, max))
}

// IsolatePool manages a pool of V8 Isolates for reuse.
// Creating a new Isolate is expensive; pooling dramatically improves
// throughput for multi-session workloads.
type IsolatePool struct {
	pool   chan *v8go.Isolate
	mu     sync.Mutex
	size   int
	closed atomic.Bool // set by DisposeAll; Put checks it to avoid returning to a dead pool
}

// NewIsolatePool creates a pool with the given size.
// Isolates are created lazily on demand.
func NewIsolatePool(size int) *IsolatePool {
	return &IsolatePool{
		pool: make(chan *v8go.Isolate, size),
		size: size,
	}
}

// Get retrieves an Isolate from the pool, or creates a new one if empty.
func (p *IsolatePool) Get() *v8go.Isolate {
	select {
	case iso := <-p.pool:
		return iso
	default:
		return newIsolateWithLimits()
	}
}

// Put returns an Isolate to the pool for reuse.
// If the pool is full (or the pool has been disposed via DisposeAll), the
// Isolate is disposed instead of returned. The closed check prevents a session
// that outlives Engine.Dispose from quietly returning its Isolate to a pool
// whose channel DisposeAll has already drained — such an Isolate would never be
// reclaimed (the next DisposeAll is not coming), so it is disposed on the spot.
func (p *IsolatePool) Put(iso *v8go.Isolate) {
	if p.closed.Load() {
		iso.Dispose()
		return
	}
	// TODO: Reset the isolate state before returning to pool.
	// Currently we just return it as-is; a full reset would require
	// creating a fresh context anyway, so the benefit is mainly
	// avoiding the Isolate creation cost.
	select {
	case p.pool <- iso:
		// returned to pool
	default:
		// pool full, dispose
		iso.Dispose()
	}
}

// DisposeAll disposes all Isolates in the pool and marks it closed so subsequent
// Put calls dispose their Isolates on the spot rather than returning them to a
// dead pool (which would leak — no future DisposeAll would reclaim them).
func (p *IsolatePool) DisposeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed.Store(true)
	for {
		select {
		case iso := <-p.pool:
			iso.Dispose()
		default:
			return
		}
	}
}
