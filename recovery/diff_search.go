package recovery

import (
	"strconv"
	"strings"
	"sync"

	"github.com/pinchtab/semantic"
)

// RecoverySearchStats exposes diff-first recovery search metrics.
type RecoverySearchStats struct {
	DiffAttempts int     `json:"diff_attempts"`
	DiffHits     int     `json:"diff_hits"`
	FullSearches int     `json:"full_searches"`
	DiffHitRate  float64 `json:"diff_hit_rate"`
}

// RecoverySearchTracker records diff-first vs full-search behavior.
type RecoverySearchTracker struct {
	mu           sync.Mutex
	diffAttempts int
	diffHits     int
	fullSearches int
}

func NewRecoverySearchTracker() *RecoverySearchTracker {
	return &RecoverySearchTracker{}
}

func (rt *RecoverySearchTracker) RecordDiffAttempt() {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	rt.diffAttempts++
	rt.mu.Unlock()
}

func (rt *RecoverySearchTracker) RecordDiffHit() {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	rt.diffHits++
	rt.mu.Unlock()
}

func (rt *RecoverySearchTracker) RecordFullSearch() {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	rt.fullSearches++
	rt.mu.Unlock()
}

func (rt *RecoverySearchTracker) Stats() RecoverySearchStats {
	if rt == nil {
		return RecoverySearchStats{}
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	stats := RecoverySearchStats{
		DiffAttempts: rt.diffAttempts,
		DiffHits:     rt.diffHits,
		FullSearches: rt.fullSearches,
	}
	if stats.DiffAttempts > 0 {
		stats.DiffHitRate = float64(stats.DiffHits) / float64(stats.DiffAttempts)
	}
	return stats
}

// diffDescriptors returns descriptors that are either new or changed in `curr`
// relative to `prev`, plus descriptors that were removed from `prev`.
func diffDescriptors(prev, curr []semantic.ElementDescriptor) ([]semantic.ElementDescriptor, []semantic.ElementDescriptor) {
	prevByRef := make(map[string]semantic.ElementDescriptor, len(prev))
	for _, d := range prev {
		if d.Ref == "" {
			continue
		}
		prevByRef[d.Ref] = d
	}

	changedOrAdded := make([]semantic.ElementDescriptor, 0)
	seen := make(map[string]bool, len(curr))
	for _, d := range curr {
		if d.Ref == "" {
			changedOrAdded = append(changedOrAdded, d)
			continue
		}
		seen[d.Ref] = true
		if p, ok := prevByRef[d.Ref]; !ok || descriptorFingerprint(p) != descriptorFingerprint(d) {
			changedOrAdded = append(changedOrAdded, d)
		}
	}

	removed := make([]semantic.ElementDescriptor, 0)
	for _, d := range prev {
		if d.Ref == "" {
			continue
		}
		if !seen[d.Ref] {
			removed = append(removed, d)
		}
	}

	return changedOrAdded, removed
}

func descriptorFingerprint(d semantic.ElementDescriptor) string {
	var b strings.Builder
	b.WriteString(d.Ref)
	b.WriteString("|")
	b.WriteString(d.Role)
	b.WriteString("|")
	b.WriteString(d.Name)
	b.WriteString("|")
	b.WriteString(d.Value)
	b.WriteString("|")
	b.WriteString(d.Parent)
	b.WriteString("|")
	b.WriteString(d.Section)
	b.WriteString("|")
	b.WriteString(strconv.FormatBool(d.Interactive))
	return b.String()
}
