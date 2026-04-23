package engine

import (
	"sort"
	"strings"

	"github.com/pinchtab/semantic/internal/types"
)

const (
	visualDirectionalBoost = 0.12
	visualRelativeBoost    = 0.16
	visualRelativePenalty  = 0.05
	visualBoostCap         = 0.30
)

type visualQueryHints struct {
	hasHints    bool
	baseQuery   string
	top         bool
	bottom      bool
	left        bool
	right       bool
	aboveAnchor string
	belowAnchor string
}

var visualKeywordSet = map[string]bool{
	"top":    true,
	"bottom": true,
	"left":   true,
	"right":  true,
	"corner": true,
	"above":  true,
	"below":  true,
	"under":  true,
	"over":   true,
	"in":     true,
	"on":     true,
	"at":     true,
	"the":    true,
	"a":      true,
	"an":     true,
	"of":     true,
	"page":   true,
	"side":   true,
}

func parseVisualQueryHints(query string) visualQueryHints {
	cleaned := strings.TrimSpace(query)
	if cleaned == "" {
		return visualQueryHints{}
	}

	words := tokenSet(tokenize(cleaned))
	hints := visualQueryHints{
		top:       words["top"],
		bottom:    words["bottom"],
		left:      words["left"],
		right:     words["right"],
		baseQuery: cleaned,
	}
	hasDirectional := hints.top || hints.bottom || hints.left || hints.right || words["corner"]
	hasRelative := false

	lower := strings.ToLower(cleaned)
	if idx := strings.Index(lower, " below "); idx >= 0 {
		hints.belowAnchor = normalizeVisualAnchor(cleaned[idx+len(" below "):])
		hasRelative = true
		if base := strings.TrimSpace(cleaned[:idx]); base != "" {
			hints.baseQuery = stripVisualKeywords(base)
		}
	}
	if idx := strings.Index(lower, " under "); idx >= 0 {
		hints.belowAnchor = normalizeVisualAnchor(cleaned[idx+len(" under "):])
		hasRelative = true
		if base := strings.TrimSpace(cleaned[:idx]); base != "" {
			hints.baseQuery = stripVisualKeywords(base)
		}
	}
	if idx := strings.Index(lower, " above "); idx >= 0 {
		hints.aboveAnchor = normalizeVisualAnchor(cleaned[idx+len(" above "):])
		hasRelative = true
		if base := strings.TrimSpace(cleaned[:idx]); base != "" {
			hints.baseQuery = stripVisualKeywords(base)
		}
	}
	if idx := strings.Index(lower, " over "); idx >= 0 {
		hints.aboveAnchor = normalizeVisualAnchor(cleaned[idx+len(" over "):])
		hasRelative = true
		if base := strings.TrimSpace(cleaned[:idx]); base != "" {
			hints.baseQuery = stripVisualKeywords(base)
		}
	}

	if !hasRelative && hasDirectional {
		hints.baseQuery = stripVisualKeywords(cleaned)
	}
	if strings.TrimSpace(hints.baseQuery) == "" {
		hints.baseQuery = cleaned
	}

	hints.hasHints = hasDirectional || hints.aboveAnchor != "" || hints.belowAnchor != ""
	return hints
}

func stripVisualKeywords(query string) string {
	parts := tokenize(query)
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if !visualKeywordSet[p] {
			filtered = append(filtered, p)
		}
	}
	return strings.TrimSpace(strings.Join(filtered, " "))
}

func normalizeVisualAnchor(s string) string {
	anchor := stripVisualKeywords(s)
	if anchor == "" {
		anchor = strings.TrimSpace(strings.ToLower(s))
	}
	return anchor
}

type spatialStats struct {
	hasX bool
	hasY bool
	minX float64
	maxX float64
	minY float64
	maxY float64
}

func buildSpatialStats(elements []types.ElementDescriptor) spatialStats {
	stats := spatialStats{}
	for _, el := range elements {
		h := el.Positional
		if hasHorizontalPosition(h) {
			x := horizontalPosition(h)
			if !stats.hasX {
				stats.hasX = true
				stats.minX, stats.maxX = x, x
			} else {
				if x < stats.minX {
					stats.minX = x
				}
				if x > stats.maxX {
					stats.maxX = x
				}
			}
		}
		if hasVerticalPosition(h) {
			y := verticalPosition(h)
			if !stats.hasY {
				stats.hasY = true
				stats.minY, stats.maxY = y, y
			} else {
				if y < stats.minY {
					stats.minY = y
				}
				if y > stats.maxY {
					stats.maxY = y
				}
			}
		}
	}
	return stats
}

