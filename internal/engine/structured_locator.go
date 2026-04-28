package engine

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pinchtab/semantic/internal/types"
)

type structuredLocatorKind int

const (
	locatorRole structuredLocatorKind = iota + 1
	locatorText
	locatorLabel
	locatorPlaceholder
	locatorAlt
	locatorTitle
	locatorTestID
)

type structuredLocatorWrapper int

const (
	locatorNoWrapper structuredLocatorWrapper = iota
	locatorFirst
	locatorLast
	locatorNth
)

type structuredLocator struct {
	kind    structuredLocatorKind
	role    string
	value   string
	wrapper structuredLocatorWrapper
	nth     int
}

type structuredLocatorCandidate struct {
	desc  types.ElementDescriptor
	score float64
	order int
}

func normalizeSemanticQuery(raw string) (query string, forceNatural bool) {
	query = strings.TrimSpace(raw)
	if rest, ok := cutFoldedPrefix(query, "find:"); ok {
		return strings.TrimSpace(rest), true
	}
	if rest, ok := cutFoldedPrefix(query, "semantic:"); ok {
		return strings.TrimSpace(rest), true
	}
	return query, false
}

func findStructuredLocator(raw string, elements []types.ElementDescriptor, opts types.FindOptions, strategy string) (types.FindResult, bool) {
	locator, ok := parseStructuredLocator(raw)
	if !ok {
		return types.FindResult{}, false
	}
	return matchStructuredLocator(locator, elements, opts, strategy), true
}

func parseStructuredLocator(raw string) (structuredLocator, bool) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return structuredLocator{}, false
	}

	if rest, ok := cutFoldedPrefix(query, "first:"); ok {
		locator, ok := parseStructuredLocatorBase(rest)
		if !ok {
			return structuredLocator{}, false
		}
		locator.wrapper = locatorFirst
		return locator, true
	}

	if rest, ok := cutFoldedPrefix(query, "last:"); ok {
		locator, ok := parseStructuredLocatorBase(rest)
		if !ok {
			return structuredLocator{}, false
		}
		locator.wrapper = locatorLast
		return locator, true
	}

	if rest, ok := cutFoldedPrefix(query, "nth:"); ok {
		ordinal, baseRaw, ok := splitNthLocator(rest)
		if !ok {
			return structuredLocator{}, false
		}
		locator, ok := parseStructuredLocatorBase(baseRaw)
		if !ok {
			return structuredLocator{}, false
		}
		locator.wrapper = locatorNth
		locator.nth = ordinal
		return locator, true
	}

	return parseStructuredLocatorBase(query)
}

func parseStructuredLocatorBase(raw string) (structuredLocator, bool) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return structuredLocator{}, false
	}

	if rest, ok := cutFoldedPrefix(query, "role:"); ok {
		role, name, ok := splitRoleLocator(rest)
		if !ok {
			return structuredLocator{}, false
		}
		return structuredLocator{kind: locatorRole, role: role, value: name}, true
	}

	fieldPrefixes := []struct {
		prefix string
		kind   structuredLocatorKind
	}{
		{"text:", locatorText},
		{"label:", locatorLabel},
		{"placeholder:", locatorPlaceholder},
		{"alt:", locatorAlt},
		{"title:", locatorTitle},
		{"testid:", locatorTestID},
	}

	for _, prefix := range fieldPrefixes {
		if rest, ok := cutFoldedPrefix(query, prefix.prefix); ok {
			value := cleanLocatorValue(rest)
			if value == "" {
				return structuredLocator{}, false
			}
			return structuredLocator{kind: prefix.kind, value: value}, true
		}
	}

	return structuredLocator{}, false
}

func splitNthLocator(raw string) (int, string, bool) {
	rest := strings.TrimSpace(raw)
	sep := strings.Index(rest, ":")
	if sep <= 0 {
		return 0, "", false
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:sep]))
	if err != nil || n < 0 {
		return 0, "", false
	}
	base := strings.TrimSpace(rest[sep+1:])
	if base == "" {
		return 0, "", false
	}
	return n, base, true
}

