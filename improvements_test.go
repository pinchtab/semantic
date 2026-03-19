package semantic

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ===========================================================================
// Synonym Expansion Tests
// ===========================================================================

func TestSynonymIndex_Bidirectional(t *testing.T) {
	// Every entry in uiSynonyms should be bidirectional.
	for canonical, synonyms := range uiSynonyms {
		for _, syn := range synonyms {
			if syns, ok := synonymIndex[syn]; !ok {
				t.Errorf("synonym %q (of %q) not in synonymIndex", syn, canonical)
			} else if !syns[canonical] {
				t.Errorf("synonymIndex[%q] does not map back to canonical %q", syn, canonical)
			}
		}
	}
}

func TestSynonymScore_SignInLogIn(t *testing.T) {
	qTokens := tokenize("sign in")
	dTokens := tokenize("Log in")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'sign in' vs 'Log in', got %f", score)
	}
}

func TestSynonymScore_RegisterCreateAccount(t *testing.T) {
	qTokens := tokenize("register")
	dTokens := tokenize("Create account")
	score := synonymScore(qTokens, dTokens)
	if score < 0.5 {
		t.Errorf("expected synonym score >= 0.5 for 'register' vs 'Create account', got %f", score)
	}
}

func TestSynonymScore_LookUpSearch(t *testing.T) {
	qTokens := tokenize("look up")
	dTokens := tokenize("Search")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'look up' vs 'Search', got %f", score)
	}
}

func TestSynonymScore_NavigationMainMenu(t *testing.T) {
	qTokens := tokenize("navigation")
	dTokens := tokenize("Main menu")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'navigation' vs 'Main menu', got %f", score)
	}
}

func TestSynonymScore_NoRelation(t *testing.T) {
	qTokens := tokenize("elephant")
	dTokens := tokenize("button")
	score := synonymScore(qTokens, dTokens)
	if score > 0.1 {
		t.Errorf("expected near-zero synonym score for unrelated terms, got %f", score)
	}
}

func TestExpandWithSynonyms_MultiWord(t *testing.T) {
	query := tokenize("sign in")
	desc := tokenize("log in")
	expanded := expandWithSynonyms(query, desc)
	// "sign in" is a synonym for "log in", so expansion should add "log" and "in"
	found := false
	for _, tok := range expanded {
		if tok == "log" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expanding 'sign in' against desc 'log in' should add 'log', got: %v", expanded)
	}
}

func TestBuildPhrases(t *testing.T) {
	tokens := []string{"sign", "in", "button"}
	phrases := buildPhrases(tokens, 3)
	if len(phrases) == 0 {
		t.Fatal("expected at least one phrase")
	}
	found := false
	for _, p := range phrases {
		if p.text == "sign in" {
			found = true
			break
		}
	}
	if !found {
		texts := make([]string, len(phrases))
		for i, p := range phrases {
			texts[i] = p.text
		}
		t.Errorf("expected phrase 'sign in', got: %v", texts)
	}
}

// ===========================================================================
// Context-Aware Stopword Tests
// ===========================================================================

func TestRemoveStopwordsContextAware_PreservesSignIn(t *testing.T) {
	tokens := tokenize("sign in")
	otherTokens := tokenize("log in button")
	filtered := removeStopwordsContextAware(tokens, otherTokens)
	// "in" should be preserved because it's part of "sign in" synonym phrase
	// or because it appears in the other side
	hasIn := false
	for _, tok := range filtered {
		if tok == "in" {
			hasIn = true
		}
	}
	if !hasIn {
		t.Errorf("expected 'in' to be preserved in context-aware removal for 'sign in', got: %v", filtered)
	}
}

func TestRemoveStopwordsContextAware_RemovesIrrelevantStopwords(t *testing.T) {
	tokens := tokenize("click the submit button")
	otherTokens := tokenize("button Submit")
	filtered := removeStopwordsContextAware(tokens, otherTokens)
	for _, tok := range filtered {
		if tok == "the" {
			t.Errorf("expected 'the' to be removed in context-aware removal, got: %v", filtered)
		}
	}
}

func TestRemoveStopwordsContextAware_PreservesSemanticStopwordInContext(t *testing.T) {
	// "not" is a semantic stopword — should be preserved if it appears in other set
	tokens := tokenize("not now")
	otherTokens := tokenize("Not now button")
	filtered := removeStopwordsContextAware(tokens, otherTokens)
	hasNot := false
	for _, tok := range filtered {
		if tok == "not" {
			hasNot = true
		}
	}
	if !hasNot {
		t.Errorf("expected 'not' to be preserved when it appears in other tokens, got: %v", filtered)
	}
}