func applyVisualHintBoost(result types.FindResult, hints visualQueryHints, elements []types.ElementDescriptor, topK int) types.FindResult {
	if !hints.hasHints || len(result.Matches) == 0 {
		return result
	}

	refToElem := make(map[string]types.ElementDescriptor, len(elements))
	refOrder := make(map[string]int, len(elements))
	for i, el := range elements {
		refToElem[el.Ref] = el
		refOrder[el.Ref] = i
	}

	stats := buildSpatialStats(elements)
	anchorRef := ""
	if hints.aboveAnchor != "" {
		anchorRef = findVisualAnchorRef(hints.aboveAnchor, elements)
	} else if hints.belowAnchor != "" {
		anchorRef = findVisualAnchorRef(hints.belowAnchor, elements)
	}

	type boostedMatch struct {
		match types.ElementMatch
		order int
	}

	boosted := make([]boostedMatch, 0, len(result.Matches))
	for _, match := range result.Matches {
		el, ok := refToElem[match.Ref]
		if !ok {
			continue
		}
		order := refOrder[match.Ref]
		boost := computeVisualBoost(el, order, len(elements), hints, stats, anchorRef, refToElem, refOrder)
		if boost > visualBoostCap {
			boost = visualBoostCap
		}
		if boost < -visualBoostCap {
			boost = -visualBoostCap
		}
		match.Score += boost
		if match.Score > 1.0 {
			match.Score = 1.0
		}
		if match.Score < 0 {
			match.Score = 0
		}
		boosted = append(boosted, boostedMatch{match: match, order: order})
	}

	sort.SliceStable(boosted, func(i, j int) bool {
		diff := boosted[i].match.Score - boosted[j].match.Score
		if diff > 1e-9 || diff < -1e-9 {
			return diff > 0
		}
		if boosted[i].order != boosted[j].order {
			return boosted[i].order < boosted[j].order
		}
		return boosted[i].match.Ref < boosted[j].match.Ref
	})

	if topK > 0 && len(boosted) > topK {
		boosted = boosted[:topK]
	}

	result.Matches = result.Matches[:0]
	for _, bm := range boosted {
		result.Matches = append(result.Matches, bm.match)
	}
	if len(result.Matches) > 0 {
		result.BestRef = result.Matches[0].Ref
		result.BestScore = result.Matches[0].Score
	} else {
		result.BestRef = ""
		result.BestScore = 0
	}
	return result
}

func computeVisualBoost(
	el types.ElementDescriptor,
	order int,
	total int,
	hints visualQueryHints,
	stats spatialStats,
	anchorRef string,
	refToElem map[string]types.ElementDescriptor,
	refOrder map[string]int,
) float64 {
	xRatio := horizontalRatio(el.Positional, stats, order, total)
	yRatio := verticalRatio(el.Positional, stats, order, total)

	boost := 0.0
	if hints.top {
		boost += visualDirectionalBoost * (1 - yRatio)
	}
	if hints.bottom {
		boost += visualDirectionalBoost * yRatio
	}
	if hints.left {
		boost += visualDirectionalBoost * (1 - xRatio)
	}
	if hints.right {
		boost += visualDirectionalBoost * xRatio
	}

	if anchorRef != "" && anchorRef != el.Ref {
		anchorEl, ok := refToElem[anchorRef]
		if ok {
			anchorOrder := refOrder[anchorRef]
			anchorY := verticalRatio(anchorEl.Positional, stats, anchorOrder, total)
			if hints.aboveAnchor != "" {
				if yRatio < anchorY {
					boost += visualRelativeBoost
				} else {
					boost -= visualRelativePenalty
				}
			}
			if hints.belowAnchor != "" {
				if yRatio > anchorY {
					boost += visualRelativeBoost
				} else {
					boost -= visualRelativePenalty
				}
			}
		}
	}

	return boost
}

func findVisualAnchorRef(anchorQuery string, elements []types.ElementDescriptor) string {
	if strings.TrimSpace(anchorQuery) == "" {
		return ""
	}
	bestRef := ""
	bestScore := 0.0
	for _, el := range elements {
		anchorContext := strings.TrimSpace(el.Composite() + " " + el.Parent + " " + el.Section)
		score := lexicalScore(anchorQuery, anchorContext, false, nil)
		if score > bestScore {
			bestScore = score
			bestRef = el.Ref
		}
	}
	if bestScore < 0.2 {
		return ""
	}
	return bestRef
}

func horizontalRatio(h types.PositionalHints, stats spatialStats, order, total int) float64 {
	if stats.hasX && hasHorizontalPosition(h) {
		x := horizontalPosition(h)
		if stats.maxX > stats.minX {
			return (x - stats.minX) / (stats.maxX - stats.minX)
		}
		return 0.5
	}
	return fallbackOrderRatio(h, order, total)
}

func verticalRatio(h types.PositionalHints, stats spatialStats, order, total int) float64 {
	if stats.hasY && hasVerticalPosition(h) {
		y := verticalPosition(h)
		if stats.maxY > stats.minY {
			return (y - stats.minY) / (stats.maxY - stats.minY)
		}
		return 0.5
	}
	return fallbackOrderRatio(h, order, total)
}

func fallbackOrderRatio(h types.PositionalHints, order, total int) float64 {
	if h.SiblingCount > 1 {
		idx := h.SiblingIndex
		if idx < 0 {
			idx = 0
		}
		if idx > h.SiblingCount-1 {
			idx = h.SiblingCount - 1
		}
		return float64(idx) / float64(h.SiblingCount-1)
	}
	if total > 1 {
		return float64(order) / float64(total-1)
	}
	return 0.5
}

func hasHorizontalPosition(h types.PositionalHints) bool {
	return h.Width > 0 || h.X != 0
}

func hasVerticalPosition(h types.PositionalHints) bool {
	return h.Height > 0 || h.Y != 0
}

func horizontalPosition(h types.PositionalHints) float64 {
	return h.X + (h.Width / 2)
}

func verticalPosition(h types.PositionalHints) float64 {
	return h.Y + (h.Height / 2)
}
