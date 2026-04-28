package engine

import (
	"context"
	"strconv"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

// benchElements returns a realistic set of elements for benchmarking.
func benchElements() []types.ElementDescriptor {
	return []types.ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Welcome Back"},
		{Ref: "e1", Role: "textbox", Name: "Email address", Value: ""},
		{Ref: "e2", Role: "textbox", Name: "Password", Value: ""},
		{Ref: "e3", Role: "checkbox", Name: "Remember me"},
		{Ref: "e4", Role: "button", Name: "Sign In"},
		{Ref: "e5", Role: "link", Name: "Forgot password?"},
		{Ref: "e6", Role: "link", Name: "Create an account"},
		{Ref: "e7", Role: "link", Name: "Privacy Policy"},
		{Ref: "e8", Role: "link", Name: "Terms of Service"},
		{Ref: "e9", Role: "button", Name: "Sign in with Google"},
		{Ref: "e10", Role: "button", Name: "Sign in with GitHub"},
		{Ref: "e11", Role: "heading", Name: "Or continue with"},
		{Ref: "e12", Role: "link", Name: "Help Center"},
		{Ref: "e13", Role: "link", Name: "Contact Support"},
		{Ref: "e14", Role: "button", Name: "Close"},
		{Ref: "e15", Role: "navigation", Name: "Main navigation"},
		{Ref: "e16", Role: "link", Name: "Home"},
		{Ref: "e17", Role: "link", Name: "Products"},
		{Ref: "e18", Role: "link", Name: "Pricing"},
		{Ref: "e19", Role: "link", Name: "About"},
	}
}

func BenchmarkLexicalScoreFn(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		LexicalScore("sign in button", "button: Sign In")
	}
}

func BenchmarkLexicalScoreFn_Synonym(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		LexicalScore("log in", "button: Sign In")
	}
}

func BenchmarkLexicalFind(b *testing.B) {
	m := NewLexicalMatcher()
	elements := benchElements()
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "sign in button", elements, opts)
	}
}

func BenchmarkLexicalFind_200Elements(b *testing.B) {
	m := NewLexicalMatcher()
	base := benchElements()
	elements := make([]types.ElementDescriptor, 0, 200)
	for len(elements) < 200 {
		elements = append(elements, base...)
	}
	elements = elements[:200]

	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.0, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "checkout button", elements, opts)
	}
}

func BenchmarkLexicalFind_TypoQuery_200Elements(b *testing.B) {
	m := NewLexicalMatcher()
	base := benchElements()
	elements := make([]types.ElementDescriptor, 0, 200)
	for len(elements) < 200 {
		elements = append(elements, base...)
	}
	elements = elements[:200]

	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.0, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "setttings", elements, opts)
	}
}

func BenchmarkHashingEmbed(b *testing.B) {
	h := NewHashingEmbedder(128)
	texts := []string{"sign in button"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Embed(texts)
	}
}

func BenchmarkHashingEmbed_Long(b *testing.B) {
	h := NewHashingEmbedder(128)
	texts := []string{"click the big blue sign in button on the top right of the login page"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h.Embed(texts)
	}
}

func BenchmarkEmbeddingFind(b *testing.B) {
	m := NewEmbeddingMatcher(NewHashingEmbedder(128))
	elements := benchElements()
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "sign in button", elements, opts)
	}
}

func BenchmarkEmbeddingFind_NoNeighborContext(b *testing.B) {
	m := NewEmbeddingMatcherWithNeighborWeight(NewHashingEmbedder(128), 0)
	elements := benchElements()
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "sign in button", elements, opts)
	}
}

func BenchmarkCombinedFind(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := benchElements()
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "sign in button", elements, opts)
	}
}

func BenchmarkCombinedFind_Synonym(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := benchElements()
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "log in", elements, opts)
	}
}

func BenchmarkCombinedFind_100Elements(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	base := benchElements()
	// Repeat to get ~100 elements
	elements := make([]types.ElementDescriptor, 0, 100)
	for len(elements) < 100 {
		for _, e := range base {
			clone := e
			clone.Ref = "e" + strconv.Itoa(len(elements))
			elements = append(elements, clone)
			if len(elements) >= 100 {
				break
			}
		}
	}
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "sign in button", elements, opts)
	}
}

func BenchmarkCosineSim(b *testing.B) {
	h := NewHashingEmbedder(128)
	vecs1, _ := h.Embed([]string{"sign in button"})
	vecs2, _ := h.Embed([]string{"button: Sign In"})
	a, v := vecs1[0], vecs2[0]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineSimilarity(a, v)
	}
}

func BenchmarkCalibrateConfidence(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		types.CalibrateConfidence(0.75)
	}
}

func benchElementsSized(n int) []types.ElementDescriptor {
	base := benchElements()
	out := make([]types.ElementDescriptor, 0, n)
	for len(out) < n {
		for _, e := range base {
			clone := e
			clone.Ref = "e" + strconv.Itoa(len(out))
			out = append(out, clone)
			if len(out) >= n {
				break
			}
		}
	}
	return out
}

