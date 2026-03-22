package semantic

import "context"

// ElementMatcher scores accessibility tree elements against a natural language query.
type ElementMatcher interface {
	// Find scores elements against a natural language query and returns
	// the top-K matches above the threshold.
	Find(ctx context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error)

	// Strategy returns the name of the matching strategy (e.g. "lexical", "embedding").
	Strategy() string
}

// FindOptions controls matching behavior.
type FindOptions struct {
	// Threshold is the minimum similarity score for a match to be included.
	Threshold float64

	// TopK is the maximum number of matches to return.
	TopK int

	// Per-request weight overrides (optional). If both are zero the
	// matcher's default weights are used.
	LexicalWeight   float64
	EmbeddingWeight float64

	// Explain enables verbose per-match scoring breakdown.
	Explain bool
}

// FindResult holds the top matches from a Find call.
type FindResult struct {
	Matches      []ElementMatch
	BestRef      string
	BestScore    float64
	Strategy     string
	ElementCount int // total elements evaluated
}

// ConfidenceLabel returns "high", "medium", or "low" for the best match.
func (r *FindResult) ConfidenceLabel() string {
	return CalibrateConfidence(r.BestScore)
}

// ElementMatch is a single scored match.
type ElementMatch struct {
	Ref   string  `json:"ref"`
	Score float64 `json:"score"`
	Role  string  `json:"role,omitempty"`
	Name  string  `json:"name,omitempty"`

	// Explain is populated when FindOptions.Explain is true.
	Explain *MatchExplain `json:"explain,omitempty"`
}

// MatchExplain is the per-strategy score breakdown (when FindOptions.Explain is true).
type MatchExplain struct {
	LexicalScore   float64 `json:"lexical_score"`
	EmbeddingScore float64 `json:"embedding_score"`
	Composite      string  `json:"composite"`
}
