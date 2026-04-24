package benchmark

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
)

type Config struct {
	Version  string             `json:"version"`
	Defaults DefaultsConfig     `json:"defaults"`
	Profiles map[string]Profile `json:"profiles"`
	Baseline BaselineConfig     `json:"baseline"`
}

type DefaultsConfig struct {
	Profile string `json:"profile"`
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
	MaxOverallPAt1Drop   float64 `json:"max_overall_p_at_1_drop"`
	MaxOverallMRRDrop    float64 `json:"max_overall_mrr_drop"`
	MaxOverallHitAt3Drop float64 `json:"max_overall_hit_at_3_drop"`
	MaxCorpusPAt1Drop    float64 `json:"max_corpus_p_at_1_drop"`
	MaxTagPAt1Drop       float64 `json:"max_tag_p_at_1_drop"`
}

type BaselineRuntime struct {
	MaxNsOpRegressionRatio  float64 `json:"max_ns_op_regression_ratio"`
	MaxAllocRegressionRatio float64 `json:"max_alloc_regression_ratio"`
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
		return Profile{
			Strategy:  "combined",
			Threshold: 0.01,
			TopK:      5,
			Weights:   Weights{Lexical: 0.6, Embedding: 0.4},
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
