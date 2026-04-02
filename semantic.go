// Package semantic provides zero-dependency semantic matching for
// accessibility tree elements. Match natural language queries like
// "sign in button" against UI element descriptors using lexical
// similarity, synonym expansion, and embedding-based fuzzy matching.
//
// Implementations are internal — consumers use the ElementMatcher
// interface returned by constructors.
package semantic

import (
	"github.com/pinchtab/semantic/internal/engine"
	"github.com/pinchtab/semantic/internal/types"
)

// --- Type aliases (re-exported from internal/types) ---

// ElementMatcher scores accessibility tree elements against a natural language query.
type ElementMatcher = types.ElementMatcher

// Embedder converts text into dense vectors.
type Embedder = types.Embedder

// ElementDescriptor describes a single accessibility tree node.
type ElementDescriptor = types.ElementDescriptor

// PositionalHints captures optional AX-tree relationship metadata.
type PositionalHints = types.PositionalHints

// ElementMatch is a single scored match.
type ElementMatch = types.ElementMatch

// FindOptions controls matching behavior.
type FindOptions = types.FindOptions

// FindResult holds the top matches from a Find call.
type FindResult = types.FindResult

// MatchExplain is the per-strategy score breakdown.
type MatchExplain = types.MatchExplain

// --- Functions ---

// CalibrateConfidence maps a score to "high", "medium", or "low".
func CalibrateConfidence(score float64) string {
	return types.CalibrateConfidence(score)
}

// NewCombinedMatcher creates a matcher that fuses lexical and embedding
// strategies with default weights (0.6 lexical, 0.4 embedding).
func NewCombinedMatcher(embedder Embedder) ElementMatcher {
	return engine.NewCombinedMatcher(embedder)
}

// NewHashingEmbedder creates a zero-dependency hashing-based embedder
// with the given vector dimensionality. Default: 128.
func NewHashingEmbedder(dim int) Embedder {
	return engine.NewHashingEmbedder(dim)
}

// NewLexicalMatcher creates a standalone lexical matcher (Jaccard
// similarity with synonym expansion and role boosting).
func NewLexicalMatcher() ElementMatcher {
	return engine.NewLexicalMatcher()
}

// NewEmbeddingMatcher creates a standalone embedding-based matcher
// (cosine similarity on dense vectors).
func NewEmbeddingMatcher(e Embedder) ElementMatcher {
	return engine.NewEmbeddingMatcher(e)
}

// LexicalScore computes lexical similarity between a query and an
// element description string. Returns [0, 1].
func LexicalScore(query, desc string) float64 {
	return engine.LexicalScore(query, desc)
}

// CosineSimilarity computes cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	return engine.CosineSimilarity(a, b)
}
