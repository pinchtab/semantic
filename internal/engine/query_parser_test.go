package engine

import (
	"reflect"
	"testing"
)

func TestParseQuery_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		positive []string
		negative []string
	}{
		{
			name:     "button not submit",
			raw:      "button not submit",
			positive: []string{"button"},
			negative: []string{"submit"},
		},
		{
			name:     "button not sign in",
			raw:      "button not sign in",
			positive: []string{"button"},
			negative: []string{"sign", "in"},
		},
		{
			name:     "link without logout",
			raw:      "link without logout",
			positive: []string{"link"},
			negative: []string{"logout"},
		},
		{
			name:     "input excluding email",
			raw:      "input excluding email",
			positive: []string{"input"},
			negative: []string{"email"},
		},
		{
			name:     "button except close",
			raw:      "button except close",
			positive: []string{"button"},
			negative: []string{"close"},
		},
		{
			name:     "sign in button",
			raw:      "sign in button",
			positive: []string{"sign", "in", "button"},
			negative: nil,
		},
		{
			name:     "not button",
			raw:      "not button",
			positive: nil,
			negative: []string{"button"},
		},
		{
			name:     "input no password no username",
			raw:      "input no password no username",
			positive: []string{"input"},
			negative: []string{"password", "username"},
		},
		{
			name:     "negative segment break by trigger",
			raw:      "button not sign in except submit",
			positive: []string{"button"},
			negative: []string{"sign", "in", "submit"},
		},
		{
			name:     "trailing trigger behaves as positive query",
			raw:      "button not",
			positive: []string{"button"},
			negative: nil,
		},
		{
			name:     "repeated triggers",
			raw:      "button not submit not cancel",
			positive: []string{"button"},
			negative: []string{"submit", "cancel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseQuery(tt.raw)
			if !reflect.DeepEqual(normalizeTokens(got.Positive), normalizeTokens(tt.positive)) {
				t.Fatalf("positive mismatch: got=%v want=%v", got.Positive, tt.positive)
			}
			if !reflect.DeepEqual(normalizeTokens(got.Negative), normalizeTokens(tt.negative)) {
				t.Fatalf("negative mismatch: got=%v want=%v", got.Negative, tt.negative)
			}
		})
	}
}

func normalizeTokens(tokens []string) []string {
	if len(tokens) == 0 {
		return []string{}
	}
	return tokens
}
