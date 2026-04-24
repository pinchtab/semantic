package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func RunCheck(cfg CheckConfig) (*CheckResult, error) {
	root := FindBenchmarkRoot()

	ds, err := LoadDataset(root)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	benchCfg, err := LoadConfig(root)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	profile := ResolveProfile(benchCfg, cfg.Profile)

	runCfg := RunConfig{
		Suite:           "corpus",
		Strategy:        profile.Strategy,
		Threshold:       profile.Threshold,
		TopK:            profile.TopK,
		LexicalWeight:   profile.Weights.Lexical,
		EmbeddingWeight: profile.Weights.Embedding,
		Profile:         cfg.Profile,
		Mode:            "library",
		Verbose:         cfg.Verbose,
		Explain:         cfg.Explain,
		OutputDir:       cfg.OutputDir,
		Quick:           cfg.Quick,
	}

	report, err := RunCorpusBenchmark(ds, runCfg)
	if err != nil {
		return nil, fmt.Errorf("run benchmark: %w", err)
	}

	result := &CheckResult{
		Status: "pass",
		Report: report,
	}
	result.Summary.PAt1 = report.Metrics.Overall.PAt1
	result.Summary.MRR = report.Metrics.Overall.MRR
	result.Summary.HitAt3 = report.Metrics.Overall.HitAt3
	result.Summary.Total = report.Metrics.Overall.Total

	for _, r := range report.Results {
		if r.Status == "miss" {
			result.TopRegs = append(result.TopRegs, Regression{
				ID:           r.ID,
				Corpus:       r.Corpus,
				Query:        r.Query,
				Expected:     r.Expected.RelevantRefs,
				CurrentRef:   r.Actual.BestRef,
				Reason:       "miss",
				DebugCommand: fmt.Sprintf("semantic-bench run --query %s --verbose --explain", r.ID),
			})
		}
	}
	result.Summary.Regressions = len(result.TopRegs)

	// Determine baseline path from config
	baselinePath := cfg.BaselinePath
	if baselinePath == "" {
		baselinePath = filepath.Join(benchCfg.BaselinesDir(root), "combined.json")
	}

	// Get quality thresholds from config
	thresholds := benchCfg.QualityThresholds()

	if _, err := os.Stat(baselinePath); err == nil {
		baseline, err := loadReport(baselinePath)
		if err == nil {
			result.Delta = &MetricsDelta{
				PAt1:   report.Metrics.Overall.PAt1 - baseline.Metrics.Overall.PAt1,
				MRR:    report.Metrics.Overall.MRR - baseline.Metrics.Overall.MRR,
				HitAt3: report.Metrics.Overall.HitAt3 - baseline.Metrics.Overall.HitAt3,
			}
			if cfg.FailOnReg {
				// Check overall thresholds
				if result.Delta.PAt1 < -thresholds.MaxOverallPAt1Drop ||
					result.Delta.MRR < -thresholds.MaxOverallMRRDrop ||
					result.Delta.HitAt3 < -thresholds.MaxOverallHitAt3Drop {
					result.Status = "fail"
				}
				// Check corpus-level thresholds
				for corpus, current := range report.Metrics.ByCorpus {
					if base, ok := baseline.Metrics.ByCorpus[corpus]; ok {
						if current.PAt1-base.PAt1 < -thresholds.MaxCorpusPAt1Drop {
							result.Status = "fail"
						}
					}
				}
				// Check difficulty-level thresholds
				for diff, current := range report.Metrics.ByDifficulty {
					if base, ok := baseline.Metrics.ByDifficulty[diff]; ok {
						if current.PAt1-base.PAt1 < -thresholds.MaxDifficultyPAt1Drop {
							result.Status = "fail"
						}
					}
				}
				// Check tag-level thresholds
				for tag, current := range report.Metrics.ByTag {
					if base, ok := baseline.Metrics.ByTag[tag]; ok {
						if current.PAt1-base.PAt1 < -thresholds.MaxTagPAt1Drop {
							result.Status = "fail"
						}
					}
				}
			}
		}
	}

	// Sort regressions for deterministic output
	sort.Slice(result.TopRegs, func(i, j int) bool {
		if result.TopRegs[i].Corpus != result.TopRegs[j].Corpus {
			return result.TopRegs[i].Corpus < result.TopRegs[j].Corpus
		}
		return result.TopRegs[i].ID < result.TopRegs[j].ID
	})

	_ = os.MkdirAll(cfg.OutputDir, 0755)
	ts := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("bench_%s.json", ts))
	summaryPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("bench_%s.md", ts))

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(reportPath, reportJSON, 0644)

	summaryMD := generateSummaryMD(report, result)
	_ = os.WriteFile(summaryPath, []byte(summaryMD), 0644)

	result.Artifacts.ReportJSON = reportPath
	result.Artifacts.SummaryMD = summaryPath

	return result, nil
}

