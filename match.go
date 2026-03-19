package semantic

import "context"

// ElementMatcher is the interface for all element matching strategies.
// Implementations include LexicalMatcher (Jaccard-based, zero deps) and
// EmbeddingMatcher (cosine similarity on vector embeddings).
type ElementMatcher interface {
	// Find scores elements against a natural language query and returns
	// the top-K matches above the threshold.
	Find(ctx context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error)

	// Strategy returns the name of the matching strategy (e.g. "lexical", "embedding").
	Strategy() string
}

// FindOptions controls matching behaviour.
type FindOptions struct {
	Threshold float64
	TopK      int

	// Per-request weight overrides (optional). If both are zero the
	// matcher's default weights are used.
	LexicalWeight   float64
	EmbeddingWeight float64

	// Explain enables verbose per-match scoring breakdown.
	Explain bool
}

// FindResult holds the output of a Find operation.
type FindResult struct {
	Matches      []ElementMatch
	BestRef      string
	BestScore    float64
	Strategy     string
	ElementCount int // total elements evaluated
}

// ConfidenceLabel returns a human-readable confidence level for the best
// match score. Delegates to CalibrateConfidence for consistent labelling.
func (r *FindResult) ConfidenceLabel() string {
	return CalibrateConfidence(r.BestScore)
}

// ElementMatch holds a scored match result.
type ElementMatch struct {
	Ref   string  `json:"ref"`
	Score float64 `json:"score"`
	Role  string  `json:"role,omitempty"`
	Name  string  `json:"name,omitempty"`

	// Explain is populated when FindOptions.Explain is true.
	Explain *MatchExplain `json:"explain,omitempty"`
}

// MatchExplain exposes the per-strategy score breakdown for debugging.
type MatchExplain struct {
	LexicalScore   float64 `json:"lexical_score"`
	EmbeddingScore float64 `json:"embedding_score"`
	Composite      string  `json:"composite"`
}
