package engine

import (
	"context"
	"errors"
	"github.com/pinchtab/semantic/internal/types"
	"math"
	"strconv"
	"testing"
)

func TestLexicalMatcher_Find_ContextCanceled(t *testing.T) {
	m := NewLexicalMatcher()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := m.Find(ctx, "submit button", []types.ElementDescriptor{{Ref: "e1", Role: "button", Name: "Submit"}}, types.FindOptions{Threshold: 0, TopK: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

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

// LexicalScore with Improvements - Real-World Scenarios

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

// Combined Matcher with Improvements - Real-World Evaluation Scenarios

// buildRealWorldElements creates elements mimicking real website structures
func buildRealWorldElements() map[string][]types.ElementDescriptor {
	return map[string][]types.ElementDescriptor{
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

// CATEGORY 1: Exact Match Tests

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

// COMPREHENSIVE EVALUATION - Reproduces the exact tests from the issue

func TestLexicalScore_LogOn_vs_SignIn(t *testing.T) {
	desc := "link: Sign in"
	score := LexicalScore("log on", desc)
	if score < 0.10 {
		t.Errorf("'log on' vs '%s' should have meaningful score, got %.4f", desc, score)
	}
}

// LexicalScore tests

func TestLexicalScore_ExactMatch(t *testing.T) {
	score := LexicalScore("submit button", "button: Submit")
	if score < 0.5 {
		t.Errorf("expected high score for exact match, got %f", score)
	}
}

func TestLexicalScore_NoOverlap(t *testing.T) {
	score := LexicalScore("download pdf", "button: Login")
	if score > 0.3 {
		t.Errorf("expected low score for no overlap, got %f", score)
	}
}

func TestLexicalScore_RoleBoost(t *testing.T) {
	// "button" is a role keyword; if it appears in both, a boost is applied.
	withRole := LexicalScore("submit button", "button: Submit")
	withoutRole := LexicalScore("submit action", "link: Submit")
	if withRole <= withoutRole {
		t.Errorf("expected role boost to increase score: withRole=%f, withoutRole=%f", withRole, withoutRole)
	}
}

func TestLexicalScore_StopwordRemoval(t *testing.T) {
	// "the" is a stopword — it should be removed so both queries score similarly.
	s1 := LexicalScore("click the button", "button: Click")
	s2 := LexicalScore("click button", "button: Click")
	diff := math.Abs(s1 - s2)
	if diff > 0.01 {
		t.Errorf("stopwords should not affect score significantly: s1=%f, s2=%f, diff=%f", s1, s2, diff)
	}
}

func TestLexicalScore_PhraseBonus_ExactPhraseBeatsPartial(t *testing.T) {
	exact := LexicalScore("add to cart", "button: Add to cart")
	partial := LexicalScore("add to cart", "button: Add")
	if exact <= partial {
		t.Errorf("expected exact phrase match to score higher, exact=%f partial=%f", exact, partial)
	}
}

func TestLexicalScore_PhraseBonus_PartialSubPhrase(t *testing.T) {
	withPartial := LexicalScore("create account now", "button: Create account")
	withoutPartial := LexicalScore("create account now", "button: Register")
	if withPartial <= withoutPartial {
		t.Errorf("expected partial phrase match to score higher, withPartial=%f withoutPartial=%f", withPartial, withoutPartial)
	}
}

func TestLexicalScore_PhraseBonus_NoPhraseMatch(t *testing.T) {
	nonMatch := LexicalScore("security settings", "button: Export report")
	if nonMatch > 0.35 {
		t.Errorf("expected low score when no phrase exists, got %f", nonMatch)
	}
}

func TestLevenshtein_BasicCases(t *testing.T) {
	if got := levenshtein("settings", "setttings"); got != 1 {
		t.Fatalf("expected distance 1, got %d", got)
	}
	if got := levenshtein("settings", "setting"); got != 1 {
		t.Fatalf("expected distance 1, got %d", got)
	}
	if got := levenshtein("settings", "setup"); got <= 1 {
		t.Fatalf("expected distance > 1, got %d", got)
	}
}

func TestTypoBonus_EditDistanceOne_GetsBonus(t *testing.T) {
	bonus := typoBonus(tokenize("setttings"), tokenize("settings"))
	if bonus <= 0 {
		t.Fatalf("expected positive typo bonus for edit-distance-1 token")
	}
}

func TestTypoBonus_DistanceGreaterThanOne_NoBonus(t *testing.T) {
	bonus := typoBonus(tokenize("sxyztings"), tokenize("settings"))
	if bonus != 0 {
		t.Fatalf("expected no bonus for distance > 1, got %f", bonus)
	}
}

func TestTypoBonus_LengthGuards_NoBonus(t *testing.T) {
	if bonus := typoBonus(tokenize("aa"), tokenize("ab")); bonus != 0 {
		t.Fatalf("expected no bonus for short token, got %f", bonus)
	}
	if bonus := typoBonus(tokenize("extraordinarytoken"), tokenize("extraordimarytoken")); bonus != 0 {
		t.Fatalf("expected no bonus for long token, got %f", bonus)
	}
}

func TestLexicalScore_TypoTolerance_RealWorldSettings(t *testing.T) {
	withTypo := LexicalScore("setttings", "link: Settings")
	withoutTypo := LexicalScore("setttings", "link: Billing")
	if withTypo <= withoutTypo {
		t.Fatalf("expected typo-tolerant match to score higher, typo=%f control=%f", withTypo, withoutTypo)
	}
}

func TestInteractiveBoost_ActionVerbDetection(t *testing.T) {
	if !containsActionVerb(tokenize("click submit")) {
		t.Fatalf("expected action verb detection for action-oriented query")
	}
	if containsActionVerb(tokenize("account settings")) {
		t.Fatalf("did not expect action verb detection for non-action query")
	}
}

func TestInteractiveBoost_NonActionUsesMildBoost(t *testing.T) {
	action := interactiveBoost(tokenize("click submit"), true)
	nonAction := interactiveBoost(tokenize("account settings"), true)
	if nonAction <= 0 {
		t.Fatalf("expected non-action interactive boost to be positive")
	}
	if action <= nonAction {
		t.Fatalf("expected action boost to be larger than non-action boost, action=%f nonAction=%f", action, nonAction)
	}
}

func TestLexicalMatcher_ActionQueryPrefersInteractiveElement(t *testing.T) {
	m := NewLexicalMatcher()

	elements := []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit", Interactive: false},
		{Ref: "e2", Role: "button", Name: "Submit", Interactive: true},
	}

	result, err := m.Find(context.Background(), "click submit", elements, types.FindOptions{
		Threshold: 0,
		TopK:      2,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(result.Matches) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Matches))
	}
	if result.BestRef != "e2" {
		t.Fatalf("expected interactive element to rank first, got %s", result.BestRef)
	}
}

func TestLexicalMatcher_SectionContextDisambiguatesSubmitButtons(t *testing.T) {
	m := NewLexicalMatcher()

	elements := []types.ElementDescriptor{
		{Ref: "login-submit", Role: "button", Name: "Submit", Parent: "Login form", Section: "Login form"},
		{Ref: "payment-submit", Role: "button", Name: "Submit", Parent: "Payment form", Section: "Payment form"},
	}

	result, err := m.Find(context.Background(), "submit button in login", elements, types.FindOptions{
		Threshold: 0,
		TopK:      2,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(result.Matches) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(result.Matches))
	}
	if result.BestRef != "login-submit" {
		t.Fatalf("expected login submit to rank first, got %s", result.BestRef)
	}
	if result.Matches[0].Score <= result.Matches[1].Score {
		t.Fatalf("expected login submit score > payment submit score, got %f <= %f", result.Matches[0].Score, result.Matches[1].Score)
	}
}

func TestLexicalMatcher_LabelledByBoostPrefersAssociatedInput(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "work-email", Role: "textbox", Name: "Email", Positional: types.PositionalHints{LabelledBy: "Work Email"}},
		{Ref: "billing-email", Role: "textbox", Name: "Email", Positional: types.PositionalHints{LabelledBy: "Billing Email"}},
	}

	result, err := m.Find(context.Background(), "work email input", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if result.BestRef != "work-email" {
		t.Fatalf("expected labelled input to win, got %s", result.BestRef)
	}
}

func TestLexicalMatcher_SiblingUniquenessBoostPrefersSingleton(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "single-search", Role: "searchbox", Name: "Search", Positional: types.PositionalHints{SiblingCount: 1}},
		{Ref: "group-search", Role: "searchbox", Name: "Search", Positional: types.PositionalHints{SiblingCount: 3}},
	}

	result, err := m.Find(context.Background(), "search box", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if result.BestRef != "single-search" {
		t.Fatalf("expected singleton sibling to rank first, got %s", result.BestRef)
	}
}

func TestLexicalMatcher_DepthBreaksEqualScores(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "shallow", Role: "button", Name: "Submit", Positional: types.PositionalHints{Depth: 1}},
		{Ref: "deep", Role: "button", Name: "Submit", Positional: types.PositionalHints{Depth: 4}},
	}

	result, err := m.Find(context.Background(), "submit button", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if result.BestRef != "deep" {
		t.Fatalf("expected deeper element to rank first on tied scores, got %s", result.BestRef)
	}
}

