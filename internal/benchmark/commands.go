package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pinchtab/semantic"
)

type CheckResult struct {
	Status    string        `json:"status"`
	Summary   CheckSummary  `json:"summary"`
	Delta     *MetricsDelta `json:"delta,omitempty"`
	TopRegs   []Regression  `json:"top_regressions,omitempty"`
	Artifacts Artifacts     `json:"artifacts"`
	Report    *Report       `json:"-"`
}

type CheckSummary struct {
	PAt1        float64 `json:"p_at_1"`
	MRR         float64 `json:"mrr"`
	HitAt3      float64 `json:"hit_at_3"`
	Total       int     `json:"total"`
	Regressions int     `json:"regressions"`
	Warnings    int     `json:"warnings"`
}

type MetricsDelta struct {
	PAt1   float64 `json:"p_at_1"`
	MRR    float64 `json:"mrr"`
	HitAt3 float64 `json:"hit_at_3"`
}

type Regression struct {
	ID           string   `json:"id"`
	Corpus       string   `json:"corpus"`
	Query        string   `json:"query"`
	Expected     []string `json:"expected"`
	BaselineRef  string   `json:"baseline_ref,omitempty"`
	CurrentRef   string   `json:"current_ref"`
	Reason       string   `json:"reason"`
	DebugCommand string   `json:"debug_command"`
}

type Artifacts struct {
	ReportJSON string `json:"report_json"`
	SummaryMD  string `json:"summary_md"`
}

type CompareResult struct {
	Status       string       `json:"status"`
	Delta        MetricsDelta `json:"delta"`
	Regressions  []Regression `json:"regressions"`
	Improvements []string     `json:"improvements"`
}

type LintResult struct {
	Errors   int      `json:"errors"`
	Warnings int      `json:"warnings"`
	Messages []string `json:"messages"`
}

type CatalogResult struct {
	Corpora      []CorpusSummary `json:"corpora"`
	TotalQueries int             `json:"total_queries"`
	ByTag        map[string]int  `json:"by_tag,omitempty"`
	ByDifficulty map[string]int  `json:"by_difficulty,omitempty"`
}

type CorpusSummary struct {
	ID      string   `json:"id"`
	Queries int      `json:"queries"`
	Tags    []string `json:"tags"`
}

func RunCheck(cfg CheckConfig) (*CheckResult, error) {
	root := FindBenchmarkRoot()

	ds, err := LoadDataset(root)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	benchCfg, _ := LoadConfig(root)
	profile := Profile{
		Strategy:  "combined",
		Threshold: 0.01,
		TopK:      5,
		Weights:   Weights{Lexical: 0.6, Embedding: 0.4},
	}
	if benchCfg != nil {
		profile = ResolveProfile(benchCfg, cfg.Profile)
	}

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

	// Count misses
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

	// Compare to baseline if exists
	baselinePath := cfg.BaselinePath
	if baselinePath == "" {
		baselinePath = filepath.Join(root, "baselines", "combined.json")
	}
	if _, err := os.Stat(baselinePath); err == nil {
		baseline, err := loadReport(baselinePath)
		if err == nil {
			result.Delta = &MetricsDelta{
				PAt1:   report.Metrics.Overall.PAt1 - baseline.Metrics.Overall.PAt1,
				MRR:    report.Metrics.Overall.MRR - baseline.Metrics.Overall.MRR,
				HitAt3: report.Metrics.Overall.HitAt3 - baseline.Metrics.Overall.HitAt3,
			}
			if cfg.FailOnReg && (result.Delta.PAt1 < -0.02 || result.Delta.MRR < -0.02) {
				result.Status = "fail"
			}
		}
	}

	// Write artifacts
	os.MkdirAll(cfg.OutputDir, 0755)
	ts := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("bench_%s.json", ts))
	summaryPath := filepath.Join(cfg.OutputDir, fmt.Sprintf("bench_%s.md", ts))

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(reportPath, reportJSON, 0644)

	summaryMD := generateSummaryMD(report, result)
	os.WriteFile(summaryPath, []byte(summaryMD), 0644)

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

	// Find regressions
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

