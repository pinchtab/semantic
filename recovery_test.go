package semantic

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ===========================================================================
// FailureType classification tests
// ===========================================================================

func TestClassifyFailure_NilError(t *testing.T) {
	ft := ClassifyFailure(nil)
	if ft != FailureUnknown {
		t.Errorf("ClassifyFailure(nil) = %v, want FailureUnknown", ft)
	}
}

func TestClassifyFailure_ElementNotFound(t *testing.T) {
	patterns := []string{
		"could not find node with id 42",
		"node with given id not found",
		"no node for backendNodeId 123",
		"ref not found: e15",
		"Node not found for the given backend node id",
		"backend node id cannot be resolved",
		"no node with given id",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementNotFound {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementNotFound", p, ft)
		}
	}
}

func TestClassifyFailure_ElementStale(t *testing.T) {
	patterns := []string{
		"stale element reference",
		"node is detached from the document",
		"execution context was destroyed",
		"orphan node detected",
		"object reference not set",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementStale {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementStale", p, ft)
		}
	}
}

func TestClassifyFailure_NotInteractable(t *testing.T) {
	patterns := []string{
		"node is not visible on the page",
		"element is not interactable at point (100, 200)",
		"overlapping element covers the target",
		"element is disabled",
		"cannot focus the target element",
		"Element is outside of the viewport",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementNotInteractable {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementNotInteractable", p, ft)
		}
	}
}

func TestClassifyFailure_Navigation(t *testing.T) {
	patterns := []string{
		"page navigated while action was pending",
		"Frame was detached during execution",
		"inspected target navigated or closed",
		"Navigated away from page",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureNavigation {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureNavigation", p, ft)
		}
	}
}

func TestClassifyFailure_Network(t *testing.T) {
	patterns := []string{
		"connection refused to remote debugging port",
		"websocket connection closed unexpectedly",
		"could not connect to Chrome",
		"timeout waiting for response from browser",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureNetwork {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureNetwork", p, ft)
		}
	}
}

func TestClassifyFailure_Unknown(t *testing.T) {
	patterns := []string{
		"something completely unexpected happened",
		"random error",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureUnknown {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureUnknown", p, ft)
		}
	}
}

func TestFailureType_String(t *testing.T) {
	cases := map[FailureType]string{
		FailureUnknown:                "unknown",
		FailureElementNotFound:        "element_not_found",
		FailureElementStale:           "element_stale",
		FailureElementNotInteractable: "element_not_interactable",
		FailureNavigation:             "navigation",
		FailureNetwork:                "network",
	}
	for ft, want := range cases {
		if ft.String() != want {
			t.Errorf("FailureType(%d).String() = %q, want %q", ft, ft.String(), want)
		}
	}
}

func TestFailureType_Recoverable(t *testing.T) {
	recoverable := []FailureType{
		FailureElementNotFound,
		FailureElementStale,
		FailureElementNotInteractable,
		FailureNavigation,
	}
	for _, ft := range recoverable {
		if !ft.Recoverable() {
			t.Errorf("%v.Recoverable() = false, want true", ft)
		}
	}
	nonRecoverable := []FailureType{
		FailureUnknown,
		FailureNetwork,
	}
	for _, ft := range nonRecoverable {
		if ft.Recoverable() {
			t.Errorf("%v.Recoverable() = true, want false", ft)
		}
	}
}

// ===========================================================================
// IntentCache tests
// ===========================================================================

func TestIntentCache_StoreAndLookup(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	entry := IntentEntry{
		Query:      "submit button",
		Descriptor: ElementDescriptor{Ref: "e1", Role: "button", Name: "Submit"},
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	}
	c.Store("tab1", "e1", entry)

	got, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup returned false for stored entry")
	}
	if got.Query != "submit button" {
		t.Errorf("Query = %q, want %q", got.Query, "submit button")
	}
	if got.Descriptor.Role != "button" {
		t.Errorf("Descriptor.Role = %q, want %q", got.Descriptor.Role, "button")
	}
	if got.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", got.Score)
	}
}

func TestIntentCache_LookupMiss(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("Lookup returned true for missing entry")
	}
	// Wrong tab.
	c.Store("tab1", "e1", IntentEntry{Query: "test"})
	_, ok = c.Lookup("tab2", "e1")
	if ok {
		t.Error("Lookup returned true for wrong tab")
	}
}

