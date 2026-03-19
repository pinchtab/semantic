// Package semantic provides lightweight lexical similarity matching
// for accessibility tree elements. Zero external dependencies.
package semantic

import "strings"

// ElementDescriptor builds a composite text description from an
// accessibility tree node's properties for similarity comparison.
type ElementDescriptor struct {
	Ref   string
	Role  string
	Name  string
	Value string
}

// Composite returns a single string that captures the semantic identity
// of an element, suitable for lexical similarity comparison.
func (ed *ElementDescriptor) Composite() string {
	var parts []string

	if ed.Role != "" {
		parts = append(parts, ed.Role+":")
	}
	if ed.Name != "" {
		parts = append(parts, ed.Name)
	}
	if ed.Value != "" && ed.Value != ed.Name {
		parts = append(parts, "["+ed.Value+"]")
	}

	return strings.Join(parts, " ")
}