func RunBenchmark(cfg RunConfig) (*Report, error) {
	root := FindBenchmarkRoot()
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, err
	}
	return RunCorpusBenchmark(ds, cfg)
}

func loadReport(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func generateSummaryMD(report *Report, result *CheckResult) string {
	var sb strings.Builder

	sb.WriteString("# Benchmark Summary\n\n")
	fmt.Fprintf(&sb, "Generated: %s\n\n", report.Run.Timestamp)

	sb.WriteString("## Overall Metrics\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	fmt.Fprintf(&sb, "| Total | %d |\n", report.Metrics.Overall.Total)
	fmt.Fprintf(&sb, "| MRR | %.4f |\n", report.Metrics.Overall.MRR)
	fmt.Fprintf(&sb, "| P@1 | %.4f |\n", report.Metrics.Overall.PAt1)
	fmt.Fprintf(&sb, "| Hit@3 | %.4f |\n", report.Metrics.Overall.HitAt3)
	fmt.Fprintf(&sb, "| Avg Margin | %.4f |\n", report.Metrics.Overall.AvgMargin)

	if result.Delta != nil {
		sb.WriteString("\n## Delta from Baseline\n\n")
		sb.WriteString("| Metric | Delta |\n")
		sb.WriteString("|--------|-------|\n")
		fmt.Fprintf(&sb, "| P@1 | %+.4f |\n", result.Delta.PAt1)
		fmt.Fprintf(&sb, "| MRR | %+.4f |\n", result.Delta.MRR)
		fmt.Fprintf(&sb, "| Hit@3 | %+.4f |\n", result.Delta.HitAt3)
	}

	if len(result.TopRegs) > 0 {
		sb.WriteString("\n## Misses\n\n")
		sb.WriteString("| ID | Corpus | Query | Got | Expected |\n")
		sb.WriteString("|----|--------|-------|-----|----------|\n")
		for i, r := range result.TopRegs {
			if i >= 10 {
				break
			}
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n",
				r.ID, r.Corpus, r.Query, r.CurrentRef, strings.Join(r.Expected, ","))
		}
		if len(result.TopRegs) > 10 {
			fmt.Fprintf(&sb, "\n*Showing 10 of %d misses.*\n", len(result.TopRegs))
		}
	}

	return sb.String()
}

func PrintCheckResult(result *CheckResult, cfg CheckConfig) {
	if cfg.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n")
	if result.Status == "pass" {
		fmt.Printf("  \033[32m✓\033[0m Benchmark passed\n")
	} else {
		fmt.Printf("  \033[31m✗\033[0m Benchmark failed\n")
	}
	fmt.Printf("\n")

	fmt.Printf("  %-12s %8.4f\n", "MRR", result.Summary.MRR)
	fmt.Printf("  %-12s %8.4f\n", "P@1", result.Summary.PAt1)
	fmt.Printf("  %-12s %8.4f\n", "Hit@3", result.Summary.HitAt3)
	fmt.Printf("  %-12s %8d\n", "Total", result.Summary.Total)
	fmt.Printf("  %-12s %8d\n", "Misses", result.Summary.Regressions)

	if result.Delta != nil {
		fmt.Printf("\n  Delta from baseline:\n")
		printDelta("P@1", result.Delta.PAt1)
		printDelta("MRR", result.Delta.MRR)
		printDelta("Hit@3", result.Delta.HitAt3)
	}

	fmt.Printf("\n  Artifacts:\n")
	fmt.Printf("    Report:  %s\n", result.Artifacts.ReportJSON)
	fmt.Printf("    Summary: %s\n", result.Artifacts.SummaryMD)
	fmt.Printf("\n")
}

func printDelta(name string, delta float64) {
	color := "\033[0m"
	sign := ""
	if delta > 0.001 {
		color = "\033[32m"
		sign = "+"
	} else if delta < -0.001 {
		color = "\033[31m"
	}
	fmt.Printf("    %s%-8s %s%.4f\033[0m\n", color, name, sign, delta)
}

func PrintRunResult(report *Report, cfg RunConfig) {
	fmt.Printf("\n")
	fmt.Printf("  %-12s %8.4f\n", "MRR", report.Metrics.Overall.MRR)
	fmt.Printf("  %-12s %8.4f\n", "P@1", report.Metrics.Overall.PAt1)
	fmt.Printf("  %-12s %8.4f\n", "Hit@3", report.Metrics.Overall.HitAt3)
	fmt.Printf("  %-12s %8d\n", "Total", report.Metrics.Overall.Total)
	fmt.Printf("\n")

	if cfg.Verbose {
		for _, r := range report.Results {
			status := "\033[32mHIT \033[0m"
			switch r.Status {
			case "miss":
				status = "\033[31mMISS\033[0m"
			case "partial":
				status = "\033[33mPART\033[0m"
			}
			fmt.Printf("  [%s] %s | %s | got=%s score=%.3f\n",
				r.ID, status, r.Query, r.Actual.BestRef, r.Actual.BestScore)
		}
	}
}