func splitRoleLocator(raw string) (role string, name string, ok bool) {
	rest := strings.TrimSpace(raw)
	if rest == "" {
		return "", "", false
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", "", false
	}

	role = cleanRoleValue(fields[0])
	if role == "" {
		return "", "", false
	}

	nameStart := strings.Index(rest, fields[0]) + len(fields[0])
	if nameStart < len(rest) {
		name = cleanLocatorValue(rest[nameStart:])
	}
	return role, name, true
}

func cutFoldedPrefix(s, prefix string) (string, bool) {
	if len(s) < len(prefix) {
		return "", false
	}
	if !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	return s[len(prefix):], true
}

func cleanRoleValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ",.;")
	return strings.ToLower(value)
}

func cleanLocatorValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, ",.;")
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') || (first == '`' && last == '`') || (first == '[' && last == ']') {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func matchStructuredLocator(locator structuredLocator, elements []types.ElementDescriptor, opts types.FindOptions, strategy string) types.FindResult {
	opts = sanitizeFindOptions(opts, len(elements), 3)

	result := types.FindResult{
		Strategy:     strategy + ":structured",
		ElementCount: len(elements),
	}

	candidates := make([]structuredLocatorCandidate, 0, len(elements))
	for i, el := range elements {
		score, ok := scoreStructuredLocatorCandidate(locator, el)
		if !ok || score < opts.Threshold {
			continue
		}
		candidates = append(candidates, structuredLocatorCandidate{
			desc:  el,
			score: score,
			order: structuredElementOrder(i, el),
		})
	}

	if locator.wrapper != locatorNoWrapper {
		sortStructuredLocatorCandidatesByDocumentOrder(candidates)
		candidates = selectStructuredLocatorWrapper(candidates, locator)
	} else {
		sortStructuredLocatorCandidatesByRank(candidates)
		if len(candidates) > opts.TopK {
			candidates = candidates[:opts.TopK]
		}
	}

	for _, candidate := range candidates {
		result.Matches = append(result.Matches, types.ElementMatch{
			Ref:   candidate.desc.Ref,
			Score: candidate.score,
			Role:  candidate.desc.Role,
			Name:  candidate.desc.Name,
		})
	}
	if len(result.Matches) > 0 {
		result.BestRef = result.Matches[0].Ref
		result.BestScore = result.Matches[0].Score
	}
	return result
}

func sortStructuredLocatorCandidatesByRank(candidates []structuredLocatorCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return rankedMatchLess(
			candidates[i].score, candidates[i].desc, candidates[i].order,
			candidates[j].score, candidates[j].desc, candidates[j].order,
		)
	})
}

func sortStructuredLocatorCandidatesByDocumentOrder(candidates []structuredLocatorCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].order != candidates[j].order {
			return candidates[i].order < candidates[j].order
		}
		return candidates[i].desc.Ref < candidates[j].desc.Ref
	})
}

func selectStructuredLocatorWrapper(candidates []structuredLocatorCandidate, locator structuredLocator) []structuredLocatorCandidate {
	if len(candidates) == 0 {
		return nil
	}

	idx := 0
	switch locator.wrapper {
	case locatorFirst:
		idx = 0
	case locatorLast:
		idx = len(candidates) - 1
	case locatorNth:
		idx = locator.nth - 1
	default:
		return candidates
	}

	if idx < 0 || idx >= len(candidates) {
		return nil
	}
	return []structuredLocatorCandidate{candidates[idx]}
}

