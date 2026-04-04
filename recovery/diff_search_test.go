package recovery

import (
	"testing"

	"github.com/pinchtab/semantic"
)

func TestDiffDescriptors_DetectsAddedChangedRemoved(t *testing.T) {
	prev := []semantic.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
		{Ref: "e2", Role: "textbox", Name: "Email"},
	}
	curr := []semantic.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit now"}, // changed
		{Ref: "e3", Role: "button", Name: "Cancel"},     // added
	}

	diff, removed := diffDescriptors(prev, curr)
	if len(diff) != 2 {
		t.Fatalf("expected 2 diff descriptors, got %d", len(diff))
	}
	if len(removed) != 1 || removed[0].Ref != "e2" {
		t.Fatalf("expected e2 removed, got %+v", removed)
	}
}

func TestRecoverySearchTracker_Stats(t *testing.T) {
	tracker := NewRecoverySearchTracker()
	tracker.RecordDiffAttempt()
	tracker.RecordDiffHit()
	tracker.RecordFullSearch()

	stats := tracker.Stats()
	if stats.DiffAttempts != 1 {
		t.Fatalf("expected diff attempts=1, got %d", stats.DiffAttempts)
	}
	if stats.DiffHits != 1 {
		t.Fatalf("expected diff hits=1, got %d", stats.DiffHits)
	}
	if stats.FullSearches != 1 {
		t.Fatalf("expected full searches=1, got %d", stats.FullSearches)
	}
	if stats.DiffHitRate != 1.0 {
		t.Fatalf("expected diff hit rate=1.0, got %f", stats.DiffHitRate)
	}
}
