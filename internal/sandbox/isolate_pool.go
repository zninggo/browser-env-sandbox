package sandbox

import (
	"sync"

	"rogchap.com/v8go"
)

// IsolatePool manages a pool of V8 Isolates for reuse.
// Creating a new Isolate is expensive; pooling dramatically improves
// throughput for multi-session workloads.
type IsolatePool struct {
	pool chan *v8go.Isolate
	mu   sync.Mutex
	size int
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
		return v8go.NewIsolate()
	}
}

// Put returns an Isolate to the pool for reuse.
// If the pool is full, the Isolate is disposed.
func (p *IsolatePool) Put(iso *v8go.Isolate) {
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

// DisposeAll disposes all Isolates in the pool.
func (p *IsolatePool) DisposeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		select {
		case iso := <-p.pool:
			iso.Dispose()
		default:
			return
		}
	}
}
