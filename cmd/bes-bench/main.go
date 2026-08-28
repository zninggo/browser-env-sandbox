// Package main implements the bes-bench performance benchmark.
//
// Measures Isolate pool reuse vs. fresh creation, and sandbox session
// creation + eval throughput. Outputs JSON results to stdout.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
)

type benchResult struct {
	Name        string  `json:"name"`
	Iterations  int     `json:"iterations"`
	TotalMs     int64   `json:"total_ms"`
	AvgUs       int64   `json:"avg_us"`
	P50Us       int64   `json:"p50_us"`
	P99Us       int64   `json:"p99_us"`
	OpsPerSec   float64 `json:"ops_per_sec"`
	MemAllocMB  float64 `json:"mem_alloc_mb"`
}

func main() {
	fpEng := fpengine.New()

	// Benchmark 1: Pooled Isolate reuse — CreateSession + Eval + Dispose
	benchPooled := benchCreateEval(fpEng, 8, 100, "pooled-isolate")
	// Benchmark 2: Fresh Isolate (pool size 1, forced creation each time)
	benchFresh := benchCreateEval(fpEng, 1, 100, "fresh-isolate")
	// Benchmark 3: Eval-only on a pre-warmed session
	benchEval := benchEvalOnly(fpEng, 500)

	results := []benchResult{benchPooled, benchFresh, benchEval}
	for _, r := range results {
		fmt.Printf("%-25s  %d iters  avg=%dµs  p99=%dµs  %.0f ops/s\n",
			r.Name, r.Iterations, r.AvgUs, r.P99Us, r.OpsPerSec)
	}
	json.NewEncoder(os.Stdout).Encode(results)
}

func benchCreateEval(fpEng *fpengine.Engine, poolSize, iters int, name string) benchResult {
	eng := sandbox.New(fpEng, poolSize)
	defer eng.Dispose()

	// Warmup
	for i := 0; i < 5; i++ {
		sess, _ := eng.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
		sess.Eval("1+1")
		sess.Dispose()
	}

	times := make([]int64, iters)
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	start := time.Now()
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		sess, err := eng.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "create error: %v\n", err)
			continue
		}
		sess.Eval("navigator.userAgent")
		sess.Dispose()
		times[i] = time.Since(t0).Microseconds()
	}
	totalMs := time.Since(start).Milliseconds()
	runtime.ReadMemStats(&m2)

	return computeResult(name, iters, times, totalMs, float64(m2.TotalAlloc-m1.TotalAlloc)/1e6)
}

func benchEvalOnly(fpEng *fpengine.Engine, iters int) benchResult {
	eng := sandbox.New(fpEng, 4)
	defer eng.Dispose()
	sess, _ := eng.CreateSession(api.SessionOptions{Browser: "chrome", OS: "windows"})
	defer sess.Dispose()

	// Warmup
	for i := 0; i < 10; i++ {
		sess.Eval("1+1")
	}

	times := make([]int64, iters)
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	start := time.Now()
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		sess.Eval("navigator.userAgent + ' bench'")
		times[i] = time.Since(t0).Microseconds()
	}
	totalMs := time.Since(start).Milliseconds()
	runtime.ReadMemStats(&m2)

	return computeResult("eval-only-warm", iters, times, totalMs, float64(m2.TotalAlloc-m1.TotalAlloc)/1e6)
}

func computeResult(name string, iters int, times []int64, totalMs int64, memMB float64) benchResult {
	// Sort for percentiles (simple insertion sort for small N)
	sorted := make([]int64, len(times))
	copy(sorted, times)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	var sum int64
	for _, t := range times {
		sum += t
	}
	avg := sum / int64(len(times))
	p50 := sorted[len(sorted)*50/100]
	p99 := sorted[len(sorted)*99/100]
	opsPerSec := float64(iters) / (float64(totalMs) / 1000.0)
	return benchResult{
		Name:       name,
		Iterations: iters,
		TotalMs:    totalMs,
		AvgUs:      avg,
		P50Us:      p50,
		P99Us:      p99,
		OpsPerSec:  opsPerSec,
		MemAllocMB: memMB,
	}
}
