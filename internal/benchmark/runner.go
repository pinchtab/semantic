package benchmark

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/pinchtab/semantic"
)

type QueryResult struct {
	ID         string   `json:"id"`
	Corpus     string   `json:"corpus"`
	Query      string   `json:"query"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags"`
	Intent     string   `json:"intent,omitempty"`
	PageType   string   `json:"page_type,omitempty"`
	Expected   struct {
		RelevantRefs          []string `json:"relevant_refs"`
		PartiallyRelevantRefs []string `json:"partially_relevant_refs"`
	} `json:"expected"`
	Actual struct {
		BestRef   string  `json:"best_ref"`
		BestScore float64 `json:"best_score"`
		Matches   []Match `json:"matches"`
	} `json:"actual"`
	Metrics struct {
		RR                float64 `json:"rr"`
		PAt1              float64 `json:"p_at_1"`
		PAt3              float64 `json:"p_at_3"`
		HitAt3            int     `json:"hit_at_3"`
		HitAt5            int     `json:"hit_at_5"`
		BestRelevantRank  *int    `json:"best_relevant_rank"`
		BestRelevantScore float64 `json:"best_relevant_score"`
		BestWrongScore    float64 `json:"best_wrong_score"`
		Margin            float64 `json:"margin"`
	} `json:"metrics"`
	Latency struct {
		LibraryMs int64  `json:"library_ms"`
		CLIMs     *int64 `json:"cli_ms,omitempty"`
	} `json:"latency"`
	Status string `json:"status"`
}

type Match struct {
	Ref   string  `json:"ref"`
	Score float64 `json:"score"`
	Role  string  `json:"role"`
	Name  string  `json:"name"`
}

type Report struct {
	SchemaVersion string `json:"schema_version"`
	Run           struct {
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		Tool      string `json:"tool"`
		GitSHA    string `json:"git_sha,omitempty"`
		GitDirty  bool   `json:"git_dirty,omitempty"`
		Command   string `json:"command"`
	} `json:"run"`
	Dataset struct {
		Name        string `json:"name"`
		Version     string `json:"version,omitempty"`
		QueryCount  int    `json:"query_count"`
		CorpusCount int    `json:"corpus_count"`
	} `json:"dataset"`
	Config struct {
		Profile   string  `json:"profile"`
		Strategy  string  `json:"strategy"`
		Threshold float64 `json:"threshold"`
		TopK      int     `json:"top_k"`
		Weights   Weights `json:"weights"`
	} `json:"config"`
	Status  string `json:"status"`
	Metrics struct {
		Overall      OverallMetrics           `json:"overall"`
		Latency      LatencyMetrics           `json:"latency"`
		ByCorpus     map[string]CorpusMetrics `json:"by_corpus"`
		ByDifficulty map[string]CorpusMetrics `json:"by_difficulty"`
		ByTag        map[string]CorpusMetrics `json:"by_tag"`
	} `json:"metrics"`
	Results []QueryResult `json:"results"`
}

type OverallMetrics struct {
	Total     int     `json:"total"`
	MRR       float64 `json:"mrr"`
	PAt1      float64 `json:"p_at_1"`
	PAt3      float64 `json:"p_at_3"`
	HitAt3    float64 `json:"hit_at_3"`
	HitAt5    float64 `json:"hit_at_5"`
	AvgMargin float64 `json:"avg_margin"`
}

type LatencyMetrics struct {
	LibraryP50Ms int64  `json:"library_p50_ms"`
	LibraryP95Ms int64  `json:"library_p95_ms"`
	CLIP50Ms     *int64 `json:"cli_p50_ms,omitempty"`
	CLIP95Ms     *int64 `json:"cli_p95_ms,omitempty"`
}

type CorpusMetrics struct {
	Count     int     `json:"count"`
	MRR       float64 `json:"mrr"`
	PAt1      float64 `json:"p_at_1"`
	HitAt3    float64 `json:"hit_at_3"`
	AvgMargin float64 `json:"avg_margin"`
}

func RunCorpusBenchmark(ds *Dataset, cfg RunConfig) (*Report, error) {
	matcher := createMatcher(cfg)

	report := &Report{
		SchemaVersion: "1.0.0",
		Status:        "pass",
	}
	report.Run.ID = time.Now().Format("20060102-150405") + "-" + cfg.Profile
	report.Run.Timestamp = time.Now().UTC().Format(time.RFC3339)
	report.Run.Tool = "semantic-bench"
	report.Run.GitSHA, report.Run.GitDirty = getGitInfo()
	report.Dataset.Name = "semantic-ui-matching-corpus"
	report.Dataset.QueryCount = ds.QueryCount()
	report.Dataset.CorpusCount = ds.CorpusCount()
	report.Config.Profile = cfg.Profile
	report.Config.Strategy = cfg.Strategy
	report.Config.Threshold = cfg.Threshold
	report.Config.TopK = cfg.TopK
	report.Config.Weights = Weights{Lexical: cfg.LexicalWeight, Embedding: cfg.EmbeddingWeight}

	report.Metrics.ByCorpus = make(map[string]CorpusMetrics)
	report.Metrics.ByDifficulty = make(map[string]CorpusMetrics)
	report.Metrics.ByTag = make(map[string]CorpusMetrics)

	var allLatencies []int64

	for _, corpus := range ds.Corpora {
		if cfg.Corpus != "" && corpus.ID != cfg.Corpus {
			continue
		}

		queries := corpus.Queries
		if cfg.Quick {
			queries = selectQuickSubset(corpus.Queries)
		}

		for _, query := range queries {
			if cfg.QueryID != "" && query.ID != cfg.QueryID {
				continue
			}

			result := runQuery(matcher, corpus, query, cfg)
			report.Results = append(report.Results, result)
			allLatencies = append(allLatencies, result.Latency.LibraryMs)
		}
	}

	aggregateMetrics(report, allLatencies)
	return report, nil
}

