package semantic

// Synonym and paraphrase resolution tests.
// Verify that Find() correctly matches queries using alternate vocabulary
// ("log in" → "Sign In", "purchase" → "Checkout", etc.).

import (
	"context"
	"testing"
)



func TestCombined_ExactMatch_Wikipedia(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	tests := []struct {
		query    string
		wantRef  string
		wantDesc string
	}{
		{"Search Wikipedia", "e1", "Search Wikipedia"},
		{"Log in", "e8", "Log in"},
		{"Create account", "e9", "Create account"},
		{"Main menu", "e10", "Main menu"},
		{"Search button", "e2", "Search"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result, err := matcher.Find(context.Background(), tt.query, sites["wikipedia"], FindOptions{
				Threshold: 0.2,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: got BestRef=%s (score=%.3f), want %s (%s)",
					tt.query, result.BestRef, result.BestScore, tt.wantRef, tt.wantDesc)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

// CATEGORY 2: Synonym Tests (the primary weakness)

// CATEGORY 2: Synonym Tests (the primary weakness)

// CATEGORY 2: Synonym Tests (the primary weakness)

// CATEGORY 2: Synonym Tests (the primary weakness)

func TestCombined_Synonym_SignIn_LogIn(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	// "sign in" should find "Log in" on Wikipedia
	result, err := matcher.Find(context.Background(), "sign in", sites["wikipedia"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='sign in': BestRef=%s Score=%.3f Confidence=%s", result.BestRef, result.BestScore, result.ConfidenceLabel())
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// After improvements, "sign in" should match "Log in" (e8) with decent score
	if result.BestRef != "e8" {
		t.Errorf("expected 'sign in' to match 'Log in' (e8), got %s", result.BestRef)
	}
}

func TestCombined_Synonym_Register_CreateAccount(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "register", sites["wikipedia"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='register': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "register" should match "Create account" (e9)
	foundInTop3 := false
	for _, m := range result.Matches {
		if m.Ref == "e9" {
			foundInTop3 = true
			break
		}
	}
	if !foundInTop3 {
		t.Errorf("expected 'register' to find 'Create account' (e9) in top matches")
	}
}

func TestCombined_Synonym_LookUp_Search(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "look up", sites["wikipedia"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='look up': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "look up" should match "Search Wikipedia" (e1) or "Search" (e2)
	foundSearch := false
	for _, m := range result.Matches {
		if m.Ref == "e1" || m.Ref == "e2" {
			foundSearch = true
			break
		}
	}
	if !foundSearch {
		t.Errorf("expected 'look up' to find Search element in top matches")
	}
}

func TestCombined_Synonym_Navigation_MainMenu(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "navigation", sites["wikipedia"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='navigation': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "navigation" should match "Main menu" (e10 has role="navigation")
	if result.BestRef != "e10" {
		t.Errorf("expected 'navigation' to match 'Main menu' (e10), got %s", result.BestRef)
	}
}

func TestCombined_Synonym_Login_SignIn(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	// GitHub login page: "login" should find "Sign in" button
	result, err := matcher.Find(context.Background(), "login", sites["github_login"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='login' on GitHub: BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// Should find "Sign in" button (e5) or heading "Sign in to GitHub" (e2)
	foundSignIn := false
	for _, m := range result.Matches {
		if m.Ref == "e5" || m.Ref == "e2" {
			foundSignIn = true
			break
		}
	}
	if !foundSignIn {
		t.Errorf("expected 'login' to find 'Sign in' element on GitHub login page")
	}
}

func TestCombined_Synonym_Purchase_Checkout(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "purchase", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='purchase': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "purchase" should match Checkout, Buy Now, or Place Order
	foundPurchase := false
	for _, m := range result.Matches {
		if m.Ref == "e7" || m.Ref == "e8" || m.Ref == "e9" {
			foundPurchase = true
			break
		}
	}
	if !foundPurchase {
		t.Errorf("expected 'purchase' to find checkout/buy/order related element")
	}
}

func TestCombined_Synonym_ProceedToPayment_PlaceOrder(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "proceed to payment", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='proceed to payment': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	if result.BestRef != "e8" {
		t.Fatalf("expected 'proceed to payment' to match 'Place Order' (e8), got %s", result.BestRef)
	}
}

func TestCombined_Synonym_Dismiss_Close(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "dismiss", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='dismiss': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "dismiss" should match "Close" (e15)
	if result.BestRef != "e15" {
		t.Errorf("expected 'dismiss' to match 'Close' (e15), got %s", result.BestRef)
	}
}

func TestCombined_Synonym_Download_Export(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "download orders", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='download orders': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "download orders" should match "Export Orders" (e12)
	if result.BestRef != "e12" {
		t.Errorf("expected 'download orders' to match 'Export Orders' (e12), got %s", result.BestRef)
	}
}

// CATEGORY 3: Paraphrase Tests

// CATEGORY 3: Paraphrase Tests

// CATEGORY 3: Paraphrase Tests

// CATEGORY 3: Paraphrase Tests

func TestCombined_Paraphrase_ForgotPassword(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "reset password", sites["github_login"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='reset password': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "reset password" should match "Forgot password?" (e6)
	if result.BestRef != "e6" {
		t.Errorf("expected 'reset password' to match 'Forgot password?' (e6), got %s", result.BestRef)
	}
}

func TestCombined_Paraphrase_ShoppingBag(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "shopping bag", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='shopping bag': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "shopping bag" should match "Cart" (e3) via synonym: cart -> bag
	foundCart := false
	for _, m := range result.Matches {
		if m.Ref == "e3" || m.Ref == "e4" {
			foundCart = true
			break
		}
	}
	if !foundCart {
		t.Errorf("expected 'shopping bag' to find Cart element")
	}
}

// CATEGORY 4: Partial Match / Abbreviation Tests

// CATEGORY 4: Partial Match / Abbreviation Tests
