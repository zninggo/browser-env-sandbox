// Package main implements the bes CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/zninggo/bes/internal/fpengine"
	"github.com/zninggo/bes/internal/fpupdate"
	"github.com/zninggo/bes/internal/mcp"
	"github.com/zninggo/bes/internal/sandbox"
	"github.com/zninggo/bes/pkg/api"
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
	case "mcp":
		cmdMCP(os.Args[2:])
	case "update-fp":
		cmdUpdateFP(os.Args[2:])
	case "version":
		fmt.Println("bes v0.2.0 (V8 engine: v8go v0.9.0, V8 9.0)")
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `bes — browser-env-sandbox CLI

Usage:
  bes fingerprint [--browser chrome] [--os windows] [--seed 0]
  bes run --script <file> [--fingerprint auto] [--location <url>]
  bes export-fp --output <file> [--browser chrome] [--os windows] [--seed 0]
  bes selftest
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
		if err := sess.LoadScript(*scriptFile, string(content)); err != nil {
			fmt.Fprintf(os.Stderr, "Script error: %v\n", err)
			os.Exit(1)
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

	// Flush timers
	sess.FlushTimers(5 * time.Second)
	sess.PerformMicrotasks()
}