func RunLint(cfg LintConfig) (*LintResult, error) {
	root := FindBenchmarkRoot()
	result := &LintResult{}

	ds, err := LoadDataset(root)
	if err != nil {
		result.Errors++
		result.Messages = append(result.Messages, fmt.Sprintf("ERROR: failed to load dataset: %v", err))
		return result, nil
	}

	// Check for duplicate IDs
	ids := make(map[string]string)
	for _, c := range ds.Corpora {
		for _, q := range c.Queries {
			if existing, ok := ids[q.ID]; ok {
				result.Errors++
				result.Messages = append(result.Messages,
					fmt.Sprintf("ERROR: duplicate ID '%s' in %s (first seen in %s)", q.ID, c.ID, existing))
			} else {
				ids[q.ID] = c.ID
			}
		}
	}

	// Check refs exist
	for _, c := range ds.Corpora {
		refs := make(map[string]bool)
		for _, d := range c.Snapshot {
			refs[d.Ref] = true
		}
		for _, q := range c.Queries {
			for _, r := range q.RelevantRefs {
				if !refs[r] {
					result.Errors++
					result.Messages = append(result.Messages,
						fmt.Sprintf("ERROR: [%s] relevant_ref '%s' not found in snapshot", q.ID, r))
				}
			}
		}
	}

	// Check difficulty values
	validDiff := map[string]bool{"easy": true, "medium": true, "hard": true}
	for _, c := range ds.Corpora {
		for _, q := range c.Queries {
			if q.Difficulty != "" && !validDiff[q.Difficulty] {
				result.Errors++
				result.Messages = append(result.Messages,
					fmt.Sprintf("ERROR: invalid difficulty '%s' for query '%s'", q.Difficulty, q.ID))
			}
		}
	}

	if result.Errors == 0 && result.Warnings == 0 {
		result.Messages = append(result.Messages, "All checks passed")
	}

	return result, nil
}

func RunCatalog(cfg CatalogConfig) (*CatalogResult, error) {
	root := FindBenchmarkRoot()
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, err
	}

	result := &CatalogResult{
		ByTag:        make(map[string]int),
		ByDifficulty: make(map[string]int),
	}

	for _, c := range ds.Corpora {
		tags := make(map[string]bool)
		for _, q := range c.Queries {
			result.TotalQueries++
			result.ByDifficulty[q.Difficulty]++
			for _, t := range q.Tags {
				tags[t] = true
				result.ByTag[t]++
			}
		}
		var tagList []string
		for t := range tags {
			tagList = append(tagList, t)
		}
		sort.Strings(tagList)
		result.Corpora = append(result.Corpora, CorpusSummary{
			ID:      c.ID,
			Queries: len(c.Queries),
			Tags:    tagList,
		})
	}

	return result, nil
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
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", report.Run.Timestamp))

	sb.WriteString("## Overall Metrics\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total | %d |\n", report.Metrics.Overall.Total))
	sb.WriteString(fmt.Sprintf("| MRR | %.4f |\n", report.Metrics.Overall.MRR))
	sb.WriteString(fmt.Sprintf("| P@1 | %.4f |\n", report.Metrics.Overall.PAt1))
	sb.WriteString(fmt.Sprintf("| Hit@3 | %.4f |\n", report.Metrics.Overall.HitAt3))
	sb.WriteString(fmt.Sprintf("| Avg Margin | %.4f |\n", report.Metrics.Overall.AvgMargin))

	if result.Delta != nil {
		sb.WriteString("\n## Delta from Baseline\n\n")
		sb.WriteString("| Metric | Delta |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| P@1 | %+.4f |\n", result.Delta.PAt1))
		sb.WriteString(fmt.Sprintf("| MRR | %+.4f |\n", result.Delta.MRR))
		sb.WriteString(fmt.Sprintf("| Hit@3 | %+.4f |\n", result.Delta.HitAt3))
	}

	if len(result.TopRegs) > 0 {
		sb.WriteString("\n## Misses\n\n")
		sb.WriteString("| ID | Corpus | Query | Got | Expected |\n")
		sb.WriteString("|----|--------|-------|-----|----------|\n")
		for _, r := range result.TopRegs {
			if len(result.TopRegs) > 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				r.ID, r.Corpus, r.Query, r.CurrentRef, strings.Join(r.Expected, ",")))
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

func PrintCompareResult(result *CompareResult, cfg CompareConfig) {
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
		for _, r := range result.Regressions {
			fmt.Printf("    %s: %s (%s)\n", r.ID, r.Reason, r.Query)
		}
	}
	fmt.Printf("\n")
}

func PrintLintResult(result *LintResult, cfg LintConfig) {
	for _, msg := range result.Messages {
		fmt.Println(msg)
	}
	fmt.Printf("\nErrors: %d, Warnings: %d\n", result.Errors, result.Warnings)
}