func scoreStructuredLocatorCandidate(locator structuredLocator, el types.ElementDescriptor) (float64, bool) {
	switch locator.kind {
	case locatorRole:
		roleScore, ok := scoreLocatorRole(locator.role, el)
		if !ok {
			return 0, false
		}
		if strings.TrimSpace(locator.value) == "" {
			return roleScore, true
		}
		nameScore, ok := scoreLocatorValues(locator.value, roleNameLocatorValues(el))
		if !ok {
			return 0, false
		}
		return roleScore * nameScore, true
	case locatorText, locatorLabel, locatorPlaceholder, locatorAlt, locatorTitle, locatorTestID:
		return scoreLocatorValues(locator.value, locatorFieldValues(locator.kind, el))
	default:
		return 0, false
	}
}

func roleNameLocatorValues(el types.ElementDescriptor) []string {
	return []string{
		el.Name,
		el.Label,
		el.Positional.LabelledBy,
		el.Text,
		el.Alt,
		el.Title,
		el.Placeholder,
		el.Value,
	}
}

func scoreLocatorRole(role string, el types.ElementDescriptor) (float64, bool) {
	want := normalizeLocatorText(role)
	if want == "" {
		return 0, false
	}

	explicit := normalizeLocatorText(el.Role)
	if explicit != "" && explicit == want {
		return 1.0, true
	}
	if explicit != "" && !allowsImplicitRoleFallback(explicit) {
		return 0, false
	}

	if implicitRoleForTag(el.Tag) == want {
		return 0.92, true
	}
	return 0, false
}

func allowsImplicitRoleFallback(role string) bool {
	switch role {
	case "", "generic", "none", "presentation", "unknown":
		return true
	default:
		return false
	}
}

func implicitRoleForTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return ""
	}
	if idx := strings.IndexFunc(tag, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}); idx >= 0 {
		tag = tag[:idx]
	}

	switch tag {
	case "a", "area":
		return "link"
	case "button", "summary":
		return "button"
	case "input", "textarea":
		return "textbox"
	case "select":
		return "combobox"
	case "option":
		return "option"
	case "img":
		return "img"
	case "form":
		return "form"
	case "nav":
		return "navigation"
	case "main":
		return "main"
	case "header":
		return "banner"
	case "footer":
		return "contentinfo"
	case "aside":
		return "complementary"
	case "article":
		return "article"
	case "section":
		return "region"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "heading"
	case "ul", "ol":
		return "list"
	case "li":
		return "listitem"
	case "table":
		return "table"
	case "tr":
		return "row"
	case "th":
		return "columnheader"
	case "td":
		return "cell"
	default:
		return ""
	}
}

func locatorFieldValues(kind structuredLocatorKind, el types.ElementDescriptor) []string {
	switch kind {
	case locatorText:
		if strings.TrimSpace(el.Text) != "" {
			return []string{el.Text}
		}
		return []string{el.Name}
	case locatorLabel:
		return []string{el.Label, el.Positional.LabelledBy}
	case locatorPlaceholder:
		return []string{el.Placeholder}
	case locatorAlt:
		if strings.TrimSpace(el.Alt) != "" {
			return []string{el.Alt}
		}
		if isImageRole(el.Role) {
			return []string{el.Name}
		}
		return nil
	case locatorTitle:
		return []string{el.Title}
	case locatorTestID:
		return []string{el.TestID}
	default:
		return nil
	}
}

func isImageRole(role string) bool {
	role = normalizeLocatorText(role)
	return role == "img" || role == "image"
}

func scoreLocatorValues(needle string, values []string) (float64, bool) {
	want := normalizeLocatorText(needle)
	if want == "" {
		return 0, false
	}

	best := 0.0
	for _, value := range values {
		have := normalizeLocatorText(value)
		if have == "" {
			continue
		}
		switch {
		case have == want:
			return 1.0, true
		case strings.Contains(have, want):
			if best < 0.78 {
				best = 0.78
			}
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, true
}

func normalizeLocatorText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func structuredElementOrder(idx int, el types.ElementDescriptor) int {
	if el.DocumentIdx > 0 {
		return el.DocumentIdx
	}
	return idx
}