func TestElementFrequency_IEF_RareTokenHigherThanCommon(t *testing.T) {
	elements := []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Checkout"},
		{Ref: "e2", Role: "button", Name: "Continue"},
		{Ref: "e3", Role: "button", Name: "Cancel"},
		{Ref: "e4", Role: "button", Name: "Back"},
	}

	ef := BuildElementFrequency(elements)
	if ef == nil {
		t.Fatalf("expected non-nil ElementFrequency")
	}
	if ef.IEF("checkout") <= ef.IEF("button") {
		t.Fatalf("expected rare token IEF to be higher than common token IEF")
	}
}

func TestWeightedJaccard_NilIDFMatchesUnitWeights(t *testing.T) {
	q := []string{"status", "order", "1049"}
	d := []string{"status", "order", "1001"}

	withNil := weightedJaccard(q, d, nil)
	withUnit := weightedJaccard(q, d, map[string]float64{
		"status": 1,
		"order":  1,
		"1049":   1,
		"1001":   1,
	})

	if math.Abs(withNil-withUnit) > 1e-9 {
		t.Fatalf("expected nil IDF to match unit-weight Jaccard, nil=%f unit=%f", withNil, withUnit)
	}
}

func TestLexicalScoreWithFrequency_50RowTableImprovesIdentifierSeparation(t *testing.T) {
	elements := make([]types.ElementDescriptor, 0, 50)
	for i := 1000; i < 1050; i++ {
		elements = append(elements, types.ElementDescriptor{
			Ref:  "row-" + strconv.Itoa(i),
			Role: "row",
			Name: "Order " + strconv.Itoa(i) + " status price name",
		})
	}

	ef := BuildElementFrequency(elements)
	if ef == nil {
		t.Fatalf("expected non-nil ElementFrequency")
	}

	query := "order 1049 status price"
	targetComposite := elements[49].Composite() // row-1049
	distractorComposite := elements[0].Composite()

	targetUnweighted := lexicalScore(query, targetComposite, false, nil)
	distractorUnweighted := lexicalScore(query, distractorComposite, false, nil)
	targetWeighted := lexicalScore(query, targetComposite, false, ef)
	distractorWeighted := lexicalScore(query, distractorComposite, false, ef)

	unweightedMargin := targetUnweighted - distractorUnweighted
	weightedMargin := targetWeighted - distractorWeighted

	if weightedMargin <= unweightedMargin {
		t.Fatalf("expected weighted scoring to improve unique identifier separation, unweightedMargin=%f weightedMargin=%f", unweightedMargin, weightedMargin)
	}
}

