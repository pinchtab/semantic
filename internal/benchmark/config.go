package benchmark

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Version      string             `json:"version"`
	Defaults     DefaultsConfig     `json:"defaults"`
	Profiles     map[string]Profile `json:"profiles"`
	Baseline     BaselineConfig     `json:"baseline"`
	Results      ResultsConfig      `json:"results"`
	Strategies   []string           `json:"strategies"`
	SnapshotsDir string             `json:"snapshots_dir"`
}

type DefaultsConfig struct {
	Profile   string  `json:"profile"`
	Strategy  string  `json:"strategy"`
	Threshold float64 `json:"threshold"`
	TopK      int     `json:"top_k"`
	Weights   Weights `json:"weights"`
}

type ResultsConfig struct {
	Dir                  string `json:"dir"`
	BaselinesDir         string `json:"baselines_dir"`
	GeneratedFilesPolicy string `json:"generated_files_policy"`
}

type Profile struct {
	Strategy  string   `json:"strategy"`
	Threshold float64  `json:"threshold"`
	TopK      int      `json:"top_k"`
	Weights   Weights  `json:"weights"`
	Suites    []string `json:"suites"`
	Mode      string   `json:"mode"`
	Inherits  string   `json:"inherits"`
	Verbose   bool     `json:"verbose"`
	Explain   bool     `json:"explain"`
	FailOnReg bool     `json:"fail_on_regression"`
}

type Weights struct {
	Lexical   float64 `json:"lexical"`
	Embedding float64 `json:"embedding"`
}

type BaselineConfig struct {
	Quality BaselineQuality `json:"quality"`
	Runtime BaselineRuntime `json:"runtime"`
}

type BaselineQuality struct {
	MaxOverallPAt1Drop    float64 `json:"max_overall_p_at_1_drop"`
	MaxOverallMRRDrop     float64 `json:"max_overall_mrr_drop"`
	MaxOverallHitAt3Drop  float64 `json:"max_overall_hit_at_3_drop"`
	MaxCorpusPAt1Drop     float64 `json:"max_corpus_p_at_1_drop"`
	MaxDifficultyPAt1Drop float64 `json:"max_difficulty_p_at_1_drop"`
	MaxTagPAt1Drop        float64 `json:"max_tag_p_at_1_drop"`
	MaxMarginDropReport   float64 `json:"max_margin_drop_report"`
}

type BaselineRuntime struct {
	MaxNsOpRegressionRatio  float64 `json:"max_ns_op_regression_ratio"`
	MaxAllocRegressionRatio float64 `json:"max_alloc_regression_ratio"`
	MaxCorpusLatencyP50MS   int     `json:"max_corpus_latency_p50_ms"`
	MaxCorpusLatencyP95MS   int     `json:"max_corpus_latency_p95_ms"`
}

type CheckConfig struct {
	Profile      string
	BaselinePath string
	OutputDir    string
	Format       string
	FailOnReg    bool
	Quick        bool
	Verbose      bool
	Explain      bool
}

type RunConfig struct {
	Suite           string
	Corpus          string
	QueryID         string
	Strategy        string
	Threshold       float64
	TopK            int
	LexicalWeight   float64
	EmbeddingWeight float64
	Profile         string
	Mode            string
	Verbose         bool
	Explain         bool
	OutputDir       string
	ReportName      string
	Quick           bool
}

type CompareConfig struct {
	BaselinePath string
	CurrentPath  string
	Format       string
	Verbose      bool
}

type LintConfig struct {
	Format  string
	Verbose bool
}

type CatalogConfig struct {
	Format string
	By     string
}

type BaselineCmdConfig struct {
	Action  string // "create" or "update"
	Name    string
	Accept  bool
	Verbose bool
}

type CalibrateConfig struct {
	Corpus     string
	Thresholds []float64
	Verbose    bool
}

type TuneConfig struct {
	Corpus  string
	Step    float64
	Verbose bool
}

type RuntimeConfig struct {
	FailOnRegression bool
	Verbose          bool
}

func FindBenchmarkRoot() string {
	cwd, _ := os.Getwd()
	for d := cwd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "tests/benchmark/config/benchmark.json")); err == nil {
			return filepath.Join(d, "tests/benchmark")
		}
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "tests/benchmark")
		}
	}
	return filepath.Join(cwd, "tests/benchmark")
}

