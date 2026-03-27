package recovery

import (
	"math"
	"testing"
)

func TestConfidenceTracker_DefaultThresholdWhenInsufficientSamples(t *testing.T) {
	ct := NewConfidenceTracker(10, 3)
	ct.Record(0.9, true)
	ct.Record(0.2, false)

	got := ct.OptimalThresholdWithDefault(0.4)
	if got != 0.4 {
		t.Fatalf("expected default threshold 0.4 with insufficient samples, got %f", got)
	}
}

func TestConfidenceTracker_AdaptsAfterMinSamples(t *testing.T) {
	ct := NewConfidenceTracker(20, 2)
	ct.Record(0.8, true)
	ct.Record(0.9, true)
	ct.Record(0.2, false)
	ct.Record(0.3, false)

	got := ct.OptimalThresholdWithDefault(0.4)
	want := (0.85 + 0.25) / 2.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("adaptive threshold mismatch: got %f want %f", got, want)
	}
}

func TestConfidenceTracker_RingBufferCapsSamples(t *testing.T) {
	ct := NewConfidenceTracker(3, 1)
	for i := 1; i <= 5; i++ {
		ct.Record(float64(i)/10.0, true)
		ct.Record(float64(i)/20.0, false)
	}

	stats := ct.Stats(0.4)
	if stats.SuccessCount != 3 {
		t.Fatalf("expected 3 success samples retained, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 3 {
		t.Fatalf("expected 3 failure samples retained, got %d", stats.FailureCount)
	}
}
