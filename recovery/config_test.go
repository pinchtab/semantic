package recovery

import "testing"

func TestDefaultRecoveryConfig(t *testing.T) {
	cfg := DefaultRecoveryConfig()
	if !cfg.Enabled {
		t.Error("default should be enabled")
	}
	if cfg.MaxRetries != 1 {
		t.Errorf("MaxRetries = %d, want 1", cfg.MaxRetries)
	}
	if cfg.MinConfidence != defaultRecoveryMinConfidence {
		t.Errorf("MinConfidence = %f, want %f", cfg.MinConfidence, defaultRecoveryMinConfidence)
	}
	if cfg.PreferHighConfidence {
		t.Error("PreferHighConfidence should be false by default")
	}
}
