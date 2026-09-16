// Package main implements the bes CLI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/zninggo/browser-env-sandbox/internal/executor"
	"github.com/zninggo/browser-env-sandbox/internal/fpengine"
	"github.com/zninggo/browser-env-sandbox/internal/fpupdate"
	"github.com/zninggo/browser-env-sandbox/internal/mcp"
	"github.com/zninggo/browser-env-sandbox/internal/sandbox"
	"github.com/zninggo/browser-env-sandbox/pkg/api"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "fingerprint":
		cmdFingerprint(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "export-fp":
		cmdExportFP(os.Args[2:])
	case "selftest":
		cmdSelftest(os.Args[2:])
	case "dual-check":
		cmdDualCheck(os.Args[2:])
	case "mcp":
		cmdMCP(os.Args[2:])
	case "update-fp":
		cmdUpdateFP(os.Args[2:])
	case "version":
		fmt.Println("bes v0.2.0 (V8 engine: v8go v0.35.1, V8 15.2)")
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `bes — browser-env-sandbox CLI

Usage:
  bes fingerprint [--browser chrome] [--os windows] [--seed 0]
  bes run (--script <file> | --eval <code>) [--browser chrome] [--os windows] [--seed 0] [--location <url>]
  bes export-fp --output <file> [--browser chrome] [--os windows] [--seed 0]
  bes selftest
  bes dual-check [--browser chrome] [--os windows] [--seed 0] [--timezone <tz>] [--json]
  bes mcp                    # Start MCP server (stdio, for AI agents)
  bes update-fp              # Download latest fingerprint data from npm
  bes version
`)
}

