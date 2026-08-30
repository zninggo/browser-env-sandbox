// Package main implements the bes-bench performance benchmark.
//
// Measures Isolate pool reuse vs. fresh creation, and sandbox session
// creation + eval throughput. Supports --baseline (save) and --compare
// (compare against saved baseline) for CI performance regression.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

type benchResult struct {
	Name       string  `json:"name"`
	Iterations int     `json:"iterations"`
	TotalMs    int64   `json:"total_ms"`
	AvgUs      int64   `json:"avg_us"`
	P50Us      int64   `json:"p50_us"`
	P99Us      int64   `json:"p99_us"`
	OpsPerSec  float64 `json:"ops_per_sec"`
	MemAllocMB float64 `json:"mem_alloc_mb"`
}

func main() {
	baselineFile := flag.String("baseline", "", "save results to this JSON file")
	compareFile := flag.String("compare", "", "compare results against this baseline JSON file")
	flag.Parse()

	fpEng := fpengine.New()

	benchPooled := benchCreateEval(fpEng, 8, 100, "pooled-isolate")
	benchFresh := benchCreateEval(fpEng, 1, 100, "fresh-isolate")
	benchEval := benchEvalOnly(fpEng, 500)

	results := []benchResult{benchPooled, benchFresh, benchEval}
	for _, r := range results {
		fmt.Printf("%-25s  %d iters  avg=%dµs  p99=%dµs  %.0f ops/s\n",
			r.Name, r.Iterations, r.AvgUs, r.P99Us, r.OpsPerSec)
	}

	if *baselineFile != "" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*baselineFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write baseline failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Baseline saved to %s\n", *baselineFile)
	}

	if *compareFile != "" {
		regressed := compareBaseline(*compareFile, results)
		if regressed > 0 {
			fmt.Fprintf(os.Stderr, "PERFORMANCE REGRESSION: %d metric(s) degraded >25%%\n", regressed)
			os.Exit(1)
		}
		fmt.Println("No performance regression detected.")
	}

	json.NewEncoder(os.Stdout).Encode(results)
}

// compareBaseline loads the baseline file and compares. Returns the number of
// metrics that regressed beyond 25% threshold.
func compareBaseline(path string, current []benchResult) int {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read baseline failed: %v\n", err)
		return 0
	}
	var baseline []benchResult
	if json.Unmarshal(data, &baseline) != nil {
		fmt.Fprintf(os.Stderr, "parse baseline failed\n")
		return 0
	}
	baseMap := make(map[string]benchResult)
	for _, b := range baseline {
		baseMap[b.Name] = b
	}
	regressed := 0
	for _, c := range current {
		b, ok := baseMap[c.Name]
		if !ok {
			continue
		}
		// Latency regression: avg/p50/p99 increased >25%
		if c.AvgUs > b.AvgUs*125/100 {
			fmt.Printf("  ⚠️  %s avg regressed: %dµs → %dµs (+%d%%)\n", c.Name, b.AvgUs, c.AvgUs, (c.AvgUs*100/b.AvgUs)-100)
			regressed++
		}
		if c.P99Us > b.P99Us*125/100 {
			fmt.Printf("  ⚠️  %s p99 regressed: %dµs → %dµs (+%d%%)\n", c.Name, b.P99Us, c.P99Us, (c.P99Us*100/b.P99Us)-100)
			regressed++
		}
		// Throughput regression: ops/s dropped >25%
		if c.OpsPerSec < b.OpsPerSec*0.75 {
			fmt.Printf("  ⚠️  %s throughput regressed: %.0f → %.0f ops/s (-%.0f%%)\n", c.Name, b.OpsPerSec, c.OpsPerSec, (1-c.OpsPerSec/b.OpsPerSec)*100)
			regressed++
		}
		// No regression case
		if c.AvgUs <= b.AvgUs*125/100 && c.P99Us <= b.P99Us*125/100 && c.OpsPerSec >= b.OpsPerSec*0.75 {
			fmt.Printf("  ✅  %s OK (avg %dµs, p99 %dµs, %.0f ops/s)\n", c.Name, c.AvgUs, c.P99Us, c.OpsPerSec)
		}
	}
	return regressed
}

func benchCreateEval(fpEng *fpengine.Engine, poolSize, iters int, name string) benchResult {
	eng := sandbox.New(fpEng, poolSize)
	defer eng.Dispose()

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
	sorted := make([]int64, len(times))
	copy(sorted, times)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
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
