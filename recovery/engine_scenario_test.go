package recovery

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/semantic"
)

func TestRecovery_Scenario_SPAFormReRender(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-react-app", "e15", IntentEntry{
		Query:      "submit button",
		Descriptor: semantic.ElementDescriptor{Ref: "e15", Role: "button", Name: "Submit"},
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e30", Role: "heading", Name: "Contact Form"},
		{Ref: "e31", Role: "textbox", Name: "Full Name"},
		{Ref: "e32", Role: "textbox", Name: "Email Address"},
		{Ref: "e33", Role: "textbox", Name: "Message"},
		{Ref: "e34", Role: "button", Name: "Submit"},
		{Ref: "e35", Role: "button", Name: "Cancel"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
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

func TestRecovery_Scenario_EcommerceCheckout(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-shop", "e50", IntentEntry{
		Query:      "place order button",
		Descriptor: semantic.ElementDescriptor{Ref: "e50", Role: "button", Name: "Place Order"},
		Score:      0.92,
		Confidence: "high",
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e70", Role: "heading", Name: "Your Cart (3 items)"},
		{Ref: "e71", Role: "button", Name: "Update Cart"},
		{Ref: "e72", Role: "button", Name: "Apply Coupon"},
		{Ref: "e73", Role: "button", Name: "Place Order"},
		{Ref: "e74", Role: "link", Name: "Continue Shopping"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
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

func TestRecovery_Scenario_LoginFormNavigation(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-login", "e20", IntentEntry{
		Query: "password input field",
		Descriptor: semantic.ElementDescriptor{
			Ref: "e20", Role: "textbox", Name: "Password",
		},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e40", Role: "heading", Name: "Log In"},
		{Ref: "e41", Role: "textbox", Name: "Username or Email"},
		{Ref: "e42", Role: "textbox", Name: "Password"},
		{Ref: "e43", Role: "button", Name: "Log In"},
		{Ref: "e44", Role: "link", Name: "Forgot Password?"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
	)

	rr, res, err := re.AttemptWithClassification(
		context.Background(), "tab-login", "e20", "fill",
		FailureElementNotFound,
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			if nodeID != 3 {
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

func TestRecovery_Scenario_GoogleSearchButton(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-google", "e10", IntentEntry{
		Query:      "google search button",
		Descriptor: semantic.ElementDescriptor{Ref: "e10", Role: "button", Name: "Google Search"},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e80", Role: "textbox", Name: "Search"},
		{Ref: "e81", Role: "button", Name: "Google Search"},
		{Ref: "e82", Role: "button", Name: "I'm Feeling Lucky"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
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

func TestRecovery_Scenario_DashboardSimilarButtons(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-dash", "e5", IntentEntry{
		Query: "delete account",
		Descriptor: semantic.ElementDescriptor{
			Ref: "e5", Role: "button", Name: "Delete Account",
		},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e100", Role: "button", Name: "Delete Comment"},
		{Ref: "e101", Role: "button", Name: "Delete Post"},
		{Ref: "e102", Role: "button", Name: "Delete Account"},
		{Ref: "e103", Role: "button", Name: "Edit Account"},
		{Ref: "e104", Role: "button", Name: "Save Settings"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
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

func TestRecovery_Scenario_CMSNavigationLink(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-cms", "e7", IntentEntry{
		Descriptor: semantic.ElementDescriptor{Ref: "e7", Role: "link", Name: "About Us"},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e200", Role: "link", Name: "Home"},
		{Ref: "e201", Role: "link", Name: "Services"},
		{Ref: "e202", Role: "link", Name: "About Us"},
		{Ref: "e203", Role: "link", Name: "Contact"},
		{Ref: "e204", Role: "link", Name: "Blog"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

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
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
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

func TestRecovery_Scenario_RenamedLogoutButton(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-account", "e5", IntentEntry{
		Query:      "log out",
		Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Log Out"},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e5", Role: "button", Name: "Logout"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			if ref == "e5" {
				return 500, true
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
	)

	rr, res, err := re.AttemptWithClassification(
		context.Background(), "tab-account", "e5", "click",
		FailureElementStale,
		func(_ context.Context, _ string, nodeID int64) (map[string]any, error) {
			if nodeID != 500 {
				return nil, fmt.Errorf("wrong button, nodeID=%d want 500", nodeID)
			}
			return map[string]any{"clicked": true}, nil
		},
	)

	if err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !rr.Recovered {
		t.Fatal("should recover for 'log out' -> 'Logout'")
	}
	if rr.NewRef != "e5" {
		t.Errorf("NewRef = %q, want e5", rr.NewRef)
	}
	if rr.Score < defaultRecoveryMinConfidence {
		t.Errorf("Score = %f, want >= %f", rr.Score, defaultRecoveryMinConfidence)
	}
	if res["clicked"] != true {
		t.Error("action result should contain clicked=true")
	}
}

func TestRecovery_Scenario_RemovedDeleteButton_NoFalsePositive(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab-items", "e3", IntentEntry{
		Query:      "delete button",
		Descriptor: semantic.ElementDescriptor{Ref: "e3", Role: "button", Name: "Delete"},
	})

	freshDescs := []semantic.ElementDescriptor{
		{Ref: "e1", Role: "text", Name: "Item 1"},
		{Ref: "e2", Role: "button", Name: "Edit"},
		{Ref: "e3", Role: "button", Name: "Archive"},
	}

	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, _ string) (int64, bool) {
			t.Fatal("resolver should not be called when no candidate clears the recovery threshold")
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor { return freshDescs },
	)

	rr, _, err := re.AttemptWithClassification(
		context.Background(), "tab-items", "e3", "click",
		FailureElementNotFound,
		func(_ context.Context, _ string, _ int64) (map[string]any, error) {
			t.Fatal("action executor should not run for a low-confidence recovery")
			return nil, nil
		},
	)

	if err == nil {
		t.Fatal("expected recovery to reject low-confidence false positive")
	}
	if rr.Recovered {
		t.Fatal("should not recover when only low-confidence alternatives remain")
	}
	if !strings.Contains(rr.Error, "no match above threshold") {
		t.Fatalf("Error = %q, want threshold rejection", rr.Error)
	}
	if rr.NewRef != "" {
		t.Errorf("NewRef = %q, want empty", rr.NewRef)
	}
}

func TestRecovery_Scenario_NetworkError_NoAttempt(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}

	err := fmt.Errorf("websocket: connection refused to remote debugging port")
	if re.ShouldAttempt(err, "e1") {
		t.Error("ShouldAttempt should be false for network errors")
	}
}

func TestRecovery_Scenario_UnknownError_NoAttempt(t *testing.T) {
	re := &RecoveryEngine{Config: DefaultRecoveryConfig()}

	err := fmt.Errorf("some completely unexpected internal error")
	if re.ShouldAttempt(err, "e1") {
		t.Error("ShouldAttempt should be false for unknown errors")
	}
}

func TestRecovery_Scenario_MultipleRetries(t *testing.T) {
	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("tab1", "e1", IntentEntry{Query: "save button"})

	attempt := 0
	matcher := &mockMatcher{
		findFn: func(_ context.Context, _ string, _ []semantic.ElementDescriptor, _ semantic.FindOptions) (semantic.FindResult, error) {
			attempt++
			if attempt == 1 {
				return semantic.FindResult{BestRef: "e10", BestScore: 0.2}, nil
			}
			return semantic.FindResult{BestRef: "e11", BestScore: 0.95, Strategy: "combined"}, nil
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
		func(_ string) []semantic.ElementDescriptor {
			return []semantic.ElementDescriptor{{Ref: "e11", Role: "button", Name: "Save"}}
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
