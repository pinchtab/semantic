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
  check     Run benchmark and compare against baseline (default)
  run       Run benchmark suites
  compare   Compare two reports
  lint      Validate dataset
  catalog   Print dataset inventory

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
