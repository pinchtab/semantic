package semantic

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestComprehensiveEvaluation(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	type evalCase struct {
		category string
		query    string
		site     string
		wantRef  string
		wantName string
	}

	cases := []evalCase{
		// Exact matches
		{"exact", "Search Wikipedia", "wikipedia", "e1", "Search Wikipedia"},
		{"exact", "Log in", "wikipedia", "e8", "Log in"},
		{"exact", "Create account", "wikipedia", "e9", "Create account"},
		{"exact", "Sign in", "github_login", "e5", "Sign in"},
		{"exact", "Google Search", "google", "e2", "Google Search"},

		// Synonyms (the main weakness)
		{"synonym", "sign in", "wikipedia", "e8", "Log in"},
		{"synonym", "register", "wikipedia", "e9", "Create account"},
		{"synonym", "look up", "wikipedia", "e1", "Search Wikipedia"},
		{"synonym", "navigation", "wikipedia", "e10", "Main menu"},
		{"synonym", "login button", "github_login", "e5", "Sign in"},
		{"synonym", "authenticate", "github_login", "e5", "Sign in"},
		{"synonym", "dismiss", "ecommerce", "e15", "Close"},
		{"synonym", "download orders", "ecommerce", "e12", "Export Orders"},

		// Paraphrases
		{"paraphrase", "reset password", "github_login", "e6", "Forgot password?"},
		{"paraphrase", "email field", "github_login", "e3", "Username or email address"},

		// Partial / abbreviations
		{"partial", "qty", "ecommerce", "e11", "Quantity"},
	}

	results := make(map[string][]bool)
	var totalPass, totalFail int

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%s", tc.category, tc.query), func(t *testing.T) {
			result, err := matcher.Find(context.Background(), tc.query, sites[tc.site], FindOptions{
				Threshold: 0.1,
				TopK:      5,
			})
			if err != nil {
				t.Fatal(err)
			}

			pass := false
			for i, m := range result.Matches {
				if i >= 3 {
					break
				}
				if m.Ref == tc.wantRef {
					pass = true
					break
				}
			}

			results[tc.category] = append(results[tc.category], pass)
			if pass {
				totalPass++
				t.Logf("PASS: query=%q -> %s (score=%.3f)", tc.query, tc.wantName, result.BestScore)
			} else {
				totalFail++
				t.Logf("MISS: query=%q wanted %s (%s), got BestRef=%s (score=%.3f)",
					tc.query, tc.wantRef, tc.wantName, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%.3f name=%s", m.Ref, m.Score, m.Name)
				}
			}
		})
	}

	// Summary
	t.Logf("\n=== EVALUATION SUMMARY ===")
	t.Logf("Total: %d/%d (%.1f%%)", totalPass, totalPass+totalFail, 100*float64(totalPass)/float64(totalPass+totalFail))
	for cat, res := range results {
		passed := 0
		for _, r := range res {
			if r {
				passed++
			}
		}
		t.Logf("  %s: %d/%d (%.0f%%)", cat, passed, len(res), 100*float64(passed)/float64(len(res)))
	}
}

// ===========================================================================
// Hashing Embedder Synonym Feature Tests
// ===========================================================================

func TestScoreDistribution_BeforeVsExpected(t *testing.T) {
	// This test documents the expected improvement in scores
	// for the queries that were failing in the real-world evaluation
	type scoreCase struct {
		query    string
		desc     string
		minScore float64
		label    string
	}

	cases := []scoreCase{
		// These had very low scores before (from the issue)
		{"sign in", "link: Log in", 0.15, "synonym: sign in -> Log in"},
		{"register", "link: Create account", 0.10, "synonym: register -> Create account"},
		{"look up", "search: Search", 0.10, "synonym: look up -> Search"},
		{"navigation", "navigation: Main menu", 0.15, "synonym: navigation -> Main menu"},
		{"login button", "button: Sign in", 0.15, "synonym: login -> Sign in"},
		{"dismiss", "button: Close", 0.10, "synonym: dismiss -> Close"},
		{"download", "button: Export", 0.10, "synonym: download -> Export"},

		// Prefix/abbreviation cases
		{"btn submit", "button: Submit", 0.30, "prefix: btn -> button"},
		{"nav", "navigation: Main navigation", 0.15, "prefix: nav -> navigation"},

		// These should still work well (exact matches)
		{"submit button", "button: Submit", 0.50, "exact: submit button"},
		{"search box", "search: Search", 0.30, "exact: search"},
		{"email input", "textbox: Email", 0.20, "exact-ish: email input"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			score := lexicalScore(tc.query, tc.desc)
			status := "PASS"
			if score < tc.minScore {
				status = "FAIL"
			}
			t.Logf("[%s] lexicalScore(%q, %q) = %.4f (min: %.2f)", status, tc.query, tc.desc, score, tc.minScore)
			if score < tc.minScore {
				t.Errorf("score %.4f below minimum %.2f", score, tc.minScore)
			}
		})
	}
}

