package benchmark

import (
	"encoding/json"
	"fmt"
	"sort"
)

func RunCompare(cfg CompareConfig) (*CompareResult, error) {
	baseline, err := loadReport(cfg.BaselinePath)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}
	current, err := loadReport(cfg.CurrentPath)
	if err != nil {
		return nil, fmt.Errorf("load current: %w", err)
	}

	result := &CompareResult{
		Status: "pass",
		Delta: MetricsDelta{
			PAt1:   current.Metrics.Overall.PAt1 - baseline.Metrics.Overall.PAt1,
			MRR:    current.Metrics.Overall.MRR - baseline.Metrics.Overall.MRR,
			HitAt3: current.Metrics.Overall.HitAt3 - baseline.Metrics.Overall.HitAt3,
		},
	}

	if result.Delta.PAt1 < -0.02 || result.Delta.MRR < -0.02 {
		result.Status = "fail"
	}

	baselineResults := make(map[string]QueryResult)
	for _, r := range baseline.Results {
		baselineResults[r.ID] = r
	}
	for _, r := range current.Results {
		if base, ok := baselineResults[r.ID]; ok {
			if base.Status == "hit" && r.Status != "hit" {
				result.Regressions = append(result.Regressions, Regression{
					ID:          r.ID,
					Corpus:      r.Corpus,
					Query:       r.Query,
					BaselineRef: base.Actual.BestRef,
					CurrentRef:  r.Actual.BestRef,
					Reason:      fmt.Sprintf("%s -> %s", base.Status, r.Status),
				})
			}
		}
	}

	return result, nil
}

func PrintCompareResult(result *CompareResult, cfg CompareConfig) {
	if cfg.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n")
	if result.Status == "pass" {
		fmt.Printf("  \033[32m✓\033[0m No regression\n")
	} else {
		fmt.Printf("  \033[31m✗\033[0m Regression detected\n")
	}
	fmt.Printf("\n")
	printDelta("P@1", result.Delta.PAt1)
	printDelta("MRR", result.Delta.MRR)
	printDelta("Hit@3", result.Delta.HitAt3)

	if len(result.Regressions) > 0 {
		fmt.Printf("\n  Regressions:\n")
		sortRegressions(result.Regressions)
		for _, r := range result.Regressions {
			fmt.Printf("    %s: %s (%s)\n", r.ID, r.Reason, r.Query)
		}
	}
	fmt.Printf("\n")
}

func sortRegressions(regs []Regression) {
	sort.Slice(regs, func(i, j int) bool {
		if regs[i].Corpus != regs[j].Corpus {
			return regs[i].Corpus < regs[j].Corpus
		}
		return regs[i].ID < regs[j].ID
	})
}