func PrintCatalogResult(result *CatalogResult, cfg CatalogConfig) {
	if cfg.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n  Corpora: %d\n", len(result.Corpora))
	fmt.Printf("  Total Queries: %d\n\n", result.TotalQueries)

	fmt.Printf("  %-30s %8s\n", "Corpus", "Queries")
	fmt.Printf("  %-30s %8s\n", "------", "-------")
	for _, c := range result.Corpora {
		fmt.Printf("  %-30s %8d\n", c.ID, c.Queries)
	}

	switch cfg.By {
	case "difficulty":
		fmt.Printf("\n  By Difficulty:\n")
		for d, n := range result.ByDifficulty {
			fmt.Printf("    %-10s %4d\n", d, n)
		}
	case "tag":
		fmt.Printf("\n  By Tag:\n")
		for t, n := range result.ByTag {
			fmt.Printf("    %-20s %4d\n", t, n)
		}
	}
	fmt.Printf("\n")
}

// Baseline management

type BaselineResult struct {
	Action   string         `json:"action"`
	Path     string         `json:"path"`
	Metrics  OverallMetrics `json:"metrics"`
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
		os.WriteFile(backupPath, data, 0644)
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

// Threshold calibration

type CalibrateResult struct {
	ByThreshold     map[string]ThresholdMetrics `json:"by_threshold"`
	Recommendations CalibrateRecommendations    `json:"recommendations"`
	TotalCases      int                         `json:"total_cases"`
}

type ThresholdMetrics struct {
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	TN        int     `json:"tn"`
	Recall    float64 `json:"recall"`
	Precision float64 `json:"precision"`
	FPR       float64 `json:"false_positive_rate"`
	F1        float64 `json:"f1"`
}

type CalibrateRecommendations struct {
	DefaultThreshold  float64 `json:"default_threshold"`
	RecoveryThreshold float64 `json:"recovery_threshold"`
	BestF1            float64 `json:"best_f1"`
}

func RunCalibrate(cfg CalibrateConfig) (*CalibrateResult, error) {
	root := FindBenchmarkRoot()
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	result := &CalibrateResult{
		ByThreshold: make(map[string]ThresholdMetrics),
	}

	type testCase struct {
		query         Query
		corpus        *Corpus
	}

	var cases []testCase
	for i := range ds.Corpora {
		corpus := &ds.Corpora[i]
		if cfg.Corpus != "" && corpus.ID != cfg.Corpus {
			continue
		}
		for _, q := range corpus.Queries {
			cases = append(cases, testCase{query: q, corpus: corpus})
		}
	}
	result.TotalCases = len(cases)

	if cfg.Verbose {
		fmt.Printf("Testing %d thresholds against %d cases...\n\n", len(cfg.Thresholds), len(cases))
	}

	runCfg := RunConfig{
		Strategy:        "combined",
		TopK:            5,
		LexicalWeight:   0.6,
		EmbeddingWeight: 0.4,
	}
	matcher := createMatcher(runCfg)

	var bestF1, bestF1Threshold float64
	var bestRecallThreshold float64
	var bestRecallWithPrecision float64

	for _, threshold := range cfg.Thresholds {
		tp, fp, fn, tn := 0, 0, 0, 0

		for _, tc := range cases {
			findResult, _ := matcher.Find(nil, tc.query.QueryText, tc.corpus.Snapshot, semantic.FindOptions{
				Threshold: threshold,
				TopK:      5,
			})

			hasMatch := len(findResult.Matches) > 0
			topRef := ""
			if hasMatch {
				topRef = findResult.Matches[0].Ref
			}

			if tc.query.ExpectNoMatch {
				if hasMatch {
					fp++
				} else {
					tn++
				}
			} else if len(tc.query.RelevantRefs) > 0 {
				if !hasMatch {
					fn++
				} else if contains(tc.query.RelevantRefs, topRef) {
					tp++
				} else {
					fp++
				}
			}
		}

		totalPos := tp + fn
		totalNeg := tn + fp

		var recall, precision, fpr, f1 float64
		if totalPos > 0 {
			recall = float64(tp) / float64(totalPos)
		}
		if tp+fp > 0 {
			precision = float64(tp) / float64(tp+fp)
		}
		if totalNeg > 0 {
			fpr = float64(fp) / float64(totalNeg)
		}
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}

		key := fmt.Sprintf("%.2f", threshold)
		result.ByThreshold[key] = ThresholdMetrics{
			TP: tp, FP: fp, FN: fn, TN: tn,
			Recall: recall, Precision: precision, FPR: fpr, F1: f1,
		}

		if f1 > bestF1 {
			bestF1 = f1
			bestF1Threshold = threshold
		}
		if recall >= 0.85 && precision > bestRecallWithPrecision {
			bestRecallWithPrecision = precision
			bestRecallThreshold = threshold
		}

		if cfg.Verbose {
			fmt.Printf("  threshold=%.2f | TP=%3d FP=%3d FN=%3d TN=%3d | recall=%.3f precision=%.3f F1=%.3f\n",
				threshold, tp, fp, fn, tn, recall, precision, f1)
		}
	}

	if bestRecallThreshold == 0 && len(cfg.Thresholds) > 0 {
		bestRecallThreshold = cfg.Thresholds[0]
	}

	result.Recommendations = CalibrateRecommendations{
		DefaultThreshold:  bestF1Threshold,
		RecoveryThreshold: bestRecallThreshold,
		BestF1:            bestF1,
	}

	return result, nil
}