func LoadConfig(benchmarkRoot string) (*Config, error) {
	path := filepath.Join(benchmarkRoot, "config/benchmark.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ResolveProfile(cfg *Config, name string) Profile {
	p, ok := cfg.Profiles[name]
	if !ok {
		// Use defaults from config, falling back to hardcoded values
		strategy := cfg.Defaults.Strategy
		if strategy == "" {
			strategy = "combined"
		}
		threshold := cfg.Defaults.Threshold
		if threshold == 0 {
			threshold = 0.01
		}
		topK := cfg.Defaults.TopK
		if topK == 0 {
			topK = 5
		}
		weights := cfg.Defaults.Weights
		if weights.Lexical == 0 && weights.Embedding == 0 {
			weights = Weights{Lexical: 0.6, Embedding: 0.4}
		}
		return Profile{
			Strategy:  strategy,
			Threshold: threshold,
			TopK:      topK,
			Weights:   weights,
			Suites:    []string{"corpus"},
			Mode:      "library",
		}
	}
	if p.Inherits != "" {
		base := ResolveProfile(cfg, p.Inherits)
		if p.Strategy == "" {
			p.Strategy = base.Strategy
		}
		if p.Threshold == 0 {
			p.Threshold = base.Threshold
		}
		if p.TopK == 0 {
			p.TopK = base.TopK
		}
		if p.Weights.Lexical == 0 && p.Weights.Embedding == 0 {
			p.Weights = base.Weights
		}
		if len(p.Suites) == 0 {
			p.Suites = base.Suites
		}
		if p.Mode == "" {
			p.Mode = base.Mode
		}
	}
	return p
}

// projectRoot returns the project root (parent of tests/benchmark).
func projectRoot(benchmarkRoot string) string {
	return filepath.Dir(filepath.Dir(benchmarkRoot))
}

// ResultsDir returns the configured results directory.
func (c *Config) ResultsDir(benchmarkRoot string) string {
	if c.Results.Dir != "" {
		if filepath.IsAbs(c.Results.Dir) {
			return c.Results.Dir
		}
		return filepath.Join(projectRoot(benchmarkRoot), c.Results.Dir)
	}
	return filepath.Join(benchmarkRoot, "results")
}

// BaselinesDir returns the configured baselines directory.
func (c *Config) BaselinesDir(benchmarkRoot string) string {
	if c.Results.BaselinesDir != "" {
		if filepath.IsAbs(c.Results.BaselinesDir) {
			return c.Results.BaselinesDir
		}
		return filepath.Join(projectRoot(benchmarkRoot), c.Results.BaselinesDir)
	}
	return filepath.Join(benchmarkRoot, "baselines")
}

// QualityThresholds returns quality thresholds with fallback defaults.
func (c *Config) QualityThresholds() BaselineQuality {
	q := c.Baseline.Quality
	if q.MaxOverallPAt1Drop == 0 {
		q.MaxOverallPAt1Drop = 0.02
	}
	if q.MaxOverallMRRDrop == 0 {
		q.MaxOverallMRRDrop = 0.02
	}
	if q.MaxOverallHitAt3Drop == 0 {
		q.MaxOverallHitAt3Drop = 0.02
	}
	if q.MaxCorpusPAt1Drop == 0 {
		q.MaxCorpusPAt1Drop = 0.08
	}
	if q.MaxDifficultyPAt1Drop == 0 {
		q.MaxDifficultyPAt1Drop = 0.08
	}
	if q.MaxTagPAt1Drop == 0 {
		q.MaxTagPAt1Drop = 0.08
	}
	if q.MaxMarginDropReport == 0 {
		q.MaxMarginDropReport = 0.15
	}
	return q
}

// RuntimeThresholds returns runtime thresholds with fallback defaults.
func (c *Config) RuntimeThresholds() BaselineRuntime {
	r := c.Baseline.Runtime
	if r.MaxNsOpRegressionRatio == 0 {
		r.MaxNsOpRegressionRatio = 1.25
	}
	if r.MaxAllocRegressionRatio == 0 {
		r.MaxAllocRegressionRatio = 1.25
	}
	return r
}

// ValidateConfig checks the config for errors and returns a descriptive error if invalid.
func ValidateConfig(cfg *Config) error {
	var errs []error

	// Validate strategies
	if len(cfg.Strategies) == 0 {
		errs = append(errs, errors.New("strategies list is empty"))
	} else {
		validStrategies := make(map[string]bool)
		for _, s := range cfg.Strategies {
			validStrategies[s] = true
		}
		// Check default strategy is in list
		if cfg.Defaults.Strategy != "" && !validStrategies[cfg.Defaults.Strategy] {
			errs = append(errs, fmt.Errorf("default strategy %q not in strategies list", cfg.Defaults.Strategy))
		}
		// Check profile strategies
		for name, p := range cfg.Profiles {
			if p.Strategy != "" && !validStrategies[p.Strategy] {
				errs = append(errs, fmt.Errorf("profile %q uses strategy %q not in strategies list", name, p.Strategy))
			}
		}
	}

	// Validate weights
	if cfg.Defaults.Weights.Lexical < 0 {
		errs = append(errs, errors.New("defaults.weights.lexical must be non-negative"))
	}
	if cfg.Defaults.Weights.Embedding < 0 {
		errs = append(errs, errors.New("defaults.weights.embedding must be non-negative"))
	}
	if cfg.Defaults.Weights.Lexical == 0 && cfg.Defaults.Weights.Embedding == 0 {
		errs = append(errs, errors.New("defaults.weights: lexical and embedding cannot both be zero"))
	}

	// Validate profile weights
	for name, p := range cfg.Profiles {
		if p.Weights.Lexical < 0 {
			errs = append(errs, fmt.Errorf("profile %q: weights.lexical must be non-negative", name))
		}
		if p.Weights.Embedding < 0 {
			errs = append(errs, fmt.Errorf("profile %q: weights.embedding must be non-negative", name))
		}
	}

	// Validate quality thresholds (should be positive when set)
	q := cfg.Baseline.Quality
	if q.MaxOverallPAt1Drop < 0 {
		errs = append(errs, errors.New("baseline.quality.max_overall_p_at_1_drop must be non-negative"))
	}
	if q.MaxOverallMRRDrop < 0 {
		errs = append(errs, errors.New("baseline.quality.max_overall_mrr_drop must be non-negative"))
	}
	if q.MaxOverallHitAt3Drop < 0 {
		errs = append(errs, errors.New("baseline.quality.max_overall_hit_at_3_drop must be non-negative"))
	}

	// Validate runtime thresholds (must be >= 1)
	r := cfg.Baseline.Runtime
	if r.MaxNsOpRegressionRatio != 0 && r.MaxNsOpRegressionRatio < 1 {
		errs = append(errs, errors.New("baseline.runtime.max_ns_op_regression_ratio must be >= 1"))
	}
	if r.MaxAllocRegressionRatio != 0 && r.MaxAllocRegressionRatio < 1 {
		errs = append(errs, errors.New("baseline.runtime.max_alloc_regression_ratio must be >= 1"))
	}

	// Validate profile inheritance
	if err := validateProfileInheritance(cfg); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("config has %d errors: %v", len(errs), errs)
}

// validateProfileInheritance checks for missing references and cycles.
func validateProfileInheritance(cfg *Config) error {
	for name, p := range cfg.Profiles {
		if p.Inherits == "" {
			continue
		}
		// Check reference exists
		if _, ok := cfg.Profiles[p.Inherits]; !ok {
			return fmt.Errorf("profile %q inherits from non-existent profile %q", name, p.Inherits)
		}
		// Check for cycles
		visited := map[string]bool{name: true}
		current := p.Inherits
		for current != "" {
			if visited[current] {
				return fmt.Errorf("profile inheritance cycle detected: %q -> %q", name, current)
			}
			visited[current] = true
			if parent, ok := cfg.Profiles[current]; ok {
				current = parent.Inherits
			} else {
				break
			}
		}
	}
	return nil
}

func ParseCheckFlags(args []string) CheckConfig {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	cfg := CheckConfig{
		Profile:   "default",
		OutputDir: filepath.Join(FindBenchmarkRoot(), "results"),
		Format:    "text",
	}
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "benchmark profile")
	fs.StringVar(&cfg.BaselinePath, "baseline", "", "baseline file path")
	fs.StringVar(&cfg.OutputDir, "out", cfg.OutputDir, "output directory")
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format (text|json|github)")
	fs.BoolVar(&cfg.FailOnReg, "fail-on-regression", false, "exit 1 on regression")
	fs.BoolVar(&cfg.Quick, "quick", false, "run subset for fast checks")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "print per-corpus details")
	fs.BoolVar(&cfg.Explain, "explain", false, "include matcher explanations")
	_ = fs.Parse(args)
	return cfg
}