func TestIntentCache_TTLExpiry(t *testing.T) {
	c := NewIntentCache(100, 50*time.Millisecond)

	c.Store("tab1", "e1", IntentEntry{Query: "test", CachedAt: time.Now()})

	// Should be found immediately.
	_, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup should find fresh entry")
	}

	// Wait for expiry.
	time.Sleep(60 * time.Millisecond)
	_, ok = c.Lookup("tab1", "e1")
	if ok {
		t.Error("Lookup should return false after TTL expiry")
	}
}

func TestIntentCache_LRUEviction(t *testing.T) {
	c := NewIntentCache(3, 5*time.Minute)

	// Fill to capacity.
	c.Store("tab1", "e1", IntentEntry{Query: "first", CachedAt: time.Now().Add(-3 * time.Minute)})
	c.Store("tab1", "e2", IntentEntry{Query: "second", CachedAt: time.Now().Add(-2 * time.Minute)})
	c.Store("tab1", "e3", IntentEntry{Query: "third", CachedAt: time.Now().Add(-1 * time.Minute)})

	// Add one more — oldest (e1) should be evicted.
	c.Store("tab1", "e4", IntentEntry{Query: "fourth"})

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("e1 should have been evicted (oldest)")
	}
	_, ok = c.Lookup("tab1", "e4")
	if !ok {
		t.Error("e4 should be present after eviction")
	}
}

func TestIntentCache_InvalidateTab(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	c.Store("tab1", "e1", IntentEntry{Query: "test"})
	c.Store("tab1", "e2", IntentEntry{Query: "test2"})
	c.Store("tab2", "e1", IntentEntry{Query: "other"})

	c.InvalidateTab("tab1")

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("tab1/e1 should be gone after InvalidateTab")
	}
	_, ok = c.Lookup("tab2", "e1")
	if !ok {
		t.Error("tab2/e1 should still be present")
	}
}

func TestIntentCache_Size(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)
	if c.Size() != 0 {
		t.Errorf("Size = %d, want 0", c.Size())
	}

	c.Store("tab1", "e1", IntentEntry{Query: "a"})
	c.Store("tab1", "e2", IntentEntry{Query: "b"})
	c.Store("tab2", "e3", IntentEntry{Query: "c"})
	if c.Size() != 3 {
		t.Errorf("Size = %d, want 3", c.Size())
	}
}

func TestIntentCache_UpdateExisting(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	c.Store("tab1", "e1", IntentEntry{Query: "old query"})
	c.Store("tab1", "e1", IntentEntry{Query: "new query"})

	got, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup should find updated entry")
	}
	if got.Query != "new query" {
		t.Errorf("Query = %q, want %q", got.Query, "new query")
	}
	if c.Size() != 1 {
		t.Errorf("Size = %d, want 1 (update should not add)", c.Size())
	}
}

func TestIntentCache_DefaultValues(t *testing.T) {
	// Zero/negative values should be replaced with defaults.
	c := NewIntentCache(0, 0)
	if c.maxRefs != 200 {
		t.Errorf("maxRefs = %d, want 200", c.maxRefs)
	}
	if c.ttl != 10*time.Minute {
		t.Errorf("ttl = %v, want 10m", c.ttl)
	}
}

// ===========================================================================
// RecoveryEngine tests
// ===========================================================================

// mockMatcher is a configurable test double for ElementMatcher.
type mockMatcher struct {
	findFn func(ctx context.Context, query string, descs []ElementDescriptor, opts FindOptions) (FindResult, error)
}

