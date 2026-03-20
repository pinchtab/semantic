// Package semantic provides natural language matching for accessibility tree elements.
package semantic

import "strings"

// ElementDescriptor represents an accessibility tree node.
type ElementDescriptor struct {
	Ref   string
	Role  string
	Name  string
	Value string
}

// Composite returns "role: name [value]" for similarity comparison.
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