func ParseRunFlags(args []string) RunConfig {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfg := RunConfig{
		Suite:           "corpus",
		Strategy:        "combined",
		Threshold:       0.01,
		TopK:            5,
		LexicalWeight:   0.6,
		EmbeddingWeight: 0.4,
		Profile:         "default",
		Mode:            "library",
		OutputDir:       filepath.Join(FindBenchmarkRoot(), "results"),
	}
	fs.StringVar(&cfg.Suite, "suite", cfg.Suite, "suite to run (corpus|recovery|classification|runtime|all)")
	fs.StringVar(&cfg.Corpus, "corpus", "", "specific corpus to run")
	fs.StringVar(&cfg.QueryID, "query", "", "specific query ID to run")
	fs.StringVar(&cfg.Strategy, "strategy", cfg.Strategy, "matching strategy")
	fs.Float64Var(&cfg.Threshold, "threshold", cfg.Threshold, "score threshold")
	fs.IntVar(&cfg.TopK, "top-k", cfg.TopK, "number of results")
	fs.Float64Var(&cfg.LexicalWeight, "lexical-weight", cfg.LexicalWeight, "lexical weight")
	fs.Float64Var(&cfg.EmbeddingWeight, "embedding-weight", cfg.EmbeddingWeight, "embedding weight")
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "benchmark profile")
	fs.StringVar(&cfg.Mode, "mode", cfg.Mode, "execution mode (cli|library|both)")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cfg.Explain, "explain", false, "include explanations")
	fs.StringVar(&cfg.OutputDir, "out", cfg.OutputDir, "output directory")
	fs.StringVar(&cfg.ReportName, "report-name", "", "custom report name")
	_ = fs.Parse(args)
	return cfg
}

