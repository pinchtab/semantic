package semantic

import (
	"context"
	"fmt"
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


// ===========================================================================
// Phase 3: CombinedMatcher tests
// ===========================================================================

func TestCombinedMatcher_Strategy(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	want := "combined:lexical+embedding:hashing"
	if m.Strategy() != want {
		t.Errorf("expected strategy=%s, got %s", want, m.Strategy())
	}
}

func TestCombinedMatcher_Find(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
		{Ref: "e2", Role: "textbox", Name: "Email Address"},
	}

	result, err := m.Find(context.Background(), "log in button", elements, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.ElementCount != 3 {
		t.Errorf("expected ElementCount=3, got %d", result.ElementCount)
	}
	if result.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", result.BestRef)
	}
	if result.BestScore <= 0 {
		t.Errorf("expected positive BestScore, got %f", result.BestScore)
	}
	if result.Strategy != "combined:lexical+embedding:hashing" {
		t.Errorf("expected combined strategy, got %s", result.Strategy)
	}
}

func TestCombinedMatcher_ThresholdFiltering(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Home"},
	}

	result, err := m.Find(context.Background(), "submit button", elements, FindOptions{
		Threshold: 0.99,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for _, match := range result.Matches {
		if match.Score < 0.99 {
			t.Errorf("match %s has score %f below threshold", match.Ref, match.Score)
		}
	}
}

func TestCombinedMatcher_TopK(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "button", Name: "Cancel"},
		{Ref: "e2", Role: "button", Name: "Reset"},
		{Ref: "e3", Role: "link", Name: "Home"},
		{Ref: "e4", Role: "textbox", Name: "Name"},
	}

	result, err := m.Find(context.Background(), "button", elements, FindOptions{
		Threshold: 0.01,
		TopK:      2,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Matches) > 2 {
		t.Errorf("expected at most 2 matches (TopK=2), got %d", len(result.Matches))
	}
}

func TestCombinedMatcher_ScoresDescending(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Login"},
		{Ref: "e1", Role: "textbox", Name: "Username"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
		{Ref: "e3", Role: "heading", Name: "Welcome Page"},
	}

	result, err := m.Find(context.Background(), "login button", elements, FindOptions{
		Threshold: 0.01,
		TopK:      10,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for i := 1; i < len(result.Matches); i++ {
		if result.Matches[i].Score > result.Matches[i-1].Score {
			t.Errorf("matches not sorted descending: [%d]=%f > [%d]=%f",
				i, result.Matches[i].Score, i-1, result.Matches[i-1].Score)
		}
	}
}

func TestCombinedMatcher_FusesBothStrategies(t *testing.T) {
	combined := NewCombinedMatcher(NewHashingEmbedder(128))
	lexical := NewLexicalMatcher()
	embedding := NewEmbeddingMatcher(NewHashingEmbedder(128))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
	}

	ctx := context.Background()
	opts := FindOptions{Threshold: 0.01, TopK: 3}

	combResult, err := combined.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Combined.Find error: %v", err)
	}
	lexResult, err := lexical.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Lexical.Find error: %v", err)
	}
	embResult, err := embedding.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Embedding.Find error: %v", err)
	}

	// Combined score should differ from both pure strategies (it's a weighted fusion).
	if combResult.BestScore == lexResult.BestScore && combResult.BestScore == embResult.BestScore {
		t.Errorf("combined score (%.4f) identical to both lexical (%.4f) and embedding (%.4f) — fusion not working",
			combResult.BestScore, lexResult.BestScore, embResult.BestScore)
	}

	// All three should identify the same best ref.
	if combResult.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", combResult.BestRef)
	}
}

func TestCombinedMatcher_NoElements(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := m.Find(context.Background(), "anything", nil, FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected no matches for empty elements, got %d", len(result.Matches))
	}
	if result.BestRef != "" {
		t.Errorf("expected empty BestRef, got %s", result.BestRef)
	}
}

// ===========================================================================
// Phase 3: Complex UI test scenarios
// ===========================================================================

// complexFormElements returns a realistic form page with 15+ elements.
func complexFormElements() []ElementDescriptor {
	return []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Registration Form"},
		{Ref: "e1", Role: "textbox", Name: "First Name"},
		{Ref: "e2", Role: "textbox", Name: "Last Name"},
		{Ref: "e3", Role: "textbox", Name: "Email Address"},
		{Ref: "e4", Role: "textbox", Name: "Password", Value: ""},
		{Ref: "e5", Role: "textbox", Name: "Confirm Password"},
		{Ref: "e6", Role: "combobox", Name: "Country"},
		{Ref: "e7", Role: "checkbox", Name: "I agree to the Terms of Service"},
		{Ref: "e8", Role: "checkbox", Name: "Subscribe to newsletter"},
		{Ref: "e9", Role: "button", Name: "Submit Registration"},
		{Ref: "e10", Role: "button", Name: "Cancel"},
		{Ref: "e11", Role: "link", Name: "Already have an account? Log in"},
		{Ref: "e12", Role: "link", Name: "Privacy Policy"},
		{Ref: "e13", Role: "link", Name: "Terms of Service"},
		{Ref: "e14", Role: "img", Name: "Company Logo"},
		{Ref: "e15", Role: "navigation", Name: "Main Navigation"},
	}
}