func contains(refs []string, ref string) bool {
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}

func PrintCalibrateResult(result *CalibrateResult, cfg CalibrateConfig) {
	fmt.Printf("\n  Tested %d cases across %d thresholds\n\n", result.TotalCases, len(result.ByThreshold))

	fmt.Printf("  Recommendations:\n")
	fmt.Printf("    Default (best F1):   %.2f (F1=%.3f)\n", result.Recommendations.DefaultThreshold, result.Recommendations.BestF1)
	fmt.Printf("    Recovery (recall):   %.2f\n", result.Recommendations.RecoveryThreshold)
	fmt.Println()
}

// Weight tuning

type TuneResult struct {
	Results []TuneRun `json:"results"`
	Best    *TuneRun  `json:"best"`
}

type TuneRun struct {
	LexicalWeight   float64 `json:"lexical_weight"`
	EmbeddingWeight float64 `json:"embedding_weight"`
	MRR             float64 `json:"mrr"`
	PAt1            float64 `json:"p_at_1"`
	HitAt3          float64 `json:"hit_at_3"`
}

func RunTune(cfg TuneConfig) (*TuneResult, error) {
	root := FindBenchmarkRoot()
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, fmt.Errorf("load dataset: %w", err)
	}

	result := &TuneResult{}

	if cfg.Verbose {
		fmt.Printf("  %-10s %-10s %-8s %-8s %-8s\n", "lexical", "embedding", "MRR", "P@1", "Hit@3")
	}

	for w := 0.0; w <= 1.0001; w += cfg.Step {
		lexW := w
		embW := 1.0 - w

		runCfg := RunConfig{
			Suite:           "corpus",
			Strategy:        "combined",
			Threshold:       0.01,
			TopK:            5,
			LexicalWeight:   lexW,
			EmbeddingWeight: embW,
			Mode:            "library",
		}

		if cfg.Corpus != "" {
			runCfg.Corpus = cfg.Corpus
		}

		report, err := RunCorpusBenchmark(ds, runCfg)
		if err != nil {
			return nil, fmt.Errorf("run at lexical=%.2f: %w", lexW, err)
		}

		run := TuneRun{
			LexicalWeight:   lexW,
			EmbeddingWeight: embW,
			MRR:             report.Metrics.Overall.MRR,
			PAt1:            report.Metrics.Overall.PAt1,
			HitAt3:          report.Metrics.Overall.HitAt3,
		}
		result.Results = append(result.Results, run)

		if result.Best == nil || run.PAt1 > result.Best.PAt1 ||
			(run.PAt1 == result.Best.PAt1 && run.MRR > result.Best.MRR) {
			best := run
			result.Best = &best
		}

		if cfg.Verbose {
			fmt.Printf("  %-10.2f %-10.2f %-8.4f %-8.4f %-8.4f\n",
				lexW, embW, run.MRR, run.PAt1, run.HitAt3)
		}
	}

	return result, nil
}

func PrintTuneResult(result *TuneResult, cfg TuneConfig) {
	fmt.Printf("\n  Tested %d weight combinations\n\n", len(result.Results))

	if result.Best != nil {
		fmt.Printf("  Best weights:\n")
		fmt.Printf("    Lexical:   %.2f\n", result.Best.LexicalWeight)
		fmt.Printf("    Embedding: %.2f\n", result.Best.EmbeddingWeight)
		fmt.Printf("    MRR:       %.4f\n", result.Best.MRR)
		fmt.Printf("    P@1:       %.4f\n", result.Best.PAt1)
		fmt.Printf("    Hit@3:     %.4f\n", result.Best.HitAt3)
	}
	fmt.Println()
}
