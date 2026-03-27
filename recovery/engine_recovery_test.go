package recovery

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/semantic"
)

type mockMatcher struct {
	findFn func(ctx context.Context, query string, descs []semantic.ElementDescriptor, opts semantic.FindOptions) (semantic.FindResult, error)
}

func (m *mockMatcher) Find(ctx context.Context, query string, descs []semantic.ElementDescriptor, opts semantic.FindOptions) (semantic.FindResult, error) {
	if m.findFn != nil {
		return m.findFn(ctx, query, descs, opts)
	}
	return semantic.FindResult{}, fmt.Errorf("mockMatcher: Find not configured")
}

func (m *mockMatcher) Strategy() string { return "mock" }

func TestRecoveryEngine_ShouldAttempt(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}

	if re.ShouldAttempt(nil, "e1") {
		t.Error("ShouldAttempt(nil, e1) = true, want false")
	}
	if re.ShouldAttempt(fmt.Errorf("could not find node"), "") {
		t.Error("ShouldAttempt(err, '') = true, want false (empty ref)")
	}
	if re.ShouldAttempt(fmt.Errorf("could not find node"), "e1") != true {
		t.Error("ShouldAttempt(notFound, e1) = false, want true")
	}
	if re.ShouldAttempt(fmt.Errorf("stale element"), "e1") != true {
		t.Error("ShouldAttempt(stale, e1) = false, want true")
	}
	if re.ShouldAttempt(fmt.Errorf("websocket connection closed"), "e1") {
		t.Error("ShouldAttempt(network, e1) = true, want false (not recoverable)")
	}
	if re.ShouldAttempt(fmt.Errorf("random error xyz"), "e1") {
		t.Error("ShouldAttempt(unknown, e1) = true, want false")
	}
}

func TestRecoveryEngine_ShouldAttempt_Disabled(t *testing.T) {
	cfg := DefaultRecoveryConfig()
	cfg.Enabled = false
	re := &RecoveryEngine{Config: cfg}

	if re.ShouldAttempt(fmt.Errorf("could not find node"), "e1") {
		t.Error("ShouldAttempt should return false when disabled")
	}
}

func TestRecoveryEngine_Attempt_NoCachedIntent(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		&mockMatcher{},
		cache,
		nil, nil, nil,
	)

	rr, res, err := re.Attempt(context.Background(), "tab1", "e99", "click", nil)
	if err == nil {
		t.Error("expected error when no intent cached")
	}
	if rr.Recovered {
		t.Error("Recovered should be false")
	}
	if res != nil {
		t.Error("result should be nil")
	}
	if !strings.Contains(rr.Error, "no cached intent") {
		t.Errorf("Error = %q, want to contain 'no cached intent'", rr.Error)
	}
}

func TestRecoveryEngine_Attempt_Success(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e5", IntentEntry{
		Query:      "submit button",
		Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Submit"},
	})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, descs []semantic.ElementDescriptor, opts semantic.FindOptions) (semantic.FindResult, error) {
			if query != "submit button" {
				return semantic.FindResult{}, fmt.Errorf("unexpected query: %s", query)
			}
			return semantic.FindResult{
				BestRef:   "e12",
				BestScore: 0.88,
				Matches: []semantic.ElementMatch{
					{Ref: "e12", Role: "button", Name: "Submit Form", Score: 0.88},
				},
				Strategy:     "combined",
				ElementCount: 3,
			}, nil
		},
	}

	refreshCalled := false
	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(ctx context.Context, tabID string) error {
			refreshCalled = true
			return nil
		},
		func(tabID, ref string) (int64, bool) {
			if ref == "e12" {
				return 42, true
			}
			return 0, false
		},
		func(tabID string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{
				{Ref: "e10", Role: "link", Name: "Home"},
				{Ref: "e12", Role: "button", Name: "Submit Form"},
				{Ref: "e14", Role: "textbox", Name: "Email"},
			}
		},
	)

	actionCalled := false
	rr, res, err := re.Attempt(context.Background(), "tab1", "e5", "click",
		func(ctx context.Context, kind string, nodeID int64) (map[string]any, error) {
			actionCalled = true
			if kind != "click" {
				return nil, fmt.Errorf("wrong kind: %s", kind)
			}
			if nodeID != 42 {
				return nil, fmt.Errorf("wrong nodeID: %d", nodeID)
			}
			return map[string]any{"clicked": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rr.Recovered {
		t.Error("Recovered should be true")
	}
	if !refreshCalled {
		t.Error("snapshot refresh was not called")
	}
	if !actionCalled {
		t.Error("action executor was not called")
	}
	if rr.NewRef != "e12" {
		t.Errorf("NewRef = %q, want %q", rr.NewRef, "e12")
	}
	if rr.Score != 0.88 {
		t.Errorf("Score = %f, want 0.88", rr.Score)
	}
	if rr.OriginalRef != "e5" {
		t.Errorf("OriginalRef = %q, want %q", rr.OriginalRef, "e5")
	}
	if res["clicked"] != true {
		t.Errorf("action result missing clicked=true")
	}
	if rr.LatencyMs < 0 {
		t.Errorf("LatencyMs = %d, should be >= 0", rr.LatencyMs)
	}
}

func TestRecoveryEngine_Attempt_ScoreBelowThreshold(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{
		Query: "submit button",
	})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{
				BestRef:   "e2",
				BestScore: 0.25, // Below default MinConfidence (0.4)
			}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) { return 10, true },
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e2", Role: "button", Name: "Cancel"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			t.Error("action should not be called when score is below threshold")
			return nil, nil
		},
	)

	if err == nil {
		t.Error("expected error for low score")
	}
	if rr.Recovered {
		t.Error("should not recover with score below threshold")
	}
	if !strings.Contains(rr.Error, "no match above threshold") {
		t.Errorf("Error = %q, want to contain threshold message", rr.Error)
	}
}

