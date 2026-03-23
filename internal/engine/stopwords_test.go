package engine

import (
	"context"
	"github.com/pinchtab/semantic/internal/types"
	"testing"
)

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

// Prefix Matching Tests

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

// Benchmark: Synonym Expansion Overhead

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
	opts := types.FindOptions{Threshold: 0.15, TopK: 3}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := matcher.Find(ctx, "sign in", elements, opts)
		if err != nil {
			b.Fatalf("Find error: %v", err)
		}
		_ = result
	}
}

// Multi-Site Comprehensive Evaluation (scoring table)

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

// Stopword tests

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

// LexicalScore tests
