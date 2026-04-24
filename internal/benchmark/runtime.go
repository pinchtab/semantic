package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RuntimeResult struct {
	Status       string             `json:"status"`
	Benchmarks   []RuntimeBenchmark `json:"benchmarks"`
	Regressions  int                `json:"regressions"`
	BaselinePath string             `json:"baseline_path"`
	Created      bool               `json:"created"`
}

type RuntimeBenchmark struct {
	Name       string  `json:"name"`
	NsOp       float64 `json:"ns_op"`
	BytesOp    int     `json:"bytes_op"`
	AllocsOp   int     `json:"allocs_op"`
	BaselineNs float64 `json:"baseline_ns,omitempty"`
	Ratio      float64 `json:"ratio,omitempty"`
	Status     string  `json:"status"`
}

type runtimeBaseline struct {
	Timestamp  string             `json:"timestamp"`
	Benchmarks []RuntimeBenchmark `json:"benchmarks"`
}

func RunRuntime(cfg RuntimeConfig) (*RuntimeResult, error) {
	root := FindBenchmarkRoot()
	baselinePath := filepath.Join(root, "baselines", "runtime.json")

	benchmarks, err := runGoBenchmarks()
	if err != nil {
		return nil, err
	}

	result := &RuntimeResult{
		Status:       "pass",
		Benchmarks:   benchmarks,
		BaselinePath: baselinePath,
	}

	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		if err := saveRuntimeBaseline(baselinePath, benchmarks); err != nil {
			return nil, err
		}
		result.Created = true
		return result, nil
	}

	baseline, err := loadRuntimeBaseline(baselinePath)
	if err != nil {
		return nil, err
	}

	baselineMap := make(map[string]RuntimeBenchmark)
	for _, b := range baseline.Benchmarks {
		baselineMap[b.Name] = b
	}

	maxRatio := 1.25
	for i, b := range result.Benchmarks {
		if base, ok := baselineMap[b.Name]; ok {
			ratio := b.NsOp / base.NsOp
			result.Benchmarks[i].BaselineNs = base.NsOp
			result.Benchmarks[i].Ratio = ratio

			switch {
			case ratio > maxRatio:
				result.Benchmarks[i].Status = "regression"
				result.Regressions++
			case ratio > 1.1:
				result.Benchmarks[i].Status = "warning"
			default:
				result.Benchmarks[i].Status = "ok"
			}
		} else {
			result.Benchmarks[i].Status = "new"
		}
	}

	if result.Regressions > 0 {
		result.Status = "fail"
	}

	return result, nil
}

func runGoBenchmarks() ([]RuntimeBenchmark, error) {
	root := FindBenchmarkRoot()
	projectRoot := filepath.Join(root, "..", "..")

	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "./internal/engine/...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go test failed: %w\n%s", err, output)
	}

	return parseBenchOutput(string(output)), nil
}

func parseBenchOutput(output string) []RuntimeBenchmark {
	var results []RuntimeBenchmark
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := strings.TrimSuffix(fields[0], "-8")
		name = strings.TrimSuffix(name, "-10")
		name = strings.TrimSuffix(name, "-12")
		name = strings.TrimSuffix(name, "-16")

		var nsOp float64
		var bytesOp, allocsOp int

		for i, f := range fields {
			if f == "ns/op" && i > 0 {
				_, _ = fmt.Sscanf(fields[i-1], "%f", &nsOp)
			}
			if f == "B/op" && i > 0 {
				_, _ = fmt.Sscanf(fields[i-1], "%d", &bytesOp)
			}
			if f == "allocs/op" && i > 0 {
				_, _ = fmt.Sscanf(fields[i-1], "%d", &allocsOp)
			}
		}

		if nsOp > 0 {
			results = append(results, RuntimeBenchmark{
				Name:     name,
				NsOp:     nsOp,
				BytesOp:  bytesOp,
				AllocsOp: allocsOp,
			})
		}
	}

	return results
}

func saveRuntimeBaseline(path string, benchmarks []RuntimeBenchmark) error {
	baseline := runtimeBaseline{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Benchmarks: benchmarks,
	}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadRuntimeBaseline(path string) (*runtimeBaseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baseline runtimeBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}
	return &baseline, nil
}

func PrintRuntimeResult(result *RuntimeResult, cfg RuntimeConfig) {
	if result.Created {
		fmt.Printf("\n  Created runtime baseline: %s\n", result.BaselinePath)
		fmt.Printf("  Benchmarks: %d\n\n", len(result.Benchmarks))
		return
	}

	fmt.Printf("\n  Runtime Baseline Check\n\n")

	for _, b := range result.Benchmarks {
		var status string
		switch b.Status {
		case "regression":
			status = "\033[31mREGRESSION\033[0m"
		case "warning":
			status = "\033[33mWARNING\033[0m"
		case "ok":
			status = "\033[32mOK\033[0m"
		case "new":
			status = "\033[33mNEW\033[0m"
		}

		if b.BaselineNs > 0 {
			fmt.Printf("  %-10s %s: %.0f -> %.0f ns/op (%.2fx)\n",
				status, b.Name, b.BaselineNs, b.NsOp, b.Ratio)
		} else {
			fmt.Printf("  %-10s %s: %.0f ns/op\n", status, b.Name, b.NsOp)
		}
	}

	fmt.Println()
	if result.Regressions > 0 {
		fmt.Printf("  \033[31mRegressions: %d\033[0m\n\n", result.Regressions)
	} else {
		fmt.Printf("  \033[32mNo regressions\033[0m\n\n")
	}
}
