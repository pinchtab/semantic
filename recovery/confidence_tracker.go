package recovery

import "sync"

const (
	defaultAdaptiveFallback = 0.4
	// confidenceThresholdHigh indicates high confidence matches (auto-accept)
	confidenceThresholdHigh = 0.8
	// confidenceThresholdMedium indicates medium confidence (may need verification)
	confidenceThresholdMedium = 0.6
	// confidenceThresholdLow indicates low confidence (likely needs recovery)
	confidenceThresholdLow = 0.4
	// decayFactor controls how quickly old samples lose influence (0.95 = 5% decay per update)
	decayFactor = 0.95
	// warmupPeriod is the minimum number of samples before adaptive threshold activates
	warmupPeriod = 15
)

// ConfidenceStats exposes adaptive-threshold diagnostics.
type ConfidenceStats struct {
	SuccessCount     int     `json:"success_count"`
	FailureCount     int     `json:"failure_count"`
	MeanSuccess      float64 `json:"mean_success"`
	MeanFailure      float64 `json:"mean_failure"`
	CurrentThreshold float64 `json:"current_threshold"`
	MinSamples       int     `json:"min_samples"`
	MaxSamples       int     `json:"max_samples"`
	Adaptive         bool    `json:"adaptive"`
}

// ConfidenceTracker tracks recovery outcomes and computes an adaptive threshold.
type ConfidenceTracker struct {
	mu         sync.Mutex
	successes  []float64
	failures   []float64
	maxSamples int
	minSamples int
}

// NewConfidenceTracker creates a tracker with bounded per-class sample buffers.
func NewConfidenceTracker(maxSamples, minSamples int) *ConfidenceTracker {
	if maxSamples <= 0 {
		maxSamples = 200
	}
	if minSamples <= 0 {
		minSamples = 20
	}
	return &ConfidenceTracker{
		maxSamples: maxSamples,
		minSamples: minSamples,
	}
}

// Record stores one recovery score outcome.
func (ct *ConfidenceTracker) Record(score float64, succeeded bool) {
	if ct == nil {
		return
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if succeeded {
		ct.successes = appendWithRing(ct.successes, score, ct.maxSamples)
		return
	}
	ct.failures = appendWithRing(ct.failures, score, ct.maxSamples)
}

// OptimalThreshold returns the adaptive threshold with a fixed fallback.
func (ct *ConfidenceTracker) OptimalThreshold() float64 {
	return ct.OptimalThresholdWithDefault(defaultAdaptiveFallback)
}

// OptimalThresholdWithDefault returns the adaptive threshold if there are enough
// success and failure samples; otherwise it returns defaultThreshold.
func (ct *ConfidenceTracker) OptimalThresholdWithDefault(defaultThreshold float64) float64 {
	if ct == nil {
		return defaultThreshold
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if len(ct.successes) < ct.minSamples || len(ct.failures) < ct.minSamples {
		return defaultThreshold
	}

	meanSuccess := mean(ct.successes)
	meanFailure := mean(ct.failures)
	threshold := (meanSuccess + meanFailure) / 2.0
	if threshold < 0 {
		return 0
	}
	if threshold > 1 {
		return 1
	}
	return threshold
}

// Stats returns tracker state for diagnostics.
func (ct *ConfidenceTracker) Stats(defaultThreshold float64) ConfidenceStats {
	if ct == nil {
		return ConfidenceStats{CurrentThreshold: defaultThreshold}
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	stats := ConfidenceStats{
		SuccessCount: len(ct.successes),
		FailureCount: len(ct.failures),
		MinSamples:   ct.minSamples,
		MaxSamples:   ct.maxSamples,
	}
	if len(ct.successes) > 0 {
		stats.MeanSuccess = mean(ct.successes)
	}
	if len(ct.failures) > 0 {
		stats.MeanFailure = mean(ct.failures)
	}
	if len(ct.successes) >= ct.minSamples && len(ct.failures) >= ct.minSamples {
		stats.Adaptive = true
		stats.CurrentThreshold = (stats.MeanSuccess + stats.MeanFailure) / 2.0
		if stats.CurrentThreshold < 0 {
			stats.CurrentThreshold = 0
		}
		if stats.CurrentThreshold > 1 {
			stats.CurrentThreshold = 1
		}
		return stats
	}
	stats.CurrentThreshold = defaultThreshold
	return stats
}

func appendWithRing(values []float64, v float64, max int) []float64 {
	values = append(values, v)
	if len(values) <= max {
		return values
	}
	start := len(values) - max
	trimmed := make([]float64, max)
	copy(trimmed, values[start:])
	return trimmed
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}
