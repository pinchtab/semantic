// Package types defines the public API types for the semantic library.
// These are re-exported by the root semantic package.
package types

import (
	"context"
	"strings"
)

// ElementMatcher scores accessibility tree elements against a natural language query.
type ElementMatcher interface {
	// Find scores elements against a natural language query and returns
	// the top-K matches above the threshold.
	Find(ctx context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error)

	// Strategy returns the name of the matching strategy (e.g. "lexical", "embedding").
	Strategy() string
}

// Embedder converts text into dense vectors. See NewHashingEmbedder.
type Embedder interface {
	// Embed converts a batch of text strings into float32 vectors.
	// All returned vectors must have the same dimensionality.
	Embed(texts []string) ([][]float32, error)

	// Strategy returns the name of the embedding strategy (e.g. "hashing", "openai").
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

// ParsedQuery splits a raw query into positive and negative token groups.
// Negative tokens are interpreted as terms that should be penalized or excluded.
type ParsedQuery struct {
	Positive []string
	Negative []string
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

// PositionalHints captures optional AX-tree relationship and position metadata.
type PositionalHints struct {
	Depth        int     `json:"depth,omitempty"`
	SiblingIndex int     `json:"sibling_index,omitempty"`
	SiblingCount int     `json:"sibling_count,omitempty"`
	LabelledBy   string  `json:"labelled_by,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	Width        float64 `json:"width,omitempty"`
	Height       float64 `json:"height,omitempty"`
}

// ElementDescriptor describes a single accessibility tree node.
type ElementDescriptor struct {
	Ref         string          `json:"ref,omitempty"`
	Role        string          `json:"role,omitempty"`
	Name        string          `json:"name,omitempty"`
	Value       string          `json:"value,omitempty"`
	Label       string          `json:"label,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Alt         string          `json:"alt,omitempty"`
	Title       string          `json:"title,omitempty"`
	TestID      string          `json:"testid,omitempty"`
	Text        string          `json:"text,omitempty"`
	Tag         string          `json:"tag,omitempty"`
	Interactive bool            `json:"interactive,omitempty"`
	Parent      string          `json:"parent,omitempty"`
	Section     string          `json:"section,omitempty"`
	DocumentIdx int             `json:"document_idx,omitempty"`
	Positional  PositionalHints `json:"positional,omitempty"`
}

// Composite returns a single string combining public element identity fields
// for matching purposes: "role: name [value]".
func (ed *ElementDescriptor) Composite() string {
	var parts []string

	if ed.Role != "" {
		parts = append(parts, ed.Role+":")
	} else if ed.Tag != "" {
		parts = append(parts, ed.Tag+":")
	}
	if ed.Name != "" {
		parts = append(parts, ed.Name)
	}
	if ed.Value != "" && ed.Value != ed.Name {
		parts = append(parts, "["+ed.Value+"]")
	}
	if shouldAddCompositeField(ed.Text, ed.Name, ed.Value) {
		parts = append(parts, ed.Text)
	}
	if shouldAddCompositeField(ed.Label, ed.Name, ed.Text) {
		parts = append(parts, "label:"+ed.Label)
	}
	if shouldAddCompositeField(ed.Placeholder, ed.Name, ed.Text) {
		parts = append(parts, "placeholder:"+ed.Placeholder)
	}
	if shouldAddCompositeField(ed.Alt, ed.Name, ed.Text) {
		parts = append(parts, "alt:"+ed.Alt)
	}
	if shouldAddCompositeField(ed.Title, ed.Name, ed.Text) {
		parts = append(parts, "title:"+ed.Title)
	}
	if ed.Section != "" {
		parts = append(parts, "{"+ed.Section+"}")
	}

	return strings.Join(parts, " ")
}

func shouldAddCompositeField(value string, existing ...string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	normValue := normalizeCompositeValue(value)
	for _, candidate := range existing {
		if normValue == normalizeCompositeValue(candidate) {
			return false
		}
	}
	return true
}

func normalizeCompositeValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

// CalibrateConfidence maps a score to a human-readable confidence level.
func CalibrateConfidence(score float64) string {
	switch {
	case score >= 0.8:
		return "high"
	case score >= 0.6:
		return "medium"
	default:
		return "low"
	}
}