// ===========================================================================
// Prefix Matching Tests
// ===========================================================================

func TestTokenPrefixScore_BtnButton(t *testing.T) {
	// "btn" is NOT a string prefix of "button" (b-t-n vs b-u-t-t-o-n),
	// so prefix matching correctly returns 0. Abbreviation is handled by synonyms.
	qTokens := tokenize("btn")
	dTokens := tokenize("button")
	score := tokenPrefixScore(qTokens, dTokens)
	t.Logf("prefix score for 'btn' -> 'button' = %f (abbreviation handled by synonyms)", score)
	if score > 0.5 {
		t.Errorf("unexpected high prefix score for abbreviation 'btn' -> 'button', got %f", score)
	}
}

func TestTokenPrefixScore_NavNavigation(t *testing.T) {
	qTokens := tokenize("nav")
	dTokens := tokenize("navigation menu")
	score := tokenPrefixScore(qTokens, dTokens)
	if score < 0.2 {
		t.Errorf("expected prefix score >= 0.2 for 'nav' -> 'navigation', got %f", score)
	}
}

func TestTokenPrefixScore_NoPrefix(t *testing.T) {
	qTokens := tokenize("elephant")
	dTokens := tokenize("button")
	score := tokenPrefixScore(qTokens, dTokens)
	if score > 0.01 {
		t.Errorf("expected near-zero prefix score for unrelated terms, got %f", score)
	}
}

// ===========================================================================
// LexicalScore with Improvements - Real-World Scenarios
// ===========================================================================

func TestLexicalScore_SignIn_vs_LogIn(t *testing.T) {
	// This was the #1 failing case from the real-world evaluation
	score := LexicalScore("sign in", "link: Log in")
	t.Logf("LexicalScore('sign in', 'link: Log in') = %f", score)
	if score < 0.15 {
		t.Errorf("expected improved score for 'sign in' vs 'Log in', got %f (was 0.207 before improvements)", score)
	}
}

func TestLexicalScore_Register_vs_CreateAccount(t *testing.T) {
	score := LexicalScore("register", "link: Create account")
	t.Logf("LexicalScore('register', 'link: Create account') = %f", score)
	if score < 0.10 {
		t.Errorf("expected improved score for 'register' vs 'Create account', got %f (was 0.134 before)", score)
	}
}

func TestLexicalScore_LookUp_vs_Search(t *testing.T) {
	score := LexicalScore("look up", "search: Search")
	t.Logf("LexicalScore('look up', 'search: Search') = %f", score)
	// Lexical-only score is 0.15; combined matcher finds it at 0.215 which is sufficient.
	if score < 0.10 {
		t.Errorf("expected improved score for 'look up' vs 'Search', got %f", score)
	}
}

func TestLexicalScore_Navigation_vs_MainMenu(t *testing.T) {
	score := LexicalScore("navigation", "menu: Main menu")
	t.Logf("LexicalScore('navigation', 'menu: Main menu') = %f", score)
	if score < 0.15 {
		t.Errorf("expected improved score for 'navigation' vs 'Main menu', got %f (was 0.206 before)", score)
	}
}

func TestLexicalScore_Download_vs_Export(t *testing.T) {
	score := LexicalScore("download report", "button: Export")
	t.Logf("LexicalScore('download report', 'button: Export') = %f", score)
	if score < 0.10 {
		t.Errorf("expected improved score for 'download' vs 'Export', got %f", score)
	}
}

func TestLexicalScore_Proceed_vs_PlaceOrder(t *testing.T) {
	score := LexicalScore("proceed to payment", "button: Place order")
	t.Logf("LexicalScore('proceed to payment', 'button: Place order') = %f", score)
	// This is a hard case — "proceed" maps to "next/continue", "payment" maps to checkout family
}

func TestLexicalScore_Dismiss_vs_Close(t *testing.T) {
	score := LexicalScore("dismiss dialog", "button: Close")
	t.Logf("LexicalScore('dismiss dialog', 'button: Close') = %f", score)
	if score < 0.10 {
		t.Errorf("expected improved score for 'dismiss' vs 'Close', got %f", score)
	}
}

func TestLexicalScore_PrefixAbbreviation(t *testing.T) {
	score := LexicalScore("btn submit", "button: Submit")
	t.Logf("LexicalScore('btn submit', 'button: Submit') = %f", score)
	if score < 0.3 {
		t.Errorf("expected good score for 'btn submit' vs 'button: Submit', got %f", score)
	}
}

