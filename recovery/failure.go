package recovery

import "strings"

type FailureType int

const (
	// FailureUnknown — the error could not be classified; not recoverable.
	FailureUnknown FailureType = iota

	// FailureElementNotFound — the ref no longer maps to a DOM node.
	// Recoverable via semantic re-match on a fresh snapshot.
	FailureElementNotFound

	// FailureElementStale — the node exists but has been detached from
	// the live DOM (SPA re-render, modal overlay, etc.).
	FailureElementStale

	// FailureElementNotInteractable — the element is hidden, disabled,
	// or covered by an overlay and cannot receive input.
	FailureElementNotInteractable

	// FailureNavigation — the action triggered a page navigation; the
	// entire snapshot is invalid. Recoverable via fresh snapshot.
	FailureNavigation

	// FailureNetwork — a transport or Chrome-level error; not recoverable
	// by the recovery engine.
	FailureNetwork
)

func (f FailureType) String() string {
	switch f {
	case FailureElementNotFound:
		return "element_not_found"
	case FailureElementStale:
		return "element_stale"
	case FailureElementNotInteractable:
		return "element_not_interactable"
	case FailureNavigation:
		return "navigation"
	case FailureNetwork:
		return "network"
	default:
		return "unknown"
	}
}

func (f FailureType) Recoverable() bool {
	switch f {
	case FailureElementNotFound,
		FailureElementStale,
		FailureElementNotInteractable,
		FailureNavigation:
		return true
	default:
		return false
	}
}

type classificationRule struct {
	failureType FailureType
	patterns    []string
}

var classificationRules = []classificationRule{
	{FailureElementNotFound, []string{
		"could not find node", "node with given id", "no node",
		"ref not found", "node not found", "backend node id", "no node with given",
	}},
	{FailureElementStale, []string{
		"stale", "orphan", "object reference", "node is detached",
		"execution context was destroyed", "context was destroyed", "target closed",
	}},
	{FailureElementNotInteractable, []string{
		"not interactable", "not clickable", "element is not visible",
		"not visible", "element is disabled", "overlapped", "overlapping",
		"obscured", "pointer-events: none", "cannot focus",
		"outside of the viewport", "outside the viewport",
	}},
	{FailureNavigation, []string{
		"navigat", "page crashed", "page was destroyed",
		"inspected target", "frame was detached", "frame detached",
	}},
	{FailureNetwork, []string{
		"net::", "connection refused", "could not connect",
		"timeout", "deadline exceeded", "websocket", "eof", "broken pipe",
	}},
}

// ClassifyFailure maps an error to a FailureType by pattern-matching
// against known CDP, chromedp, and PinchTab error messages.
func ClassifyFailure(err error) FailureType {
	if err == nil {
		return FailureUnknown
	}
	e := strings.ToLower(err.Error())

	for _, rule := range classificationRules {
		for _, p := range rule.patterns {
			if strings.Contains(e, p) {
				return rule.failureType
			}
		}
	}

	// Special case: "detached" alone is stale, but "frame" + "detached"
	// is navigation (already caught by the navigation rule above).
	if strings.Contains(e, "detached") && !strings.Contains(e, "frame") {
		return FailureElementStale
	}

	return FailureUnknown
}