func (m *mockMatcher) Find(ctx context.Context, query string, descs []ElementDescriptor, opts FindOptions) (FindResult, error) {
	if m.findFn != nil {
		return m.findFn(ctx, query, descs, opts)
	}
	return FindResult{}, fmt.Errorf("mockMatcher: Find not configured")
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
		Descriptor: ElementDescriptor{Ref: "e5", Role: "button", Name: "Submit"},
	})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, descs []ElementDescriptor, opts FindOptions) (FindResult, error) {
			if query != "submit button" {
				return FindResult{}, fmt.Errorf("unexpected query: %s", query)
			}
			return FindResult{
				BestRef:   "e12",
				BestScore: 0.88,
				Matches: []ElementMatch{
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
		func(tabID string) []ElementDescriptor {
			return []ElementDescriptor{
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
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{
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
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e2", Role: "button", Name: "Cancel"}}
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
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{
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
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e8", Role: "button", Name: "Login"}}
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
		func(_ string) []ElementDescriptor { return nil }, // empty snapshot
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
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{}, fmt.Errorf("internal matcher error")
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		nil,
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e2", Role: "button", Name: "Submit"}}
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
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{BestRef: "e99", BestScore: 0.9, Strategy: "combined"}, nil
		},
	}

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) { return 0, false }, // ref not in cache
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e99", Role: "button", Name: "Submit"}}
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
		Descriptor: ElementDescriptor{Ref: "e3", Role: "textbox", Name: "Email"},
	})

	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{
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
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e20", Role: "textbox", Name: "Email Address"}}
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
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			return FindResult{
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
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e2", Role: "button", Name: "Submit"}}
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
		Descriptor: ElementDescriptor{Ref: "e1", Role: "button", Name: "Sign In"},
	})

	querySeen := ""
	matcher := &mockMatcher{
		findFn: func(_ context.Context, query string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			querySeen = query
			return FindResult{
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
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e10", Role: "button", Name: "Sign In"}}
		},
	)

	_, _, _ = re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	)

	// The query should be the Composite() of the descriptor: "button: Sign In"
	desc := ElementDescriptor{Ref: "e1", Role: "button", Name: "Sign In"}
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
		Descriptor: ElementDescriptor{Ref: "e5", Role: "textbox", Name: "Search"},
	})

	entry, ok := cache.Lookup("tab1", "e5")
	if !ok {
		t.Fatal("RecordIntent should store in IntentCache")
	}
	if entry.Query != "search box" {
		t.Errorf("Query = %q, want %q", entry.Query, "search box")
	}
}

func TestRecoveryEngine_RecordIntent_NilCache(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}
	// Should not panic with nil IntentCache.
	re.RecordIntent("tab1", "e1", IntentEntry{Query: "test"})
}

// ===========================================================================
// Realistic scenario tests — simulating real website interactions
// ===========================================================================