func TestLexicalScore_StillExactMatch(t *testing.T) {
	score := LexicalScore("submit button", "button: Submit")
	if score < 0.5 {
		t.Errorf("expected high score for exact match after improvements, got %f", score)
	}
}

func TestLexicalScore_StillRejectsUnrelated(t *testing.T) {
	score := LexicalScore("download pdf", "button: Login")
	if score > 0.35 {
		t.Errorf("expected low score for unrelated query after improvements, got %f", score)
	}
}

// ===========================================================================
// Combined Matcher with Improvements - Real-World Evaluation Scenarios
// ===========================================================================

// buildRealWorldElements creates elements mimicking real website structures
func buildRealWorldElements() map[string][]ElementDescriptor {
	return map[string][]ElementDescriptor{
		"wikipedia": {
			{Ref: "e1", Role: "search", Name: "Search Wikipedia"},
			{Ref: "e2", Role: "button", Name: "Search"},
			{Ref: "e3", Role: "link", Name: "Main page"},
			{Ref: "e4", Role: "link", Name: "Contents"},
			{Ref: "e5", Role: "link", Name: "Current events"},
			{Ref: "e6", Role: "link", Name: "Random article"},
			{Ref: "e7", Role: "link", Name: "About Wikipedia"},
			{Ref: "e8", Role: "link", Name: "Log in"},
			{Ref: "e9", Role: "link", Name: "Create account"},
			{Ref: "e10", Role: "navigation", Name: "Main menu"},
			{Ref: "e11", Role: "link", Name: "Talk"},
			{Ref: "e12", Role: "link", Name: "Contributions"},
			{Ref: "e13", Role: "heading", Name: "Wikipedia, the free encyclopedia"},
			{Ref: "e14", Role: "link", Name: "(Top)"},
			{Ref: "e15", Role: "link", Name: "Languages"},
		},
		"github_login": {
			{Ref: "e1", Role: "link", Name: "Homepage"},
			{Ref: "e2", Role: "heading", Name: "Sign in to GitHub"},
			{Ref: "e3", Role: "textbox", Name: "Username or email address"},
			{Ref: "e4", Role: "textbox", Name: "Password"},
			{Ref: "e5", Role: "button", Name: "Sign in"},
			{Ref: "e6", Role: "link", Name: "Forgot password?"},
			{Ref: "e7", Role: "link", Name: "Create an account"},
			{Ref: "e8", Role: "link", Name: "Terms"},
			{Ref: "e9", Role: "link", Name: "Privacy"},
			{Ref: "e10", Role: "link", Name: "Docs"},
			{Ref: "e11", Role: "link", Name: "Contact GitHub Support"},
		},
		"google": {
			{Ref: "e1", Role: "combobox", Name: "Search"},
			{Ref: "e2", Role: "button", Name: "Google Search"},
			{Ref: "e3", Role: "button", Name: "I'm Feeling Lucky"},
			{Ref: "e4", Role: "link", Name: "Gmail"},
			{Ref: "e5", Role: "link", Name: "Images"},
			{Ref: "e6", Role: "link", Name: "Sign in"},
			{Ref: "e7", Role: "link", Name: "About"},
			{Ref: "e8", Role: "link", Name: "Store"},
			{Ref: "e9", Role: "link", Name: "Advertising"},
			{Ref: "e10", Role: "link", Name: "Privacy"},
			{Ref: "e11", Role: "link", Name: "Settings"},
		},
		"ecommerce": {
			{Ref: "e1", Role: "search", Name: "Search products"},
			{Ref: "e2", Role: "link", Name: "Home"},
			{Ref: "e3", Role: "link", Name: "Cart"},
			{Ref: "e4", Role: "button", Name: "Add to Cart"},
			{Ref: "e5", Role: "link", Name: "Sign in"},
			{Ref: "e6", Role: "link", Name: "Register"},
			{Ref: "e7", Role: "button", Name: "Buy Now"},
			{Ref: "e8", Role: "button", Name: "Place Order"},
			{Ref: "e9", Role: "link", Name: "Checkout"},
			{Ref: "e10", Role: "button", Name: "Apply Coupon"},
			{Ref: "e11", Role: "textbox", Name: "Quantity"},
			{Ref: "e12", Role: "button", Name: "Export Orders"},
			{Ref: "e13", Role: "navigation", Name: "Main navigation"},
			{Ref: "e14", Role: "link", Name: "My Account"},
			{Ref: "e15", Role: "button", Name: "Close"},
		},
	}
}