func cmdUpdateFP(args []string) {
	dataPath := "data/fp_real_data.json"
	if p := os.Getenv("BES_FP_DATA"); p != "" {
		dataPath = p
	}
	updated, version, err := fpupdate.CheckAndUpdate(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	if updated {
		fmt.Printf("\nFingerprint data updated to version %s\n", version)
		fmt.Printf("Data file: %s\n", dataPath)
		fpengine.ReloadFpRealData()
		fpengine.ReloadCanvasDataset()
		stats := fpengine.FpDataStats()
		fmt.Printf("Loaded: %d GPUs, %d screens, %d hardwareConcurrency, %d deviceMemory\n",
			stats["gpus"], stats["screens"], stats["hardware_concurrency"], stats["device_memory"])
	} else {
		fmt.Printf("Already up-to-date (version %s)\n", version)
	}
}

func cmdExportFP(args []string) {
	fs := flag.NewFlagSet("export-fp", flag.ExitOnError)
	browser := fs.String("browser", "chrome", "Browser type")
	osName := fs.String("os", "windows", "Operating system")
	seed := fs.Uint64("seed", 0, "Fingerprint seed (0 = random)")
	output := fs.String("output", "", "Output file path (required)")
	fs.Parse(args)

	if *output == "" {
		fmt.Fprintln(os.Stderr, "Error: --output required")
		os.Exit(1)
	}

	eng := fpengine.New()
	fp, err := eng.Generate(*seed, *browser, *osName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := fpengine.ExportToFile(fp, *output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Fingerprint exported to %s (seed: %d)\n", *output, fp.Seed)
}

func cmdSelftest(args []string) {
	fmt.Println("Run ./bes-selftest for the full self-test suite")
}

// cmdDualCheck verifies the dual-executor contract: ONE device profile
// driving both the V8 sandbox and a real local Chrome (CDP), with identity
// points compared field-by-field and canvas/wasm checked for real execution.
func cmdDualCheck(args []string) {
	fs := flag.NewFlagSet("dual-check", flag.ExitOnError)
	browser := fs.String("browser", "chrome", "Browser type")
	osName := fs.String("os", "windows", "Operating system")
	seed := fs.Uint64("seed", 0, "Fingerprint seed (0 = random)")
	timezone := fs.String("timezone", "", "IANA timezone (empty = random from KB)")
	timeout := fs.Int("timeout", 30, "Per-probe timeout (seconds)")
	jsonOut := fs.Bool("json", false, "Print the report as JSON")
	fs.Parse(args)

	fpEng := fpengine.New()
	v8 := executor.NewV8Executor(fpEng, 2)
	defer v8.Dispose()
	chrome := executor.NewChromeExecutor()
	defer chrome.Dispose()

	snap, err := executor.TakeSnapshot(fpEng, *seed, *browser, *osName, *timezone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout+45)*time.Second)
	defer cancel()
	rep, err := executor.VerifyConsistency(ctx, snap, v8, chrome, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		out, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(out))
	} else {
		printDualCheckReport(rep)
	}

	chromeAvailable := chrome.Available()
	if !rep.AllPass || (chromeAvailable && !rep.RealPass) {
		os.Exit(1)
	}
}

func printDualCheckReport(rep *executor.ConsistencyReport) {
	fmt.Printf("Profile: %s\n\n", rep.Profile)

	fmt.Println("Identity consistency (profile vs V8 vs Chrome):")
	fmt.Printf("  %-22s %-44s %-6s %-6s %s\n", "field", "want", "v8", "chrome", "pass")
	for _, f := range rep.Fields {
		chromeCol := "-"
		if rep.ChromeRan {
			chromeCol = mark(f.Chrome == f.Want)
		}
		fmt.Printf("  %-22s %-44s %-6s %-6s %t%s\n",
			f.Field, truncFor(f.Want, 44), mark(f.V8 == f.Want), chromeCol,
			f.Pass, failNote(f))
	}
	if rep.V8Error != "" {
		fmt.Printf("  v8 error: %s\n", rep.V8Error)
	}
	if rep.ChromeError != "" {
		fmt.Printf("  chrome error: %s\n", rep.ChromeError)
	}

	fmt.Println("\nReal execution (Chrome):")
	fmt.Printf("  canvas: %s\n", canvasLine(rep.Canvas))
	fmt.Printf("  wasm:   %s\n", wasmLine(rep.Wasm))
	fmt.Println("V8 (mocked, for contrast):")
	fmt.Printf("  canvas: %s\n", canvasLine(rep.V8Canvas))
	fmt.Printf("  wasm:   %s\n", wasmLine(rep.V8Wasm))

	fmt.Printf("\nSummary: %s\n", rep.Summary())
}

func truncFor(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func mark(ok bool) string {
	if ok {
		return "ok"
	}
	return "DIFF"
}

func failNote(f executor.FieldCheck) string {
	if f.Pass {
		return ""
	}
	return "  [" + f.Note + "]"
}

func canvasLine(m map[string]any) string {
	if m == nil {
		return "unavailable"
	}
	png, _ := m["png"].(bool)
	length := 0
	switch l := m["len"].(type) {
	case float64:
		length = int(l)
	}
	return fmt.Sprintf("png=%t len=%d", png, length)
}

func wasmLine(m map[string]any) string {
	if m == nil {
		return "unavailable"
	}
	if ok, _ := m["ok"].(bool); ok {
		val, _ := m["val"].(float64)
		return fmt.Sprintf("compiled+ran, f() = %d", int(val))
	}
	why, _ := m["why"].(string)
	if why == "" {
		why = "no"
	}
	return "not real: " + why
}

func cmdMCP(args []string) {
	// Start MCP server (stdio JSON-RPC for AI agents)
	server := mcp.New()
	defer server.Dispose()
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

func cmdFingerprint(args []string) {
	fs := flag.NewFlagSet("fingerprint", flag.ExitOnError)
	browser := fs.String("browser", "chrome", "Browser type")
	osName := fs.String("os", "windows", "Operating system")
	seed := fs.Uint64("seed", 0, "Fingerprint seed (0 = random)")
	fs.Parse(args)

	eng := fpengine.New()
	fp, err := eng.Generate(*seed, *browser, *osName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Seed: %d\n", fp.Seed)
	fmt.Printf("Browser: %s %s\n", fp.Browser.Name, fp.Browser.Version)
	fmt.Printf("OS: %s %s (platform: %s)\n", fp.OS.Name, fp.OS.Version, fp.OS.Platform)
	fmt.Printf("GPU: %s\n", fp.GPU.Renderer)
	fmt.Printf("UA: %s\n", fp.Navigator["userAgent"])
	fmt.Printf("Timezone: %s\n", fp.Timezone)
	fmt.Printf("Languages: %v\n", fp.Languages)
	fmt.Printf("Screen: %v\n", fp.Screen)
	fmt.Printf("Window: %dx%d (DPR: %.1f)\n", fp.Window.InnerWidth, fp.Window.InnerHeight, fp.Window.DevicePixelRatio)
	fmt.Printf("Fonts: %d fonts\n", len(fp.Fonts))
	fmt.Printf("Canvas hash: %s\n", fp.Canvas.ToDataURLHash)
	fmt.Printf("Audio hash: %s\n", fp.Audio.Hash)
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	scriptFile := fs.String("script", "", "Script file to execute")
	evalCode := fs.String("eval", "", "Code to evaluate directly")
	browser := fs.String("browser", "chrome", "Browser type")
	osName := fs.String("os", "windows", "Operating system")
	location := fs.String("location", "https://example.com/", "document.URL")
	seed := fs.Uint64("seed", 0, "Fingerprint seed (0 = random)")
	fs.Parse(args)

	if *scriptFile == "" && *evalCode == "" {
		fmt.Fprintln(os.Stderr, "Error: --script or --eval required")
		os.Exit(1)
	}

	// Create fingerprint engine + sandbox engine
	fpEng := fpengine.New()
	eng := sandbox.New(fpEng, 4)
	defer eng.Dispose()

	// Create session
	sess, err := eng.CreateSession(api.SessionOptions{
		Seed:     *seed,
		Browser:  *browser,
		OS:       *osName,
		Location: *location,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Dispose()

	fmt.Printf("Session: %s\n", sess.ID)
	fmt.Printf("UA: %s\n", sess.GetFingerprint().Navigator["userAgent"])

	// Execute
	if *scriptFile != "" {
		content, err := os.ReadFile(*scriptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading script: %v\n", err)
			os.Exit(1)
		}
		// Use EvalAwait instead of LoadScript so the timer/microtask drain
		// loop runs — worker→parent messages delivered via scheduleTimer(0)
		// need the drain loop to execute their callbacks on the isolate thread.
		result, err := sess.EvalAwait(string(content), 30*time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Script error: %v\n", err)
			os.Exit(1)
		}
		if result != "" {
			fmt.Printf("Result: %s\n", result)
		}
		fmt.Println("Script loaded successfully")
	}

	if *evalCode != "" {
		result, err := sess.Eval(*evalCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Eval error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Result: %s\n", result)
	}

	// Flush timers + drain worker callbacks (fire-and-forget worker messages)
	sess.FlushTimers(5 * time.Second)
	sess.PerformMicrotasks()
	sess.DrainWorkerCallbacks(3 * time.Second)
}