func BenchmarkCombinedFind_Issue24_100Elements(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := benchElementsSized(100)
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.3, TopK: 3}

	queries := []string{
		"sign in button",
		"button not submit",
		"textbox not email",
	}

	for _, q := range queries {
		b.Run(q, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = m.Find(ctx, q, elements, opts)
			}
		})
	}
}

// Focused microbenchmarks for individual components

func BenchmarkParseQueryContext(b *testing.B) {
	queries := []string{
		"sign in button",
		"the first email textbox in the login form",
		"button not submit near the checkout section",
		"second item in the dropdown menu",
	}
	b.ReportAllocs()

	for b.Loop() {
		for _, q := range queries {
			ParseQueryContext(q)
		}
	}
}

func BenchmarkParseQueryContext_Complex(b *testing.B) {
	q := "the third blue submit button in the checkout form not disabled"
	b.ReportAllocs()

	for b.Loop() {
		ParseQueryContext(q)
	}
}

func BenchmarkRemoveStopwords(b *testing.B) {
	tokenSets := [][]string{
		{"click", "the", "sign", "in", "button"},
		{"find", "the", "email", "address", "textbox"},
		{"the", "first", "item", "in", "a", "dropdown", "menu"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tokens := range tokenSets {
			removeStopwords(tokens)
		}
	}
}

func BenchmarkScoreFusion(b *testing.B) {
	// Test the score fusion calculation
	lexScores := make([]float64, 100)
	embScores := make([]float64, 100)
	for i := range lexScores {
		lexScores[i] = float64(i) / 100.0
		embScores[i] = float64(100-i) / 100.0
	}
	lexWeight, embWeight := 0.6, 0.4
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range lexScores {
			_ = lexWeight*lexScores[j] + embWeight*embScores[j]
		}
	}
}

func BenchmarkLexicalScore_Variants(b *testing.B) {
	cases := []struct {
		name  string
		query string
		desc  string
	}{
		{"exact", "Sign In", "button: Sign In"},
		{"partial", "sign", "button: Sign In"},
		{"synonym", "login", "button: Sign In"},
		{"mismatch", "checkout", "button: Sign In"},
		{"long_query", "click the sign in button on the login page", "button: Sign In"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				LexicalScore(tc.query, tc.desc)
			}
		})
	}
}

func BenchmarkCombinedFind_WeightVariants(b *testing.B) {
	elements := benchElements()
	ctx := context.Background()

	weights := []struct {
		name string
		lex  float64
		emb  float64
	}{
		{"lex_only", 1.0, 0.0},
		{"emb_only", 0.0, 1.0},
		{"balanced", 0.5, 0.5},
		{"lex_heavy", 0.8, 0.2},
		{"emb_heavy", 0.2, 0.8},
	}

	for _, w := range weights {
		b.Run(w.name, func(b *testing.B) {
			m := NewCombinedMatcher(NewHashingEmbedder(128))
			opts := types.FindOptions{
				Threshold:       0.3,
				TopK:            3,
				LexicalWeight:   w.lex,
				EmbeddingWeight: w.emb,
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = m.Find(ctx, "sign in button", elements, opts)
			}
		})
	}
}

func BenchmarkStructuredLocatorFind(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Welcome Back", Text: "Welcome Back", DocumentIdx: 0},
		{Ref: "e1", Role: "textbox", Name: "Email address", Label: "Email address", Placeholder: "name@example.com", Tag: "input", Interactive: true, DocumentIdx: 1},
		{Ref: "e2", Role: "textbox", Name: "Password", Label: "Password", Placeholder: "Password", Tag: "input", Interactive: true, DocumentIdx: 2},
		{Ref: "e3", Role: "checkbox", Name: "Remember me", Label: "Remember me", Tag: "input", Interactive: true, DocumentIdx: 3},
		{Ref: "e4", Role: "button", Name: "Sign In", Text: "Sign In", TestID: "submit-login", Tag: "button", Interactive: true, DocumentIdx: 4},
		{Ref: "e5", Role: "link", Name: "Forgot password?", Text: "Forgot password?", Tag: "a", Interactive: true, DocumentIdx: 5},
		{Ref: "e6", Role: "img", Name: "Company Logo", Alt: "Company Logo", Tag: "img", DocumentIdx: 6},
	}
	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0, TopK: 3}
	queries := []string{
		"role:button Sign In",
		"text:Forgot password?",
		"label:Email address",
		"placeholder:Password",
		"alt:Company Logo",
		"testid:submit-login",
		"nth:0:role:textbox",
	}

	for _, query := range queries {
		b.Run(query, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = m.Find(ctx, query, elements, opts)
			}
		})
	}
}

func BenchmarkParseStructuredLocator(b *testing.B) {
	queries := []string{
		"role:button Sign In",
		"text:Forgot password?",
		"label:Email address",
		"placeholder:Password",
		"alt:Company Logo",
		"testid:submit-login",
		"nth:2:role:button Save",
	}
	b.ReportAllocs()

	for b.Loop() {
		for _, query := range queries {
			parseStructuredLocator(query)
		}
	}
}