// selectQuickSubset returns a deterministic subset for smoke testing.
// Selects up to 3 queries per corpus by difficulty. This is NOT representative
// of full corpus coverage—edge-case tags may be missed. Use for fast iteration,
// not for final regression checks.
func selectQuickSubset(queries []Query) []Query {
	if len(queries) <= 3 {
		return queries
	}

	// Group by difficulty
	byDiff := make(map[string][]Query)
	for _, q := range queries {
		diff := q.Difficulty
		if diff == "" {
			diff = "medium"
		}
		byDiff[diff] = append(byDiff[diff], q)
	}

	// Select one from each difficulty level, up to 3 total
	var selected []Query
	for _, diff := range []string{"easy", "medium", "hard"} {
		if qs, ok := byDiff[diff]; ok && len(qs) > 0 {
			selected = append(selected, qs[0])
			if len(selected) >= 3 {
				break
			}
		}
	}

	// If we don't have 3 yet, fill from remaining
	if len(selected) < 3 {
		for _, q := range queries {
			found := false
			for _, s := range selected {
				if s.ID == q.ID {
					found = true
					break
				}
			}
			if !found {
				selected = append(selected, q)
				if len(selected) >= 3 {
					break
				}
			}
		}
	}

	return selected
}

func createMatcher(cfg RunConfig) semantic.ElementMatcher {
	embedder := semantic.NewHashingEmbedder(128)
	switch cfg.Strategy {
	case "lexical":
		return semantic.NewLexicalMatcher()
	case "embedding":
		return semantic.NewEmbeddingMatcher(embedder)
	default:
		return semantic.NewCombinedMatcher(embedder)
	}
}

func runQuery(matcher semantic.ElementMatcher, corpus Corpus, query Query, cfg RunConfig) QueryResult {
	result := QueryResult{
		ID:         query.ID,
		Corpus:     corpus.ID,
		Query:      query.QueryText,
		Difficulty: query.Difficulty,
		Tags:       query.Tags,
		Intent:     query.Intent,
		PageType:   query.PageType,
	}
	result.Expected.RelevantRefs = query.RelevantRefs
	result.Expected.PartiallyRelevantRefs = query.PartiallyRelevantRefs

	threshold := cfg.Threshold
	if query.Threshold != nil {
		threshold = *query.Threshold
	}
	topK := cfg.TopK
	if query.TopK != nil {
		topK = *query.TopK
	}

	start := time.Now()
	findResult, _ := matcher.Find(context.Background(), query.QueryText, corpus.Snapshot, semantic.FindOptions{
		Threshold:       threshold,
		TopK:            topK,
		LexicalWeight:   cfg.LexicalWeight,
		EmbeddingWeight: cfg.EmbeddingWeight,
		Explain:         cfg.Explain,
	})
	result.Latency.LibraryMs = time.Since(start).Milliseconds()

	result.Actual.BestRef = findResult.BestRef
	result.Actual.BestScore = findResult.BestScore
	for _, m := range findResult.Matches {
		result.Actual.Matches = append(result.Actual.Matches, Match{
			Ref:   m.Ref,
			Score: m.Score,
			Role:  m.Role,
			Name:  m.Name,
		})
	}

	computeQueryMetrics(&result, query)
	return result
}