// complexTableElements returns a data table with columns and actions.
func complexTableElements() []ElementDescriptor {
	return []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "User Management"},
		{Ref: "e1", Role: "search", Name: "Search Users"},
		{Ref: "e2", Role: "button", Name: "Add New User"},
		{Ref: "e3", Role: "button", Name: "Export CSV"},
		{Ref: "e4", Role: "table", Name: "Users Table"},
		{Ref: "e5", Role: "columnheader", Name: "Name"},
		{Ref: "e6", Role: "columnheader", Name: "Email"},
		{Ref: "e7", Role: "columnheader", Name: "Role"},
		{Ref: "e8", Role: "columnheader", Name: "Status"},
		{Ref: "e9", Role: "columnheader", Name: "Actions"},
		{Ref: "e10", Role: "cell", Name: "John Doe", Value: "john@pinchtab.com"},
		{Ref: "e11", Role: "button", Name: "Edit", Value: "John Doe"},
		{Ref: "e12", Role: "button", Name: "Delete", Value: "John Doe"},
		{Ref: "e13", Role: "cell", Name: "Jane Smith", Value: "jane@pinchtab.com"},
		{Ref: "e14", Role: "button", Name: "Edit", Value: "Jane Smith"},
		{Ref: "e15", Role: "button", Name: "Delete", Value: "Jane Smith"},
		{Ref: "e16", Role: "button", Name: "Previous Page"},
		{Ref: "e17", Role: "button", Name: "Next Page"},
		{Ref: "e18", Role: "combobox", Name: "Rows per page", Value: "10"},
	}
}

// complexModalElements returns a page with a modal dialog overlay.
func complexModalElements() []ElementDescriptor {
	return []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Dashboard"},
		{Ref: "e1", Role: "button", Name: "Settings"},
		{Ref: "e2", Role: "button", Name: "Notifications"},
		{Ref: "e3", Role: "dialog", Name: "Confirm Delete"},
		{Ref: "e4", Role: "heading", Name: "Are you sure?"},
		{Ref: "e5", Role: "text", Name: "This action cannot be undone. The item will be permanently deleted."},
		{Ref: "e6", Role: "button", Name: "Yes, Delete"},
		{Ref: "e7", Role: "button", Name: "Cancel"},
		{Ref: "e8", Role: "button", Name: "Close Dialog"},
		{Ref: "e9", Role: "navigation", Name: "Sidebar Menu"},
		{Ref: "e10", Role: "link", Name: "Home"},
		{Ref: "e11", Role: "link", Name: "Reports"},
		{Ref: "e12", Role: "link", Name: "Settings"},
	}
}

func TestCombinedMatcher_ComplexForm(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"submit registration", "e9", "should find the submit button"},
		{"email field", "e3", "should find email textbox"},
		{"terms checkbox", "e7", "should find terms of service checkbox"},
		{"password input", "e4", "should find password field"},
		{"cancel button", "e10", "should find cancel button"},
		{"log in link", "e11", "should find the login link"},
		{"country dropdown", "e6", "should find country combobox"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

func TestCombinedMatcher_ComplexTable(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexTableElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"search users", "e1", "should find the search box"},
		{"add new user", "e2", "should find the add button"},
		{"export csv", "e3", "should find the export button"},
		{"next page", "e17", "should find the next page button"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

func TestCombinedMatcher_ComplexModal(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexModalElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"delete button", "e6", "should find the yes delete button in modal"},
		{"close dialog", "e8", "should find the close dialog button"},
		{"cancel", "e7", "should find the cancel button in modal"},
		{"settings button", "e1", "should find the settings button"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

// ===========================================================================
// Phase 3: Benchmark tests
// ===========================================================================

func BenchmarkLexicalMatcher_Find(b *testing.B) {
	m := NewLexicalMatcher()
	elements := complexFormElements()
	opts := FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkHashingEmbedder_Embed(b *testing.B) {
	e := NewHashingEmbedder(128)
	texts := []string{"submit registration button"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Embed(texts)
	}
}

func BenchmarkHashingEmbedder_EmbedBatch(b *testing.B) {
	e := NewHashingEmbedder(128)
	elements := complexFormElements()
	texts := make([]string, len(elements))
	for i, el := range elements {
		texts[i] = el.Composite()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Embed(texts)
	}
}

func BenchmarkEmbeddingMatcher_Find(b *testing.B) {
	m := NewEmbeddingMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()
	opts := FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkCombinedMatcher_Find(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()
	opts := FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkCombinedMatcher_LargeElementSet(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	// Build a large element set (100 elements) simulating a complex page.
	elements := make([]ElementDescriptor, 100)
	roles := []string{"button", "link", "textbox", "heading", "img", "checkbox", "combobox"}
	for i := 0; i < 100; i++ {
		elements[i] = ElementDescriptor{
			Ref:  fmt.Sprintf("e%d", i),
			Role: roles[i%len(roles)],
			Name: fmt.Sprintf("Element %d action item", i),
		}
	}
	opts := FindOptions{Threshold: 0.1, TopK: 5}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "click the action button number 42", elements, opts)
	}
}