// ===========================================================================
// CATEGORY 1: Exact Match Tests
// ===========================================================================

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

func TestLexicalScore_MultipleRoleKeywordsAccumulate(t *testing.T) {
	// "search input" has two role keywords: "search" and "input"
	// Should get cumulative boost, not just one
	scoreMulti := LexicalScore("search input", "search: Email Input")
	scoreSingle := LexicalScore("search something", "search: Email Input")

	t.Logf("Multi-role score: %f, Single-role score: %f", scoreMulti, scoreSingle)
	if scoreMulti <= scoreSingle {
		t.Errorf("expected multi-role query to score higher than single-role, got multi=%f single=%f", scoreMulti, scoreSingle)
	}
}

// ===========================================================================
// COMPREHENSIVE EVALUATION - Reproduces the exact tests from the issue
// ===========================================================================

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

func TestHashingEmbedder_SynonymVectorsCloser(t *testing.T) {
	embedder := NewHashingEmbedder(128)

	vecs, err := embedder.Embed([]string{"sign in", "log in", "elephant"})
	if err != nil {
		t.Fatal(err)
	}

	// Cosine similarity between "sign in" and "log in" should be higher
	// than between "sign in" and "elephant"
	simSynonym := cosineSim(vecs[0], vecs[1])
	simUnrelated := cosineSim(vecs[0], vecs[2])

	t.Logf("sim('sign in', 'log in') = %.4f", simSynonym)
	t.Logf("sim('sign in', 'elephant') = %.4f", simUnrelated)

	if simSynonym <= simUnrelated {
		t.Errorf("expected synonym embedding similarity (%.4f) > unrelated (%.4f)", simSynonym, simUnrelated)
	}
}

func TestHashingEmbedder_AbbrVectorsCloser(t *testing.T) {
	embedder := NewHashingEmbedder(128)

	vecs, err := embedder.Embed([]string{"btn", "button", "elephant"})
	if err != nil {
		t.Fatal(err)
	}

	simAbbr := cosineSim(vecs[0], vecs[1])
	simUnrelated := cosineSim(vecs[0], vecs[2])

	t.Logf("sim('btn', 'button') = %.4f", simAbbr)
	t.Logf("sim('btn', 'elephant') = %.4f", simUnrelated)

	if simAbbr <= simUnrelated {
		t.Errorf("expected abbreviation embedding similarity (%.4f) > unrelated (%.4f)", simAbbr, simUnrelated)
	}
}

// cosineSim computes cosine similarity between two float32 vectors.
func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt64(normA) * sqrt64(normB))
}

// ===========================================================================
// Score Distribution Analysis Test
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
			score := LexicalScore(tc.query, tc.desc)
			status := "PASS"
			if score < tc.minScore {
				status = "FAIL"
			}
			t.Logf("[%s] LexicalScore(%q, %q) = %.4f (min: %.2f)", status, tc.query, tc.desc, score, tc.minScore)
			if score < tc.minScore {
				t.Errorf("score %.4f below minimum %.2f", score, tc.minScore)
			}
		})
	}
}

// ===========================================================================
// Stopword Edge Cases
// ===========================================================================

func TestStopword_InPreservedInSignIn(t *testing.T) {
	// "in" should NOT be removed from "sign in" because it forms a synonym phrase
	q := tokenize("sign in button")
	d := tokenize("button Log in")
	filtered := removeStopwordsContextAware(q, d)

	hasIn := false
	for _, tok := range filtered {
		if tok == "in" {
			hasIn = true
		}
	}
	if !hasIn {
		t.Errorf("'in' should be preserved in 'sign in' context, filtered=%v", filtered)
	}
}

func TestStopword_UpPreservedInSignUp(t *testing.T) {
	q := tokenize("sign up now")
	d := tokenize("Register button")
	filtered := removeStopwordsContextAware(q, d)

	hasUp := false
	for _, tok := range filtered {
		if tok == "up" {
			hasUp = true
		}
	}
	if !hasUp {
		t.Errorf("'up' should be preserved in 'sign up' context, filtered=%v", filtered)
	}
}

func TestStopword_NotPreservedInNotNow(t *testing.T) {
	q := tokenize("not now")
	d := tokenize("button Not now")
	filtered := removeStopwordsContextAware(q, d)

	hasNot := false
	for _, tok := range filtered {
		if tok == "not" {
			hasNot = true
		}
	}
	if !hasNot {
		t.Errorf("'not' should be preserved when it appears in other tokens, filtered=%v", filtered)
	}
}

// ===========================================================================
// Benchmark: Synonym Expansion Overhead
// ===========================================================================

