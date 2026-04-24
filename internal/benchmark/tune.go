package benchmark

import "fmt"

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
