package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BaselineResult struct {
	Action   string          `json:"action"`
	Path     string          `json:"path"`
	Metrics  OverallMetrics  `json:"metrics"`
	Previous *OverallMetrics `json:"previous,omitempty"`
}

func RunBaseline(cfg BaselineCmdConfig) (*BaselineResult, error) {
	root := FindBenchmarkRoot()
	baselinesDir := filepath.Join(root, "baselines")
	if err := os.MkdirAll(baselinesDir, 0755); err != nil {
		return nil, err
	}

	baselinePath := filepath.Join(baselinesDir, cfg.Name+".json")

	switch cfg.Action {
	case "create":
		return createBaseline(root, baselinePath, cfg)
	case "update":
		if !cfg.Accept {
			return nil, fmt.Errorf("use --accept to confirm baseline update")
		}
		return updateBaseline(root, baselinePath, cfg)
	default:
		return nil, fmt.Errorf("unknown baseline action: %s (use 'create' or 'update')", cfg.Action)
	}
}

func createBaseline(root, baselinePath string, cfg BaselineCmdConfig) (*BaselineResult, error) {
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	runCfg := RunConfig{
		Suite:           "corpus",
		Strategy:        "combined",
		Threshold:       0.01,
		TopK:            5,
		LexicalWeight:   0.6,
		EmbeddingWeight: 0.4,
		Mode:            "library",
	}

	report, err := RunCorpusBenchmark(ds, runCfg)
	if err != nil {
		return nil, fmt.Errorf("run benchmark: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(baselinePath, data, 0644); err != nil {
		return nil, err
	}

	return &BaselineResult{
		Action:  "create",
		Path:    baselinePath,
		Metrics: report.Metrics.Overall,
	}, nil
}

func updateBaseline(root, baselinePath string, cfg BaselineCmdConfig) (*BaselineResult, error) {
	var previous *OverallMetrics
	if data, err := os.ReadFile(baselinePath); err == nil {
		var old Report
		if json.Unmarshal(data, &old) == nil {
			previous = &old.Metrics.Overall
		}
		backupPath := strings.TrimSuffix(baselinePath, ".json") + "_" + time.Now().Format("20060102_150405") + ".backup.json"
		_ = os.WriteFile(backupPath, data, 0644)
	}

	result, err := createBaseline(root, baselinePath, cfg)
	if err != nil {
		return nil, err
	}
	result.Action = "update"
	result.Previous = previous
	return result, nil
}

func PrintBaselineResult(result *BaselineResult, cfg BaselineCmdConfig) {
	fmt.Printf("\n  Baseline %sd: %s\n\n", result.Action, result.Path)
	fmt.Printf("  MRR:    %.4f\n", result.Metrics.MRR)
	fmt.Printf("  P@1:    %.4f\n", result.Metrics.PAt1)
	fmt.Printf("  Hit@3:  %.4f\n", result.Metrics.HitAt3)

	if result.Previous != nil {
		fmt.Printf("\n  Previous:\n")
		fmt.Printf("    MRR:    %.4f\n", result.Previous.MRR)
		fmt.Printf("    P@1:    %.4f\n", result.Previous.PAt1)
		fmt.Printf("    Hit@3:  %.4f\n", result.Previous.HitAt3)
	}
	fmt.Println()
}