// Scenario: SPA form re-render. User found "Submit" button via /find,
// then a React re-render assigned new refs. The click on the old ref
// fails with "could not find node". Recovery should semantically
// re-locate the Submit button.
func TestRecovery_Scenario_SPAFormReRender(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	// Simulate /find having cached the intent for "Submit" button.
	cache.Store("tab-react-app", "e15", IntentEntry{
		Query:      "submit button",
		Descriptor: ElementDescriptor{Ref: "e15", Role: "button", Name: "Submit"},
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	})

	// After re-render, the fresh snapshot has different refs.
	freshDescs := []ElementDescriptor{
		{Ref: "e30", Role: "heading", Name: "Contact Form"},
		{Ref: "e31", Role: "textbox", Name: "Full Name"},
		{Ref: "e32", Role: "textbox", Name: "Email Address"},
		{Ref: "e33", Role: "textbox", Name: "Message"},
		{Ref: "e34", Role: "button", Name: "Submit"},
		{Ref: "e35", Role: "button", Name: "Cancel"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			nodeMap := map[string]int64{
				"e30": 100, "e31": 101, "e32": 102,
				"e33": 103, "e34": 104, "e35": 105,
			}
			nid, ok := nodeMap[ref]
			return nid, ok
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	err := fmt.Errorf("could not find node with id 15")
	if !re.ShouldAttempt(err, "e15") {
		t.Fatal("ShouldAttempt should be true for 'could not find node'")
	}

	rr, res, recErr := re.AttemptWithClassification(
		context.Background(), "tab-react-app", "e15", "click",
		ClassifyFailure(err),
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			if nodeID != 104 {
				return nil, fmt.Errorf("wrong node: %d (expected Submit=104)", nodeID)
			}
			return map[string]any{"clicked": true}, nil
		},
	)

	if recErr != nil {
		t.Fatalf("recovery failed: %v", recErr)
	}
	if !rr.Recovered {
		t.Error("should have recovered")
	}
	if rr.NewRef != "e34" {
		t.Errorf("NewRef = %q, want e34 (Submit button)", rr.NewRef)
	}
	if rr.FailureType != "element_not_found" {
		t.Errorf("FailureType = %q, want element_not_found", rr.FailureType)
	}
	if res["clicked"] != true {
		t.Error("action result should contain clicked=true")
	}
}

// Scenario: E-commerce checkout — the "Place Order" button becomes
// stale after cart updates via AJAX. Recovery should find the replaced
// button.
func TestRecovery_Scenario_EcommerceCheckout(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-shop", "e50", IntentEntry{
		Query:      "place order button",
		Descriptor: ElementDescriptor{Ref: "e50", Role: "button", Name: "Place Order"},
		Score:      0.92,
		Confidence: "high",
	})

	freshDescs := []ElementDescriptor{
		{Ref: "e70", Role: "heading", Name: "Your Cart (3 items)"},
		{Ref: "e71", Role: "button", Name: "Update Cart"},
		{Ref: "e72", Role: "button", Name: "Apply Coupon"},
		{Ref: "e73", Role: "button", Name: "Place Order"},
		{Ref: "e74", Role: "link", Name: "Continue Shopping"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e73" {
				return 200, true
			}
			return 0, false
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	rr, res, err := re.AttemptWithClassification(
		context.Background(), "tab-shop", "e50", "click",
		FailureElementStale,
		func(_ context.Context, _ string, nodeID int64) (map[string]any, error) {
			return map[string]any{"orderPlaced": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover")
	}
	if rr.NewRef != "e73" {
		t.Errorf("NewRef = %q, want e73", rr.NewRef)
	}
	if res["orderPlaced"] != true {
		t.Error("should have orderPlaced=true")
	}
}

// Scenario: Login page — typing into an email field that was replaced
// by a password field after an SPA navigation.
func TestRecovery_Scenario_LoginFormNavigation(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-login", "e20", IntentEntry{
		Query: "password input field",
		Descriptor: ElementDescriptor{
			Ref: "e20", Role: "textbox", Name: "Password",
		},
	})

	freshDescs := []ElementDescriptor{
		{Ref: "e40", Role: "heading", Name: "Log In"},
		{Ref: "e41", Role: "textbox", Name: "Username or Email"},
		{Ref: "e42", Role: "textbox", Name: "Password"},
		{Ref: "e43", Role: "button", Name: "Log In"},
		{Ref: "e44", Role: "link", Name: "Forgot Password?"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			m := map[string]int64{"e40": 1, "e41": 2, "e42": 3, "e43": 4, "e44": 5}
			nid, ok := m[ref]
			return nid, ok
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	rr, res, err := re.AttemptWithClassification(
		context.Background(), "tab-login", "e20", "fill",
		FailureElementNotFound,
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			if nodeID != 3 { // Password field = e42 = nodeID 3
				return nil, fmt.Errorf("filled wrong field, nodeID=%d", nodeID)
			}
			return map[string]any{"filled": true, "kind": kind}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover")
	}
	if rr.NewRef != "e42" {
		t.Errorf("NewRef = %q, want e42 (Password field)", rr.NewRef)
	}
	if res["filled"] != true {
		t.Error("should have filled=true")
	}
}

// Scenario: Google search page — "Google Search" button re-rendered
// with slightly different name "Search" after dynamic page update.
func TestRecovery_Scenario_GoogleSearchButton(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-google", "e10", IntentEntry{
		Query:      "google search button",
		Descriptor: ElementDescriptor{Ref: "e10", Role: "button", Name: "Google Search"},
	})

	freshDescs := []ElementDescriptor{
		{Ref: "e80", Role: "textbox", Name: "Search"},
		{Ref: "e81", Role: "button", Name: "Google Search"},
		{Ref: "e82", Role: "button", Name: "I'm Feeling Lucky"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e81" {
				return 300, true
			}
			return 0, false
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	rr, _, err := re.AttemptWithClassification(
		context.Background(), "tab-google", "e10", "click",
		FailureElementStale,
		func(_ context.Context, _ string, nodeID int64) (map[string]any, error) {
			return map[string]any{"clicked": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover")
	}
	if rr.NewRef != "e81" {
		t.Errorf("NewRef = %q, want e81 (Google Search button)", rr.NewRef)
	}
}

// Scenario: Dashboard with many similar buttons. The recovery should
// still find the right one using the cached query.
func TestRecovery_Scenario_DashboardSimilarButtons(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-dash", "e5", IntentEntry{
		Query: "delete account",
		Descriptor: ElementDescriptor{
			Ref: "e5", Role: "button", Name: "Delete Account",
		},
	})

	freshDescs := []ElementDescriptor{
		{Ref: "e100", Role: "button", Name: "Delete Comment"},
		{Ref: "e101", Role: "button", Name: "Delete Post"},
		{Ref: "e102", Role: "button", Name: "Delete Account"},
		{Ref: "e103", Role: "button", Name: "Edit Account"},
		{Ref: "e104", Role: "button", Name: "Save Settings"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			m := map[string]int64{
				"e100": 10, "e101": 11, "e102": 12, "e103": 13, "e104": 14,
			}
			nid, ok := m[ref]
			return nid, ok
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	rr, _, err := re.AttemptWithClassification(
		context.Background(), "tab-dash", "e5", "click",
		FailureElementNotFound,
		func(_ context.Context, _ string, nodeID int64) (map[string]any, error) {
			if nodeID != 12 {
				return nil, fmt.Errorf("wrong button, nodeID=%d want 12 (Delete Account)", nodeID)
			}
			return map[string]any{"deleted": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover with correct 'Delete Account' button")
	}
	if rr.NewRef != "e102" {
		t.Errorf("NewRef = %q, want e102 (Delete Account)", rr.NewRef)
	}
}

// Scenario: Navigation link that moved position in a CMS after page
// edit. Descriptor-only recovery (no query stored).
func TestRecovery_Scenario_CMSNavigationLink(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	// No query — only descriptor. Recovery falls back to Composite().
	cache.Store("tab-cms", "e7", IntentEntry{
		Descriptor: ElementDescriptor{Ref: "e7", Role: "link", Name: "About Us"},
	})

	freshDescs := []ElementDescriptor{
		{Ref: "e200", Role: "link", Name: "Home"},
		{Ref: "e201", Role: "link", Name: "Services"},
		{Ref: "e202", Role: "link", Name: "About Us"},
		{Ref: "e203", Role: "link", Name: "Contact"},
		{Ref: "e204", Role: "link", Name: "Blog"},
	}

	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			m := map[string]int64{
				"e200": 50, "e201": 51, "e202": 52, "e203": 53, "e204": 54,
			}
			nid, ok := m[ref]
			return nid, ok
		},
		func(_ string) []ElementDescriptor { return freshDescs },
	)

	rr, _, err := re.AttemptWithClassification(
		context.Background(), "tab-cms", "e7", "click",
		FailureElementNotFound,
		func(_ context.Context, _ string, nodeID int64) (map[string]any, error) {
			return map[string]any{"navigated": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover")
	}
	if rr.NewRef != "e202" {
		t.Errorf("NewRef = %q, want e202 (About Us link)", rr.NewRef)
	}
}

// Scenario: Unrecoverable network error — recovery should NOT attempt.
func TestRecovery_Scenario_NetworkError_NoAttempt(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}

	err := fmt.Errorf("websocket: connection refused to remote debugging port")
	if re.ShouldAttempt(err, "e1") {
		t.Error("ShouldAttempt should be false for network errors")
	}
}

// Scenario: Unknown error — recovery should NOT attempt.
func TestRecovery_Scenario_UnknownError_NoAttempt(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}

	err := fmt.Errorf("some completely unexpected internal error")
	if re.ShouldAttempt(err, "e1") {
		t.Error("ShouldAttempt should be false for unknown errors")
	}
}

// Scenario: MaxRetries > 1. First attempt fails (matcher returns low
// score), second attempt succeeds after the snapshot changes.
func TestRecovery_Scenario_MultipleRetries(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "save button"})

	attempt := 0
	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []ElementDescriptor, _ FindOptions) (FindResult, error) {
			attempt++
			if attempt == 1 {
				// First attempt: score too low.
				return FindResult{BestRef: "e10", BestScore: 0.2}, nil
			}
			// Second attempt: match found.
			return FindResult{BestRef: "e11", BestScore: 0.95, Strategy: "combined"}, nil
		},
	}

	cfg := DefaultRecoveryConfig()
	cfg.MaxRetries = 3

	re := NewRecoveryEngine(
		cfg,
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e11" {
				return 99, true
			}
			return 0, false
		},
		func(_ string) []ElementDescriptor {
			return []ElementDescriptor{{Ref: "e11", Role: "button", Name: "Save"}}
		},
	)

	rr, _, err := re.Attempt(context.Background(), "tab1", "e1", "click",
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			return map[string]any{"saved": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Error("should recover on second attempt")
	}
	if rr.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", rr.Attempts)
	}
}

// ===========================================================================
// DefaultRecoveryConfig tests
// ===========================================================================

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
