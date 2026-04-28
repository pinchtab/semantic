package engine

import (
	"github.com/pinchtab/semantic/internal/types"
	"testing"
)

// types.ElementDescriptor tests

func TestComposite(t *testing.T) {
	tests := []struct {
		name string
		desc types.ElementDescriptor
		want string
	}{
		{
			name: "role and name",
			desc: types.ElementDescriptor{Ref: "e0", Role: "button", Name: "Submit"},
			want: "button: Submit",
		},
		{
			name: "role name and value",
			desc: types.ElementDescriptor{Ref: "e1", Role: "textbox", Name: "Email", Value: "user@pinchtab.com"},
			want: "textbox: Email [user@pinchtab.com]",
		},
		{
			name: "locator identity fields",
			desc: types.ElementDescriptor{
				Ref:         "e4",
				Role:        "textbox",
				Name:        "Email",
				Label:       "Work Email",
				Placeholder: "name@example.com",
				Title:       "Primary email address",
				Text:        "Email",
				TestID:      "email-input",
			},
			want: "textbox: Email label:Work Email placeholder:name@example.com title:Primary email address",
		},
		{
			name: "tag when role missing",
			desc: types.ElementDescriptor{Ref: "e5", Tag: "button", Text: "Save"},
			want: "button: Save",
		},
		{
			name: "name only",
			desc: types.ElementDescriptor{Ref: "e2", Name: "Heading"},
			want: "Heading",
		},
		{
			name: "empty",
			desc: types.ElementDescriptor{Ref: "e3"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.desc.Composite()
			if got != tt.want {
				t.Errorf("Composite() = %q, want %q", got, tt.want)
			}
		})
	}
}

// CalibrateConfidence tests

// CalibrateConfidence tests

func TestCalibrateConfidence(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.0, "high"},
		{0.85, "high"},
		{0.8, "high"},
		{0.79, "medium"},
		{0.6, "medium"},
		{0.59, "low"},
		{0.0, "low"},
	}
	for _, c := range cases {
		got := types.CalibrateConfidence(c.score)
		if got != c.want {
			t.Errorf("types.CalibrateConfidence(%f) = %q, want %q", c.score, got, c.want)
		}
	}
}

// Stopword tests

// FindResult.ConfidenceLabel tests

func TestFindResult_ConfidenceLabel(t *testing.T) {
	r := &types.FindResult{BestScore: 0.9}
	if r.ConfidenceLabel() != "high" {
		t.Errorf("expected high, got %s", r.ConfidenceLabel())
	}

	r.BestScore = 0.65
	if r.ConfidenceLabel() != "medium" {
		t.Errorf("expected medium, got %s", r.ConfidenceLabel())
	}

	r.BestScore = 0.1
	if r.ConfidenceLabel() != "low" {
		t.Errorf("expected low, got %s", r.ConfidenceLabel())
	}
}

// Phase 3: HashingEmbedder tests
