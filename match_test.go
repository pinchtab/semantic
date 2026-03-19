package semantic

import (
	"context"
	"fmt"
	"math"
	"testing"
)

// ===========================================================================
// ElementDescriptor tests
// ===========================================================================

func TestComposite(t *testing.T) {
	tests := []struct {
		name string
		desc ElementDescriptor
		want string
	}{
		{
			name: "role and name",
			desc: ElementDescriptor{Ref: "e0", Role: "button", Name: "Submit"},
			want: "button: Submit",
		},
		{
			name: "role name and value",
			desc: ElementDescriptor{Ref: "e1", Role: "textbox", Name: "Email", Value: "user@pinchtab.com"},
			want: "textbox: Email [user@pinchtab.com]",
		},
		{
			name: "name only",
			desc: ElementDescriptor{Ref: "e2", Name: "Heading"},
			want: "Heading",
		},
		{
			name: "empty",
			desc: ElementDescriptor{Ref: "e3"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.desc.Composite()
			if got != tt.want {
				t.Errorf("Composite() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ===========================================================================
// CalibrateConfidence tests
// ===========================================================================

func TestCalibrateConfidence(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.0, "high"},
		{0.85, "high"},
		{0.8, "high"},
		{0.79, "medium"},
		{0.6, "medium"},
		{0.59, "low"},
		{0.0, "low"},
	}
	for _, c := range cases {
		got := CalibrateConfidence(c.score)
		if got != c.want {
			t.Errorf("CalibrateConfidence(%f) = %q, want %q", c.score, got, c.want)
		}
	}
}

// ===========================================================================
// Stopword tests
// ===========================================================================

func TestIsStopword(t *testing.T) {
	if !isStopword("the") {
		t.Error("expected 'the' to be a stopword")
	}
	if isStopword("button") {
		t.Error("expected 'button' not to be a stopword")
	}
}

func TestRemoveStopwords(t *testing.T) {
	tokens := []string{"click", "the", "submit", "button"}
	filtered := removeStopwords(tokens)
	if len(filtered) != 3 {
		t.Errorf("expected 3 tokens after stopword removal, got %d: %v", len(filtered), filtered)
	}

	// When ALL tokens are stopwords, the original should be preserved.
	allStop := []string{"the", "a", "is", "was"}
	kept := removeStopwords(allStop)
	if len(kept) != len(allStop) {
		t.Errorf("expected original tokens when all are stopwords, got %d", len(kept))
	}
}

// ===========================================================================
// LexicalScore tests
// ===========================================================================

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

// ===========================================================================
// LexicalMatcher (ElementMatcher interface) tests
// ===========================================================================

func TestLexicalMatcher_Find(t *testing.T) {
	m := NewLexicalMatcher()

	if m.Strategy() != "lexical" {
		t.Errorf("expected strategy=lexical, got %s", m.Strategy())
	}

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
}

func TestLexicalMatcher_ThresholdFiltering(t *testing.T) {
	m := NewLexicalMatcher()

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

	// Very high threshold — most likely nothing passes.
	for _, m := range result.Matches {
		if m.Score < 0.99 {
			t.Errorf("match %s has score %f below threshold", m.Ref, m.Score)
		}
	}
}

// ===========================================================================
// DummyEmbedder tests
// ===========================================================================

func TestDummyEmbedder_Deterministic(t *testing.T) {
	e := NewDummyEmbedder(64)

	v1, err := e.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	v2, err := e.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	if len(v1[0]) != 64 {
		t.Errorf("expected dim=64, got %d", len(v1[0]))
	}

	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("DummyEmbedder is not deterministic at dim %d", i)
		}
	}
}

func TestDummyEmbedder_Strategy(t *testing.T) {
	e := NewDummyEmbedder(32)
	if e.Strategy() != "dummy" {
		t.Errorf("expected strategy=dummy, got %s", e.Strategy())
	}
}

func TestDummyEmbedder_DefaultDim(t *testing.T) {
	e := NewDummyEmbedder(0)
	if e.Dim != 64 {
		t.Errorf("expected default dim=64, got %d", e.Dim)
	}
}

func TestDummyEmbedder_NormalizedOutput(t *testing.T) {
	e := NewDummyEmbedder(64)
	vecs, err := e.Embed([]string{"test string"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	var norm float64
	for _, v := range vecs[0] {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected unit-norm vector, got norm=%f", norm)
	}
}

// ===========================================================================
// CosineSimilarity tests
// ===========================================================================

func TestCosineSimilarity_Identical(t *testing.T) {
	v := []float32{1, 0, 0, 0}
	sim := CosineSimilarity(v, v)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("identical vectors should have similarity 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0, 0}
	b := []float32{0, 1, 0, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("empty vectors should have similarity 0, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("different-length vectors should return 0, got %f", sim)
	}
}

// ===========================================================================
// EmbeddingMatcher tests
// ===========================================================================

func TestEmbeddingMatcher_Strategy(t *testing.T) {
	m := NewEmbeddingMatcher(NewDummyEmbedder(64))
	want := "embedding:dummy"
	if m.Strategy() != want {
		t.Errorf("expected strategy=%s, got %s", want, m.Strategy())
	}
}

func TestEmbeddingMatcher_Find(t *testing.T) {
	m := NewEmbeddingMatcher(NewDummyEmbedder(64))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Login"},
		{Ref: "e1", Role: "textbox", Name: "Username"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
	}

	result, err := m.Find(context.Background(), "login button", elements, FindOptions{
		Threshold: 0.0,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.ElementCount != 3 {
		t.Errorf("expected ElementCount=3, got %d", result.ElementCount)
	}
	if result.Strategy != "embedding:dummy" {
		t.Errorf("expected strategy=embedding:dummy, got %s", result.Strategy)
	}
	if len(result.Matches) == 0 {
		t.Error("expected at least one match")
	}
	// BestScore should be in valid range
	if result.BestScore < 0 || result.BestScore > 1 {
		t.Errorf("BestScore out of [0,1] range: %f", result.BestScore)
	}
}

func TestEmbeddingMatcher_ThresholdFiltering(t *testing.T) {
	m := NewEmbeddingMatcher(NewDummyEmbedder(64))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Cancel"},
	}

	result, err := m.Find(context.Background(), "xyz completely unrelated", elements, FindOptions{
		Threshold: 0.99,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for _, m := range result.Matches {
		if m.Score < 0.99 {
			t.Errorf("match %s score %f below threshold 0.99", m.Ref, m.Score)
		}
	}
}

// ===========================================================================
// FindResult.ConfidenceLabel tests
// ===========================================================================

func TestFindResult_ConfidenceLabel(t *testing.T) {
	r := &FindResult{BestScore: 0.9}
	if r.ConfidenceLabel() != "high" {
		t.Errorf("expected high, got %s", r.ConfidenceLabel())
	}

	r.BestScore = 0.65
	if r.ConfidenceLabel() != "medium" {
		t.Errorf("expected medium, got %s", r.ConfidenceLabel())
	}

	r.BestScore = 0.1
	if r.ConfidenceLabel() != "low" {
		t.Errorf("expected low, got %s", r.ConfidenceLabel())
	}
}

// ===========================================================================
// Phase 3: HashingEmbedder tests
// ===========================================================================

func TestHashingEmbedder_Strategy(t *testing.T) {
	e := NewHashingEmbedder(128)
	if e.Strategy() != "hashing" {
		t.Errorf("expected strategy=hashing, got %s", e.Strategy())
	}
}

func TestHashingEmbedder_DefaultDim(t *testing.T) {
	e := NewHashingEmbedder(0)
	if e.dim != 128 {
		t.Errorf("expected default dim=128, got %d", e.dim)
	}
}

func TestHashingEmbedder_Deterministic(t *testing.T) {
	e := NewHashingEmbedder(128)
	v1, err := e.Embed([]string{"click the submit button"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	v2, err := e.Embed([]string{"click the submit button"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	if len(v1[0]) != 128 {
		t.Errorf("expected dim=128, got %d", len(v1[0]))
	}
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("HashingEmbedder not deterministic at dim %d: %f != %f", i, v1[0][i], v2[0][i])
		}
	}
}

func TestHashingEmbedder_Normalized(t *testing.T) {
	e := NewHashingEmbedder(128)
	vecs, err := e.Embed([]string{"button submit", "textbox username"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	for i, vec := range vecs {
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1.0) > 0.01 {
			t.Errorf("vector %d not unit-norm: norm=%f", i, norm)
		}
	}
}

func TestHashingEmbedder_EmptyInput(t *testing.T) {
	e := NewHashingEmbedder(64)
	vecs, err := e.Embed([]string{""})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vecs[0]) != 64 {
		t.Errorf("expected dim=64, got %d", len(vecs[0]))
	}
	// Empty string should produce a zero vector (no features to hash).
	var sum float64
	for _, v := range vecs[0] {
		sum += float64(v) * float64(v)
	}
	if sum > 0 {
		t.Error("empty input should produce zero vector")
	}
}

func TestHashingEmbedder_SimilarTexts(t *testing.T) {
	e := NewHashingEmbedder(256) // higher dim for less collision

	vecs, err := e.Embed([]string{
		"submit button",   // 0
		"submit form",     // 1 – shares "submit"
		"download report", // 2 – unrelated
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	simSameWord := CosineSimilarity(vecs[0], vecs[1])  // share "submit"
	simUnrelated := CosineSimilarity(vecs[0], vecs[2]) // no shared words

	if simSameWord <= simUnrelated {
		t.Errorf("texts sharing 'submit' should be more similar: same=%f, unrelated=%f",
			simSameWord, simUnrelated)
	}
}

func TestHashingEmbedder_SubwordSimilarity(t *testing.T) {
	e := NewHashingEmbedder(256)

	// Character n-grams should give nonzero similarity between "button" and "btn".
	vecs, err := e.Embed([]string{
		"button", // full word
		"btn",    // abbreviation – shares "bt" bigram via n-grams
		"search", // unrelated
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	simAbbrev := CosineSimilarity(vecs[0], vecs[1])
	simUnrelated := CosineSimilarity(vecs[0], vecs[2])

	// The abbreviation similarity might be small, but should be greater
	// than an unrelated word due to shared character n-grams.
	if simAbbrev <= simUnrelated {
		t.Errorf("abbreviation should be more similar: abbrev=%f, unrelated=%f",
			simAbbrev, simUnrelated)
	}
}

func TestHashingEmbedder_RoleFeatures(t *testing.T) {
	e := NewHashingEmbedder(128)

	// "button" is a role keyword; it should get an extra role feature.
	vecs, err := e.Embed([]string{
		"button submit",
		"button cancel",
		"textbox email",
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	// Both have "button" role features — should be more similar to each other
	// than to "textbox email" which has a different role keyword.
	simSameRole := CosineSimilarity(vecs[0], vecs[1])
	simDiffRole := CosineSimilarity(vecs[0], vecs[2])

	if simSameRole <= simDiffRole {
		t.Errorf("same-role elements should be more similar: same=%f, diff=%f",
			simSameRole, simDiffRole)
	}
}

func TestHashingEmbedder_BatchConsistency(t *testing.T) {
	e := NewHashingEmbedder(128)

	texts := []string{"login button", "search box", "navigation menu"}
	batchVecs, err := e.Embed(texts)
	if err != nil {
		t.Fatalf("batch embed error: %v", err)
	}

	// Each text embedded individually should match the batch result.
	for i, text := range texts {
		singleVecs, err := e.Embed([]string{text})
		if err != nil {
			t.Fatalf("single embed error: %v", err)
		}
		for j := range singleVecs[0] {
			if singleVecs[0][j] != batchVecs[i][j] {
				t.Errorf("batch[%d] != single at dim %d: %f != %f", i, j, batchVecs[i][j], singleVecs[0][j])
				break
			}
		}
	}
}

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

func TestCombinedMatcher_WeightsApplied(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	// Override weights to emphasize embedding.
	m.LexicalWeight = 0.2
	m.EmbeddingWeight = 0.8

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
	}

	result, err := m.Find(context.Background(), "log in", elements, FindOptions{
		Threshold: 0.01,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	// With embedding-heavy weights the score should differ from default.
	// Just verify the matcher runs and returns valid results.
	if result.ElementCount != 2 {
		t.Errorf("expected ElementCount=2, got %d", result.ElementCount)
	}
	if result.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", result.BestRef)
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