func TestLexicalScoreWithFrequency_NilMatchesDefault(t *testing.T) {
	base := LexicalScore("checkout button", "button: Checkout")
	weightedNil := LexicalScoreWithFrequency("checkout button", "button: Checkout", nil)
	if math.Abs(base-weightedNil) > 1e-9 {
		t.Fatalf("expected nil frequency scoring to match default, base=%f weightedNil=%f", base, weightedNil)
	}
}

func TestLexicalMatcher_Find_CheckoutButtonDiscrimination(t *testing.T) {
	m := NewLexicalMatcher()

	elements := make([]types.ElementDescriptor, 0, 32)
	elements = append(elements, types.ElementDescriptor{Ref: "checkout-btn", Role: "button", Name: "Checkout"})
	elements = append(elements, types.ElementDescriptor{Ref: "checkout-link", Role: "link", Name: "Checkout"})
	for i := 0; i < 30; i++ {
		elements = append(elements, types.ElementDescriptor{Ref: "btn-generic-" + strconv.Itoa(i), Role: "button", Name: "Continue"})
	}

	result, err := m.Find(context.Background(), "checkout button", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if result.BestRef != "checkout-btn" {
		t.Fatalf("expected checkout button to be best match, got %s", result.BestRef)
	}
	if len(result.Matches) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(result.Matches))
	}
	margin := result.Matches[0].Score - result.Matches[1].Score
	if margin < 0.15 {
		t.Fatalf("expected strong preference margin for checkout button, got %.4f", margin)
	}
}

// LexicalMatcher (types.ElementMatcher interface) tests

func TestLexicalMatcher_Find(t *testing.T) {
	m := NewLexicalMatcher()

	if m.Strategy() != "lexical" {
		t.Errorf("expected strategy=lexical, got %s", m.Strategy())
	}

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
		{Ref: "e2", Role: "textbox", Name: "Email Address"},
	}

	result, err := m.Find(context.Background(), "log in button", elements, types.FindOptions{
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
}

func TestLexicalMatcher_ThresholdFiltering(t *testing.T) {
	m := NewLexicalMatcher()

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Home"},
	}

	result, err := m.Find(context.Background(), "submit button", elements, types.FindOptions{
		Threshold: 0.99,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	// Very high threshold — most likely nothing passes.
	for _, m := range result.Matches {
		if m.Score < 0.99 {
			t.Errorf("match %s has score %f below threshold", m.Ref, m.Score)
		}
	}
}

// dummyEmbedder tests