// ===========================================================================
// Stopword Edge Cases
// ===========================================================================

func TestMultiSiteEvaluation(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	type testCase struct {
		category string
		query    string
		site     string
		wantRef  string
		wantName string
	}

	allCases := []testCase{
		// === EXACT MATCHES ===
		{"exact", "Search Wikipedia", "wikipedia", "e1", "Search Wikipedia"},
		{"exact", "Log in", "wikipedia", "e8", "Log in"},
		{"exact", "Create account", "wikipedia", "e9", "Create account"},
		{"exact", "Sign in", "github_login", "e5", "Sign in"},
		{"exact", "Password", "github_login", "e4", "Password"},
		{"exact", "Google Search", "google", "e2", "Google Search"},
		{"exact", "Cart", "ecommerce", "e3", "Cart"},
		{"exact", "Add to Cart", "ecommerce", "e4", "Add to Cart"},

		// === SYNONYMS ===
		{"synonym", "sign in", "wikipedia", "e8", "Log in"},
		{"synonym", "register", "wikipedia", "e9", "Create account"},
		{"synonym", "look up", "wikipedia", "e1", "Search Wikipedia"},
		{"synonym", "navigation", "wikipedia", "e10", "Main menu"},
		{"synonym", "login", "github_login", "e5", "Sign in"},
		{"synonym", "authenticate", "github_login", "e5", "Sign in"},
		{"synonym", "dismiss", "ecommerce", "e15", "Close"},
		{"synonym", "download orders", "ecommerce", "e12", "Export Orders"},
		{"synonym", "purchase", "ecommerce", "e7", "Buy Now"},

		// === PARAPHRASES ===
		{"paraphrase", "reset password", "github_login", "e6", "Forgot password?"},
		{"paraphrase", "search input", "google", "e1", "Search"},
		{"paraphrase", "email field", "github_login", "e3", "Username or email address"},
		{"paraphrase", "shopping bag", "ecommerce", "e3", "Cart"},

		// === PARTIAL/ABBREVIATIONS ===
		{"partial", "qty", "ecommerce", "e11", "Quantity"},
		{"partial", "nav menu", "ecommerce", "e13", "Main navigation"},

		// === EDGE CASES ===
		{"edge", "top right login link", "wikipedia", "e8", "Log in"},
	}

	catResults := make(map[string]struct{ pass, total int })

	for _, tc := range allCases {
		t.Run(fmt.Sprintf("%s_%s_%s", tc.site, tc.category, strings.ReplaceAll(tc.query, " ", "_")), func(t *testing.T) {
			result, err := matcher.Find(context.Background(), tc.query, sites[tc.site], FindOptions{
				Threshold: 0.1,
				TopK:      5,
			})
			if err != nil {
				t.Fatal(err)
			}

			pass := false
			for i, m := range result.Matches {
				if i >= 3 {
					break
				}
				if m.Ref == tc.wantRef {
					pass = true
					break
				}
			}

			cr := catResults[tc.category]
			cr.total++
			if pass {
				cr.pass++
				t.Logf("query=%q -> %s score=%.3f", tc.query, tc.wantName, result.BestScore)
			} else {
				t.Logf("MISS query=%q wanted=%s (%s) got=%s score=%.3f",
					tc.query, tc.wantRef, tc.wantName, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  ref=%s score=%.3f name=%s", m.Ref, m.Score, m.Name)
				}
			}
			catResults[tc.category] = cr
		})
	}

	// Print summary table
	t.Logf("\n╔══════════════════════════════════════════════════╗")
	t.Logf("║        MULTI-SITE EVALUATION SUMMARY            ║")
	t.Logf("╠══════════════════════════════════════════════════╣")
	totalP, totalT := 0, 0
	for _, cat := range []string{"exact", "synonym", "paraphrase", "partial", "edge"} {
		cr := catResults[cat]
		pct := 0.0
		if cr.total > 0 {
			pct = 100 * float64(cr.pass) / float64(cr.total)
		}
		t.Logf("║  %-14s  %d/%d  (%.0f%%)                       ║", cat, cr.pass, cr.total, pct)
		totalP += cr.pass
		totalT += cr.total
	}
	pct := 0.0
	if totalT > 0 {
		pct = 100 * float64(totalP) / float64(totalT)
	}
	t.Logf("╠══════════════════════════════════════════════════╣")
	t.Logf("║  TOTAL           %d/%d  (%.0f%%)                      ║", totalP, totalT, pct)
	t.Logf("╚══════════════════════════════════════════════════╝")
}