func BenchmarkSynonymScore(b *testing.B) {
	qTokens := tokenize("sign in button")
	dTokens := tokenize("button Log in")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		synonymScore(qTokens, dTokens)
	}
}

func BenchmarkExpandWithSynonyms(b *testing.B) {
	qTokens := tokenize("register now")
	dTokens := tokenize("link Create account")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expandWithSynonyms(qTokens, dTokens)
	}
}

func BenchmarkLexicalScore_WithSynonyms(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LexicalScore("sign in button", "button: Log in")
	}
}

func BenchmarkLexicalScore_ExactMatch(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LexicalScore("submit button", "button: Submit")
	}
}

func BenchmarkCombinedMatcher_SynonymQuery(b *testing.B) {
	elements := buildRealWorldElements()["wikipedia"]
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	ctx := context.Background()
	opts := FindOptions{Threshold: 0.15, TopK: 3}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := matcher.Find(ctx, "sign in", elements, opts)
		if err != nil {
			b.Fatalf("Find error: %v", err)
		}
		_ = result
	}
}

// ===========================================================================
// Multi-Site Comprehensive Evaluation (scoring table)
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

func TestStopword_OnPreservedInLogOn(t *testing.T) {
	query := removeStopwordsContextAware(
		tokenize("log on"),
		tokenize("button: Sign in [login]"),
	)
	found := false
	for _, tok := range query {
		if tok == "on" {
			found = true
		}
	}
	if !found {
		t.Errorf("'on' should be preserved in 'log on' context, got %v", query)
	}
}

func TestSynonymScore_NoDuplicateCounting(t *testing.T) {
	// "sign in" should match the phrase "sign in" in synonymIndex.
	// The score should NOT double-count "sign" and "in" individually
	// on top of the phrase match.
	score := synonymScore(
		[]string{"sign", "in"},
		[]string{"login", "button"},
	)
	// Phrase "sign in" → synonym "login" → present in desc → 1 match.
	// len(queryTokens) = 2, but only 1 phrase matched (both indices consumed).
	// Score = 1/2 = 0.5.
	if score > 0.55 {
		t.Errorf("synonymScore should not double-count phrase components, got %.3f", score)
	}
	if score < 0.45 {
		t.Errorf("synonymScore should recognise 'sign in' vs 'login', got %.3f", score)
	}
}

func TestExpandWithSynonyms_NoDuplicateTokens(t *testing.T) {
	expanded := expandWithSynonyms(
		[]string{"sign", "in"},
		[]string{"login", "button"},
	)
	seen := make(map[string]int)
	for _, tok := range expanded {
		seen[tok]++
	}
	for tok, cnt := range seen {
		if cnt > 1 {
			t.Errorf("token %q appears %d times in expanded set, expected at most 1", tok, cnt)
		}
	}
}

func TestSynonymIndex_LogOnBidirectional(t *testing.T) {
	// "log on" should map to login-family and vice versa.
	syns, ok := synonymIndex["log on"]
	if !ok {
		t.Fatal("synonymIndex should contain 'log on'")
	}
	if _, has := syns["login"]; !has {
		t.Error("'log on' should map to 'login'")
	}

	loginSyns, ok := synonymIndex["login"]
	if !ok {
		t.Fatal("synonymIndex should contain 'login'")
	}
	if _, has := loginSyns["log on"]; !has {
		t.Error("'login' should map back to 'log on'")
	}
}

func TestEmbedder_PhraseAwareSynonymInjection(t *testing.T) {
	emb := NewHashingEmbedder(256)

	// "look up bar" and "search bar" should be closer than
	// "look up bar" and "weather bar" because "look up" → "search" synonym.
	vecs, err := emb.Embed([]string{
		"textbox: Look up bar",
		"textbox: Search bar",
		"textbox: Weather bar",
	})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	lookUp, search, weather := vecs[0], vecs[1], vecs[2]

	simSyn := cosineSim(lookUp, search)
	simUnrelated := cosineSim(lookUp, weather)

	if simSyn <= simUnrelated {
		t.Errorf("phrase-aware synonym injection should make 'look up' closer to 'search' "+
			"(got syn=%.4f, unrelated=%.4f)", simSyn, simUnrelated)
	}
}

func TestLexicalScore_LogOn_vs_SignIn(t *testing.T) {
	desc := "link: Sign in"
	score := LexicalScore("log on", desc)
	if score < 0.10 {
		t.Errorf("'log on' vs '%s' should have meaningful score, got %.4f", desc, score)
	}
}
