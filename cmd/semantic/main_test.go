package main

import (
	"os"
	"testing"
)

func TestLoadSnapshot_PropagatesInteractiveFlag(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "snapshot-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}

	json := `[
		{"ref":"e1","role":"button","name":"Submit","interactive":true,"parent":"Login form","section":"Authentication"},
		{"ref":"e2","role":"text","name":"Submit","interactive":false,"parent":"Payment form","section":"Checkout"}
	]`
	if _, err := f.WriteString(json); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	descs, err := loadSnapshot(f.Name())
	if err != nil {
		t.Fatalf("loadSnapshot failed: %v", err)
	}
	if len(descs) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descs))
	}
	if !descs[0].Interactive {
		t.Fatalf("expected first descriptor interactive=true")
	}
	if descs[0].Parent != "Login form" {
		t.Fatalf("expected first descriptor parent=Login form, got %q", descs[0].Parent)
	}
	if descs[0].Section != "Authentication" {
		t.Fatalf("expected first descriptor section=Authentication, got %q", descs[0].Section)
	}
	if descs[1].Interactive {
		t.Fatalf("expected second descriptor interactive=false")
	}
	if descs[1].Parent != "Payment form" {
		t.Fatalf("expected second descriptor parent=Payment form, got %q", descs[1].Parent)
	}
	if descs[1].Section != "Checkout" {
		t.Fatalf("expected second descriptor section=Checkout, got %q", descs[1].Section)
	}
}
