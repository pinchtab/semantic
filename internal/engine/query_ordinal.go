package engine

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pinchtab/semantic/internal/types"
)

type ordinalConstraint struct {
	hasOrdinal bool
	last       bool
	position   int
	baseQuery  string
}

var numericOrdinalPattern = regexp.MustCompile(`^(\d+)(st|nd|rd|th)$`)

var ordinalWords = map[string]int{
	"first":   1,
	"second":  2,
	"third":   3,
	"fourth":  4,
	"fifth":   5,
	"sixth":   6,
	"seventh": 7,
	"eighth":  8,
	"ninth":   9,
	"tenth":   10,
}

var ordinalTargetWords = map[string]bool{
	"button":    true,
	"link":      true,
	"input":     true,
	"field":     true,
	"textbox":   true,
	"searchbox": true,
	"item":      true,
	"menu":      true,
	"option":    true,
	"tab":       true,
	"result":    true,
	"row":       true,
	"column":    true,
	"card":      true,
	"entry":     true,
	"element":   true,
}

func parseOrdinalConstraint(query string) ordinalConstraint {
	cleaned := strings.TrimSpace(query)
	if cleaned == "" {
		return ordinalConstraint{}
	}

	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return ordinalConstraint{}
	}

	ordIdx := -1
	ordPos := 0
	ordLast := false

	for i, w := range words {
		norm := normalizeQueryToken(w)
		if norm == "" {
			continue
		}

		if norm == "last" || norm == "final" {
			ordIdx = i
			ordLast = true
			break
		}

		if pos, ok := ordinalWords[norm]; ok {
			ordIdx = i
			ordPos = pos
			break
		}

		if pos, ok := parseNumericOrdinal(norm); ok {
			ordIdx = i
			ordPos = pos
			break
		}
	}

	if ordIdx == -1 {
		return ordinalConstraint{baseQuery: cleaned}
	}

	if !containsOrdinalTarget(words, ordIdx) {
		return ordinalConstraint{baseQuery: cleaned}
	}

	filtered := make([]string, 0, len(words)-1)
	for i, w := range words {
		if i == ordIdx {
			continue
		}
		filtered = append(filtered, w)
	}

	base := strings.Trim(strings.TrimSpace(strings.Join(filtered, " ")), ",.;:-")
	if base == "" {
		base = cleaned
	}

	return ordinalConstraint{
		hasOrdinal: true,
		last:       ordLast,
		position:   ordPos,
		baseQuery:  base,
	}
}

func parseNumericOrdinal(token string) (int, bool) {
	m := numericOrdinalPattern.FindStringSubmatch(token)
	if len(m) != 3 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func normalizeQueryToken(token string) string {
	return strings.Trim(strings.ToLower(token), ",.;:-")
}

func containsOrdinalTarget(words []string, ordIdx int) bool {
	for i, w := range words {
		if i == ordIdx {
			continue
		}
		if ordinalTargetWords[normalizeQueryToken(w)] {
			return true
		}
	}
	return false
}

func selectOrdinalMatchInOrder(result types.FindResult, constraint ordinalConstraint, elements []types.ElementDescriptor) types.FindResult {
	if !constraint.hasOrdinal || len(result.Matches) == 0 {
		return result
	}

	refOrder := make(map[string]int, len(elements))
	for idx, el := range elements {
		refOrder[el.Ref] = idx
	}

	ordered := make([]types.ElementMatch, len(result.Matches))
	copy(ordered, result.Matches)
	sort.SliceStable(ordered, func(i, j int) bool {
		idxI, okI := refOrder[ordered[i].Ref]
		idxJ, okJ := refOrder[ordered[j].Ref]
		if okI && okJ {
			return idxI < idxJ
		}
		if okI != okJ {
			return okI
		}
		return ordered[i].Ref < ordered[j].Ref
	})

	idx := -1
	if constraint.last {
		idx = len(ordered) - 1
	} else if constraint.position > 0 {
		idx = constraint.position - 1
	}

	if idx < 0 || idx >= len(ordered) {
		result.Matches = nil
		result.BestRef = ""
		result.BestScore = 0
		return result
	}

	chosen := ordered[idx]
	result.Matches = []types.ElementMatch{chosen}
	result.BestRef = chosen.Ref
	result.BestScore = chosen.Score
	return result
}
