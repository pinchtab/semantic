package recovery

import (
	"fmt"
	"testing"
)

func TestClassifyFailure_NilError(t *testing.T) {
	ft := ClassifyFailure(nil)
	if ft != FailureUnknown {
		t.Errorf("ClassifyFailure(nil) = %v, want FailureUnknown", ft)
	}
}

func TestClassifyFailure_ElementNotFound(t *testing.T) {
	patterns := []string{
		"could not find node with id 42",
		"node with given id not found",
		"no node for backendNodeId 123",
		"ref not found: e15",
		"Node not found for the given backend node id",
		"backend node id cannot be resolved",
		"no node with given id",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementNotFound {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementNotFound", p, ft)
		}
	}
}

func TestClassifyFailure_ElementStale(t *testing.T) {
	patterns := []string{
		"stale element reference",
		"node is detached from the document",
		"execution context was destroyed",
		"orphan node detected",
		"object reference not set",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementStale {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementStale", p, ft)
		}
	}
}

func TestClassifyFailure_NotInteractable(t *testing.T) {
	patterns := []string{
		"node is not visible on the page",
		"element is not interactable at point (100, 200)",
		"overlapping element covers the target",
		"element is disabled",
		"cannot focus the target element",
		"Element is outside of the viewport",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureElementNotInteractable {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureElementNotInteractable", p, ft)
		}
	}
}

func TestClassifyFailure_Navigation(t *testing.T) {
	patterns := []string{
		"page navigated while action was pending",
		"Frame was detached during execution",
		"inspected target navigated or closed",
		"Navigated away from page",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureNavigation {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureNavigation", p, ft)
		}
	}
}

func TestClassifyFailure_Network(t *testing.T) {
	patterns := []string{
		"connection refused to remote debugging port",
		"websocket connection closed unexpectedly",
		"could not connect to Chrome",
		"timeout waiting for response from browser",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureNetwork {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureNetwork", p, ft)
		}
	}
}

func TestClassifyFailure_Unknown(t *testing.T) {
	patterns := []string{
		"something completely unexpected happened",
		"random error",
	}
	for _, p := range patterns {
		ft := ClassifyFailure(fmt.Errorf("%s", p))
		if ft != FailureUnknown {
			t.Errorf("ClassifyFailure(%q) = %v, want FailureUnknown", p, ft)
		}
	}
}

func TestFailureType_String(t *testing.T) {
	cases := map[FailureType]string{
		FailureUnknown:                "unknown",
		FailureElementNotFound:        "element_not_found",
		FailureElementStale:           "element_stale",
		FailureElementNotInteractable: "element_not_interactable",
		FailureNavigation:             "navigation",
		FailureNetwork:                "network",
	}
	for ft, want := range cases {
		if ft.String() != want {
			t.Errorf("FailureType(%d).String() = %q, want %q", ft, ft.String(), want)
		}
	}
}

func TestFailureType_Recoverable(t *testing.T) {
	recoverable := []FailureType{
		FailureElementNotFound,
		FailureElementStale,
		FailureElementNotInteractable,
		FailureNavigation,
	}
	for _, ft := range recoverable {
		if !ft.Recoverable() {
			t.Errorf("%v.Recoverable() = false, want true", ft)
		}
	}
	nonRecoverable := []FailureType{
		FailureUnknown,
		FailureNetwork,
	}
	for _, ft := range nonRecoverable {
		if ft.Recoverable() {
			t.Errorf("%v.Recoverable() = true, want false", ft)
		}
	}
}
