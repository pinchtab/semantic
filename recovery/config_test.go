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
	if cfg.MinConfidence != 0.4 {
		t.Errorf("MinConfidence = %f, want 0.4", cfg.MinConfidence)
	}
	if cfg.PreferHighConfidence {
		t.Error("PreferHighConfidence should be false by default")
	}
}