func computeQueryMetrics(result *QueryResult, query Query) {
	relevantSet := make(map[string]bool)
	for _, r := range query.RelevantRefs {
		relevantSet[r] = true
	}
	partialSet := make(map[string]bool)
	for _, r := range query.PartiallyRelevantRefs {
		partialSet[r] = true
	}

	// Reciprocal Rank
	for i, m := range result.Actual.Matches {
		if relevantSet[m.Ref] {
			result.Metrics.RR = 1.0 / float64(i+1)
			break
		}
	}

	// P@1
	if len(result.Actual.Matches) > 0 {
		if relevantSet[result.Actual.Matches[0].Ref] {
			result.Metrics.PAt1 = 1.0
		} else if partialSet[result.Actual.Matches[0].Ref] {
			result.Metrics.PAt1 = 0.5
		}
	}

	// P@3, Hit@3, Hit@5
	relevantInTop3 := 0
	partialInTop3 := 0
	for i, m := range result.Actual.Matches {
		if i >= 5 {
			break
		}
		switch {
		case relevantSet[m.Ref]:
			if result.Metrics.BestRelevantRank == nil {
				rank := i + 1
				result.Metrics.BestRelevantRank = &rank
			}
			if result.Metrics.BestRelevantScore == 0 || m.Score > result.Metrics.BestRelevantScore {
				result.Metrics.BestRelevantScore = m.Score
			}
			if i < 3 {
				relevantInTop3++
				result.Metrics.HitAt3 = 1
			}
			result.Metrics.HitAt5 = 1
		case partialSet[m.Ref]:
			if i < 3 {
				partialInTop3++
			}
		default:
			if m.Score > result.Metrics.BestWrongScore {
				result.Metrics.BestWrongScore = m.Score
			}
		}
	}
	result.Metrics.PAt3 = (float64(relevantInTop3) + float64(partialInTop3)*0.5) / 3.0
	result.Metrics.Margin = result.Metrics.BestRelevantScore - result.Metrics.BestWrongScore

	// Status
	switch {
	case query.ExpectNoMatch:
		if len(result.Actual.Matches) == 0 {
			result.Status = "no_match_expected"
		} else {
			result.Status = "unexpected_match"
		}
	case result.Metrics.PAt1 >= 1.0:
		result.Status = "hit"
	case result.Metrics.PAt1 >= 0.5:
		result.Status = "partial"
	default:
		result.Status = "miss"
	}
}

func aggregateMetrics(report *Report, latencies []int64) {
	n := len(report.Results)
	if n == 0 {
		return
	}

	report.Metrics.Overall.Total = n

	var sumRR, sumP1, sumP3, sumHit3, sumHit5, sumMargin float64
	corpusAgg := make(map[string]*aggregator)
	diffAgg := make(map[string]*aggregator)
	tagAgg := make(map[string]*aggregator)

	for _, r := range report.Results {
		sumRR += r.Metrics.RR
		sumP1 += r.Metrics.PAt1
		sumP3 += r.Metrics.PAt3
		sumHit3 += float64(r.Metrics.HitAt3)
		sumHit5 += float64(r.Metrics.HitAt5)
		sumMargin += r.Metrics.Margin

		addToAgg(corpusAgg, r.Corpus, r)
		addToAgg(diffAgg, r.Difficulty, r)
		for _, t := range r.Tags {
			addToAgg(tagAgg, t, r)
		}
	}

	report.Metrics.Overall.MRR = sumRR / float64(n)
	report.Metrics.Overall.PAt1 = sumP1 / float64(n)
	report.Metrics.Overall.PAt3 = sumP3 / float64(n)
	report.Metrics.Overall.HitAt3 = sumHit3 / float64(n)
	report.Metrics.Overall.HitAt5 = sumHit5 / float64(n)
	report.Metrics.Overall.AvgMargin = sumMargin / float64(n)

	for k, a := range corpusAgg {
		report.Metrics.ByCorpus[k] = a.toMetrics()
	}
	for k, a := range diffAgg {
		report.Metrics.ByDifficulty[k] = a.toMetrics()
	}
	for k, a := range tagAgg {
		report.Metrics.ByTag[k] = a.toMetrics()
	}

	// Latency percentiles
	if len(latencies) > 0 {
		sorted := make([]int64, len(latencies))
		copy(sorted, latencies)
		sortInt64(sorted)
		report.Metrics.Latency.LibraryP50Ms = sorted[len(sorted)*50/100]
		report.Metrics.Latency.LibraryP95Ms = sorted[len(sorted)*95/100]
	}
}

type aggregator struct {
	count     int
	sumRR     float64
	sumP1     float64
	sumHit3   float64
	sumMargin float64
}

func addToAgg(m map[string]*aggregator, key string, r QueryResult) {
	if _, ok := m[key]; !ok {
		m[key] = &aggregator{}
	}
	a := m[key]
	a.count++
	a.sumRR += r.Metrics.RR
	a.sumP1 += r.Metrics.PAt1
	a.sumHit3 += float64(r.Metrics.HitAt3)
	a.sumMargin += r.Metrics.Margin
}

func (a *aggregator) toMetrics() CorpusMetrics {
	if a.count == 0 {
		return CorpusMetrics{}
	}
	return CorpusMetrics{
		Count:     a.count,
		MRR:       a.sumRR / float64(a.count),
		PAt1:      a.sumP1 / float64(a.count),
		HitAt3:    a.sumHit3 / float64(a.count),
		AvgMargin: a.sumMargin / float64(a.count),
	}
}

func sortInt64(s []int64) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func getGitInfo() (sha string, dirty bool) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	sha = strings.TrimSpace(string(out))

	cmd = exec.Command("git", "status", "--porcelain")
	out, err = cmd.Output()
	if err != nil {
		return sha, false
	}
	dirty = len(strings.TrimSpace(string(out))) > 0
	return sha, dirty
}
