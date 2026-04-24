package benchmark

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