func TestRecoveryEngine_Attempt_ActionFailsOnReMatch(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "login button"})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{
				BestRef:   "e8",
				BestScore: 0.9,
				Strategy:  "combined",
			}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e8" {
				return 77, true
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e8", Role: "button", Name: "Login"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			return nil, fmt.Errorf("element is disabled")
		},
	)

	if err == nil {
		t.Error("expected error when action fails after re-match")
	}
	if rr.Recovered {
		t.Error("should not be recovered when re-executed action fails")
	}
}

func TestRecoveryEngine_Attempt_EmptySnapshotAfterRefresh(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		&mockMatcher{},
		cache,
		func(_ context.Context, _ string) error { return nil },
		nil,
		func(_ string) []semantic.ElementDescriptor { return nil }, // empty snapshot
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click", nil)
	if err == nil {
		t.Error("expected error for empty snapshot")
	}
	if !strings.Contains(rr.Error, "empty snapshot") {
		t.Errorf("Error = %q, want to contain 'empty snapshot'", rr.Error)
	}
}

func TestRecoveryEngine_Attempt_RefreshFails(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		&mockMatcher{},
		cache,
		func(_ context.Context, _ string) error { return fmt.Errorf("CDP timeout") },
		nil, nil,
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click", nil)
	if err == nil {
		t.Error("expected error when refresh fails")
	}
	if !strings.Contains(rr.Error, "refresh snapshot") {
		t.Errorf("Error = %q, want to contain 'refresh snapshot'", rr.Error)
	}
}

func TestRecoveryEngine_Attempt_MatcherError(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{}, fmt.Errorf("internal matcher error")
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		nil,
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e2", Role: "button", Name: "Submit"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click", nil)
	if err == nil {
		t.Error("expected error when matcher fails")
	}
	if !strings.Contains(rr.Error, "matcher") {
		t.Errorf("Error = %q, want to contain 'matcher'", rr.Error)
	}
}

func TestRecoveryEngine_Attempt_NewRefNotInCache(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{BestRef: "e99", BestScore: 0.9, Strategy: "combined"}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) { return 0, false }, // ref not in cache
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e99", Role: "button", Name: "Submit"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click", nil)
	if err == nil {
		t.Error("expected error when new ref not in node cache")
	}
	if !strings.Contains(rr.Error, "not in cache after refresh") {
		t.Errorf("Error = %q, want to contain 'not in cache after refresh'", rr.Error)
	}
}

