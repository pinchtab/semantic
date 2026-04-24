package main

import (
	"fmt"
	"os"

	"github.com/pinchtab/semantic/internal/benchmark"
)

const usage = `semantic-bench - Benchmark runner for semantic matching

Usage:
  semantic-bench <command> [flags]

Commands:
  check       Run benchmark and compare against baseline (default)
  run         Run benchmark suites
  compare     Compare two reports
  lint        Validate dataset
  catalog     Print dataset inventory
  baseline    Manage quality baselines (create, update)
  calibrate   Find optimal thresholds via precision/recall analysis
  tune        Grid-search lexical/embedding weights
  runtime     Check Go benchmark performance against baseline

Flags:
  -h, --help    Show help

Run 'semantic-bench <command> --help' for command-specific help.
`

func main() {
	if len(os.Args) < 2 {
		runCheck(os.Args[1:])
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "check":
		runCheck(args)
	case "run":
		runRun(args)
	case "compare":
		runCompare(args)
	case "lint":
		runLint(args)
	case "catalog":
		runCatalog(args)
	case "baseline":
		runBaseline(args)
	case "calibrate":
		runCalibrate(args)
	case "tune":
		runTune(args)
	case "runtime":
		runRuntime(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

func runCheck(args []string) {
	cfg := benchmark.ParseCheckFlags(args)
	result, err := benchmark.RunCheck(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintCheckResult(result, cfg)
	if result.Status == "fail" {
		os.Exit(1)
	}
}

func runRun(args []string) {
	cfg := benchmark.ParseRunFlags(args)
	result, err := benchmark.RunBenchmark(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintRunResult(result, cfg)
}

func runCompare(args []string) {
	cfg := benchmark.ParseCompareFlags(args)
	result, err := benchmark.RunCompare(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintCompareResult(result, cfg)
	if result.Status == "fail" {
		os.Exit(1)
	}
}

func runLint(args []string) {
	cfg := benchmark.ParseLintFlags(args)
	result, err := benchmark.RunLint(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintLintResult(result, cfg)
	if result.Errors > 0 {
		os.Exit(1)
	}
}

func runCatalog(args []string) {
	cfg := benchmark.ParseCatalogFlags(args)
	result, err := benchmark.RunCatalog(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintCatalogResult(result, cfg)
}

func runBaseline(args []string) {
	cfg := benchmark.ParseBaselineFlags(args)
	result, err := benchmark.RunBaseline(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintBaselineResult(result, cfg)
}

func runCalibrate(args []string) {
	cfg := benchmark.ParseCalibrateFlags(args)
	result, err := benchmark.RunCalibrate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintCalibrateResult(result, cfg)
}

func runTune(args []string) {
	cfg := benchmark.ParseTuneFlags(args)
	result, err := benchmark.RunTune(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintTuneResult(result, cfg)
}

func runRuntime(args []string) {
	cfg := benchmark.ParseRuntimeFlags(args)
	result, err := benchmark.RunRuntime(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	benchmark.PrintRuntimeResult(result, cfg)
	if result.Status == "fail" && cfg.FailOnRegression {
		os.Exit(1)
	}
}
