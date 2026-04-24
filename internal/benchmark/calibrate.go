package benchmark

import (
	"context"
	"fmt"

	"github.com/pinchtab/semantic"
)

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
		query  Query
		corpus *Corpus
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
			findResult, _ := matcher.Find(context.Background(), tc.query.QueryText, tc.corpus.Snapshot, semantic.FindOptions{
				Threshold: threshold,
				TopK:      5,
			})

			hasMatch := len(findResult.Matches) > 0
			topRef := ""
			if hasMatch {
				topRef = findResult.Matches[0].Ref
			}

			switch {
			case tc.query.ExpectNoMatch && hasMatch:
				fp++
			case tc.query.ExpectNoMatch && !hasMatch:
				tn++
			case len(tc.query.RelevantRefs) > 0 && !hasMatch:
				fn++
			case len(tc.query.RelevantRefs) > 0 && contains(tc.query.RelevantRefs, topRef):
				tp++
			case len(tc.query.RelevantRefs) > 0:
				fp++
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
