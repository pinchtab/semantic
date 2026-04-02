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
		{"ref":"e1","role":"button","name":"Submit","interactive":true,"parent":"Login form","section":"Authentication","depth":3,"sibling_index":1,"sibling_count":2,"labelled_by":"Primary Action"},
		{"ref":"e2","role":"text","name":"Submit","interactive":false,"parent":"Payment form","section":"Checkout","positional":{"depth":2,"sibling_index":0,"sibling_count":1,"labeled_by":"Secondary Action"}}
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
	if descs[0].Positional.Depth != 3 {
		t.Fatalf("expected first descriptor depth=3, got %d", descs[0].Positional.Depth)
	}
	if descs[0].Positional.SiblingIndex != 1 {
		t.Fatalf("expected first descriptor sibling index=1, got %d", descs[0].Positional.SiblingIndex)
	}
	if descs[0].Positional.SiblingCount != 2 {
		t.Fatalf("expected first descriptor sibling count=2, got %d", descs[0].Positional.SiblingCount)
	}
	if descs[0].Positional.LabelledBy != "Primary Action" {
		t.Fatalf("expected first descriptor labelled_by=Primary Action, got %q", descs[0].Positional.LabelledBy)
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
	if descs[1].Positional.Depth != 2 {
		t.Fatalf("expected second descriptor depth=2, got %d", descs[1].Positional.Depth)
	}
	if descs[1].Positional.SiblingIndex != 0 {
		t.Fatalf("expected second descriptor sibling index=0, got %d", descs[1].Positional.SiblingIndex)
	}
	if descs[1].Positional.SiblingCount != 1 {
		t.Fatalf("expected second descriptor sibling count=1, got %d", descs[1].Positional.SiblingCount)
	}
	if descs[1].Positional.LabelledBy != "Secondary Action" {
		t.Fatalf("expected second descriptor labelled_by=Secondary Action, got %q", descs[1].Positional.LabelledBy)
	}
}