func TestRecoveryEngine_AttemptWithClassification(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e3", IntentEntry{
		Query:      "email input",
		Descriptor: semantic.ElementDescriptor{Ref: "e3", Role: "textbox", Name: "Email"},
	})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{
				BestRef:   "e20",
				BestScore: 0.85,
				Strategy:  "combined",
			}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e20" {
				return 55, true
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e20", Role: "textbox", Name: "Email Address"}}
		},
	)

	rr, res, err := re.AttemptWithClassification(
		context.Background(), "tab1", "e3", "fill",
		FailureElementStale,
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			return map[string]any{"filled": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rr.Recovered {
		t.Error("should be recovered")
	}
	if rr.FailureType != "element_stale" {
		t.Errorf("FailureType = %q, want %q", rr.FailureType, "element_stale")
	}
	if res["filled"] != true {
		t.Error("action result missing filled=true")
	}
}

func TestRecoveryEngine_PreferHighConfidence_RejectsLow(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			return semantic.FindResult{
				BestRef:   "e2",
				BestScore: 0.5, // CalibrateConfidence(0.5) = "low"
				Strategy:  "combined",
			}, nil
		},
	}

	cfg := DefaultRecoveryConfig()
	cfg.PreferHighConfidence = true

	re := NewRecoveryEngine(
		cfg,
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) { return 10, true },
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e2", Role: "button", Name: "Submit"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click", nil)
	if err == nil {
		t.Error("expected error when PreferHighConfidence rejects low-confidence match")
	}
	if rr.Recovered {
		t.Error("should not recover with low confidence when PreferHighConfidence=true")
	}
	if !strings.Contains(rr.Error, "confidence too low") {
		t.Errorf("Error = %q, want 'confidence too low'", rr.Error)
	}
}

func TestRecoveryEngine_ReconstructQuery_FallbackToComposite(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	// Store entry with no Query, only a descriptor.
	cache.Store("tab1", "e1", IntentEntry{
		Descriptor: semantic.ElementDescriptor{Ref: "e1", Role: "button", Name: "Sign In"},
	})

	querySeen := ""
	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			querySeen = query
			return semantic.FindResult{
				BestRef:   "e10",
				BestScore: 0.9,
				Strategy:  "combined",
			}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e10" {
				return 33, true
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e10", Role: "button", Name: "Sign In"}}
		},
	)

	_, _, _ = re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	)

	// The query should be the Composite() of the descriptor: "button: Sign In"
	desc := semantic.ElementDescriptor{Ref: "e1", Role: "button", Name: "Sign In"}
	expected := desc.Composite()
	if querySeen != expected {
		t.Errorf("reconstructed query = %q, want %q", querySeen, expected)
	}
}

func TestRecoveryEngine_RecordIntent(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		&mockMatcher{},
		cache,
		nil, nil, nil,
	)

	re.RecordIntent("tab1", "e5", IntentEntry{
		Query:      "search box",
		Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "textbox", Name: "Search"},
	})

	entry, ok := cache.Lookup("tab1", "e5")
	if !ok {
		t.Fatal("RecordIntent should store in IntentCache")
	}
	if entry.Query != "search box" {
		t.Errorf("Query = %q, want %q", entry.Query, "search box")
	}
}

func TestRecoveryEngine_Attempt_UsesAdaptiveThresholdWhenAvailable(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "submit"})

	tracker := NewConfidenceTracker(50, 1)
	tracker.Record(0.9, true)
	tracker.Record(0.3, false)
	adaptiveThreshold := tracker.OptimalThresholdWithDefault(0.4)

	var seenThreshold float64
	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, opts semantic.FindOptions) (semantic.FindResult, error) {
			seenThreshold = opts.Threshold
			return semantic.FindResult{BestRef: "e2", BestScore: 0.7, Strategy: "combined"}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) { return 10, true },
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e2", Role: "button", Name: "Submit"}}
		},
	)
	re.Confidence = tracker

	_, _, err := re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(seenThreshold-adaptiveThreshold) > 1e-9 {
		t.Fatalf("expected adaptive threshold %.6f, got %.6f", adaptiveThreshold, seenThreshold)
	}
}

func TestRecoveryEngine_ConfidenceStats_Exposed(t *testing.T) {
	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		&mockMatcher{},
		NewIntentCache(10, time.Minute),
		nil, nil, nil,
	)
	re.Confidence = NewConfidenceTracker(10, 1)
	re.Confidence.Record(0.8, true)
	re.Confidence.Record(0.2, false)

	stats := re.ConfidenceStats()
	if stats.SuccessCount != 1 {
		t.Fatalf("expected success count 1, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Fatalf("expected failure count 1, got %d", stats.FailureCount)
	}
	if !stats.Adaptive {
		t.Fatalf("expected adaptive=true when min samples are met")
	}
}
func TestRecoveryEngine_RecordIntent_NilCache(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}
	// Should not panic with nil IntentCache.
	re.RecordIntent("tab1", "e1", IntentEntry{Query: "test"})
}
