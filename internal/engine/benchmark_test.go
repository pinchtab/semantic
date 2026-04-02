package engine

import (
	"context"
	"github.com/pinchtab/semantic/internal/types"
	"testing"
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
			e.Ref = "e" + string(rune('0'+len(elements)))
			elements = append(elements, e)
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
