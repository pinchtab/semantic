package benchmark

import "testing"

func TestValidateConfig_Valid(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"lexical", "embedding", "combined"},
		Defaults: DefaultsConfig{
			Strategy: "combined",
			Weights:  Weights{Lexical: 0.6, Embedding: 0.4},
		},
		Baseline: BaselineConfig{
			Quality: BaselineQuality{
				MaxOverallPAt1Drop: 0.02,
			},
			Runtime: BaselineRuntime{
				MaxNsOpRegressionRatio: 1.25,
			},
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_EmptyStrategies(t *testing.T) {
	cfg := &Config{
		Strategies: []string{},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0.6, Embedding: 0.4},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for empty strategies")
	}
}

func TestValidateConfig_InvalidDefaultStrategy(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"lexical", "embedding"},
		Defaults: DefaultsConfig{
			Strategy: "combined",
			Weights:  Weights{Lexical: 0.6, Embedding: 0.4},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid default strategy")
	}
}

func TestValidateConfig_NegativeWeights(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: -0.5, Embedding: 0.4},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for negative weight")
	}
}

func TestValidateConfig_BothWeightsZero(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0, Embedding: 0},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error when both weights are zero")
	}
}

func TestValidateConfig_RuntimeRatioTooLow(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0.6, Embedding: 0.4},
		},
		Baseline: BaselineConfig{
			Runtime: BaselineRuntime{
				MaxNsOpRegressionRatio: 0.5,
			},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for runtime ratio < 1")
	}
}

func TestValidateConfig_ProfileInheritsMissing(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0.6, Embedding: 0.4},
		},
		Profiles: map[string]Profile{
			"fast": {Inherits: "nonexistent"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing inherited profile")
	}
}

func TestValidateConfig_ProfileInheritanceCycle(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0.6, Embedding: 0.4},
		},
		Profiles: map[string]Profile{
			"a": {Inherits: "b"},
			"b": {Inherits: "c"},
			"c": {Inherits: "a"},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for inheritance cycle")
	}
}

func TestValidateConfig_NegativeQualityThreshold(t *testing.T) {
	cfg := &Config{
		Strategies: []string{"combined"},
		Defaults: DefaultsConfig{
			Weights: Weights{Lexical: 0.6, Embedding: 0.4},
		},
		Baseline: BaselineConfig{
			Quality: BaselineQuality{
				MaxOverallPAt1Drop: -0.02,
			},
		},
	}
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for negative quality threshold")
	}
}