func ParseCompareFlags(args []string) CompareConfig {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	cfg := CompareConfig{
		Format: "text",
	}
	fs.StringVar(&cfg.BaselinePath, "baseline", "", "baseline report path (required)")
	fs.StringVar(&cfg.CurrentPath, "current", "", "current report path (required)")
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)
	return cfg
}

func ParseLintFlags(args []string) LintConfig {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	cfg := LintConfig{
		Format: "text",
	}
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)
	return cfg
}

func ParseCatalogFlags(args []string) CatalogConfig {
	fs := flag.NewFlagSet("catalog", flag.ExitOnError)
	cfg := CatalogConfig{
		Format: "table",
	}
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format (table|json)")
	fs.StringVar(&cfg.By, "by", "", "group by (tag|difficulty|intent)")
	_ = fs.Parse(args)
	return cfg
}

func ParseBaselineFlags(args []string) BaselineCmdConfig {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)
	cfg := BaselineCmdConfig{
		Action: "create",
		Name:   "combined",
	}
	fs.StringVar(&cfg.Name, "name", cfg.Name, "baseline name")
	fs.BoolVar(&cfg.Accept, "accept", false, "accept changes (for update)")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)

	if len(fs.Args()) > 0 {
		cfg.Action = fs.Args()[0]
	}
	return cfg
}

func ParseCalibrateFlags(args []string) CalibrateConfig {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	cfg := CalibrateConfig{
		Thresholds: []float64{0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.35, 0.40, 0.45, 0.50, 0.55, 0.60},
	}
	fs.StringVar(&cfg.Corpus, "corpus", "", "specific corpus to test")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)
	return cfg
}

func ParseTuneFlags(args []string) TuneConfig {
	fs := flag.NewFlagSet("tune", flag.ExitOnError)
	cfg := TuneConfig{
		Step: 0.1,
	}
	fs.StringVar(&cfg.Corpus, "corpus", "", "specific corpus to tune against")
	fs.Float64Var(&cfg.Step, "step", cfg.Step, "weight step size (0.05, 0.1, 0.2)")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)
	return cfg
}

func ParseRuntimeFlags(args []string) RuntimeConfig {
	fs := flag.NewFlagSet("runtime", flag.ExitOnError)
	cfg := RuntimeConfig{}
	fs.BoolVar(&cfg.FailOnRegression, "fail-on-regression", false, "exit 1 on regression")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	_ = fs.Parse(args)
	return cfg
}