// ---------------------------------------------------------------------------
// Round-2 bug-fix tests
// ---------------------------------------------------------------------------


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

// ===========================================================================
// CATEGORY 2: Synonym Tests (the primary weakness)
// ===========================================================================

// ===========================================================================
// CATEGORY 2: Synonym Tests (the primary weakness)
// ===========================================================================

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

// ===========================================================================
// CATEGORY 3: Paraphrase Tests
// ===========================================================================

// ===========================================================================
// CATEGORY 3: Paraphrase Tests
// ===========================================================================

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

// ===========================================================================
// CATEGORY 4: Partial Match / Abbreviation Tests
// ===========================================================================

// ===========================================================================
// CATEGORY 4: Partial Match / Abbreviation Tests
// ===========================================================================

func TestCombined_Partial_Btn(t *testing.T) {
	elements := []ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
		{Ref: "e2", Role: "link", Name: "Home"},
		{Ref: "e3", Role: "textbox", Name: "Email"},
	}
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "submit btn", elements, FindOptions{
		Threshold: 0.15,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='submit btn': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	if result.BestRef != "e1" {
		t.Errorf("expected 'submit btn' to match 'Submit' button (e1), got %s", result.BestRef)
	}
}

func TestCombined_Partial_Nav(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "nav menu", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='nav menu': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "nav menu" should match "Main navigation" (e13) via prefix + synonym
	if result.BestRef != "e13" {
		t.Errorf("expected 'nav menu' to match 'Main navigation' (e13), got %s", result.BestRef)
	}
}

func TestCombined_Partial_Qty(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "qty", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='qty': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "qty" should match "Quantity" (e11) via synonym
	if result.BestRef != "e11" {
		t.Errorf("expected 'qty' to match 'Quantity' (e11), got %s", result.BestRef)
	}
}

// ===========================================================================
// CATEGORY 5: Edge Cases
// ===========================================================================

// ===========================================================================
// CATEGORY 5: Edge Cases
// ===========================================================================

func TestCombined_EdgeCase_EmptyQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []ElementDescriptor{{Ref: "e1", Role: "button", Name: "Submit"}}

	result, err := matcher.Find(context.Background(), "", elements, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) > 0 {
		t.Errorf("expected no matches for empty query, got %d", len(result.Matches))
	}
}

func TestCombined_EdgeCase_GibberishQuery(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "xyzzy plugh qwerty", sites["wikipedia"], FindOptions{
		Threshold: 0.3,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Gibberish query: matches=%d best_score=%.3f", len(result.Matches), result.BestScore)
	// Should return no matches at threshold 0.3
	if len(result.Matches) > 0 {
		t.Errorf("expected no matches for gibberish query at threshold 0.3, got %d", len(result.Matches))
	}
}

func TestCombined_EdgeCase_AllStopwords(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
		{Ref: "e2", Role: "link", Name: "The"},
	}

	result, err := matcher.Find(context.Background(), "the a is", elements, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("All-stopwords query: matches=%d", len(result.Matches))
}

func TestCombined_EdgeCase_VeryLongQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
	}

	longQuery := "I want to find the submit button that is located on the bottom right of the page and click on it to submit the form"
	result, err := matcher.Find(context.Background(), longQuery, elements, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Long query: matches=%d best_score=%.3f", len(result.Matches), result.BestScore)
	if result.BestRef != "e1" {
		t.Errorf("expected long query to still find 'Submit' button, got %s", result.BestRef)
	}
}

func TestCombined_EdgeCase_SingleCharQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []ElementDescriptor{
		{Ref: "e1", Role: "link", Name: "X"},
		{Ref: "e2", Role: "button", Name: "Close"},
	}

	result, err := matcher.Find(context.Background(), "x", elements, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Single char 'x': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
}

// ===========================================================================
// CATEGORY 6: Role Boost Accumulation Test (Bug Fix)
// ===========================================================================


// ===========================================================================
// Phase 3: CombinedMatcher tests
// ===========================================================================
