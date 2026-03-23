package recovery

import (
	"testing"
	"time"

	"github.com/pinchtab/semantic"
)

func TestIntentCache_StoreAndLookup(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	entry := IntentEntry{
		Query:      "submit button",
		Descriptor: semantic.ElementDescriptor{Ref: "e1", Role: "button", Name: "Submit"},
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	}
	c.Store("tab1", "e1", entry)

	got, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup returned false for stored entry")
	}
	if got.Query != "submit button" {
		t.Errorf("Query = %q, want %q", got.Query, "submit button")
	}
	if got.Descriptor.Role != "button" {
		t.Errorf("Descriptor.Role = %q, want %q", got.Descriptor.Role, "button")
	}
	if got.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", got.Score)
	}
}

func TestIntentCache_LookupMiss(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("Lookup returned true for missing entry")
	}
	// Wrong tab.
	c.Store("tab1", "e1", IntentEntry{Query: "test"})
	_, ok = c.Lookup("tab2", "e1")
	if ok {
		t.Error("Lookup returned true for wrong tab")
	}
}

func TestIntentCache_TTLExpiry(t *testing.T) {
	c := NewIntentCache(100, 50*time.Millisecond)

	c.Store("tab1", "e1", IntentEntry{Query: "test", CachedAt: time.Now()})

	// Should be found immediately.
	_, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup should find fresh entry")
	}

	// Wait for expiry.
	time.Sleep(60 * time.Millisecond)
	_, ok = c.Lookup("tab1", "e1")
	if ok {
		t.Error("Lookup should return false after TTL expiry")
	}
}

func TestIntentCache_LRUEviction(t *testing.T) {
	c := NewIntentCache(3, 5*time.Minute)

	// Fill to capacity.
	c.Store("tab1", "e1", IntentEntry{Query: "first", CachedAt: time.Now().Add(-3 * time.Minute)})
	c.Store("tab1", "e2", IntentEntry{Query: "second", CachedAt: time.Now().Add(-2 * time.Minute)})
	c.Store("tab1", "e3", IntentEntry{Query: "third", CachedAt: time.Now().Add(-1 * time.Minute)})

	// Add one more — oldest (e1) should be evicted.
	c.Store("tab1", "e4", IntentEntry{Query: "fourth"})

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("e1 should have been evicted (oldest)")
	}
	_, ok = c.Lookup("tab1", "e4")
	if !ok {
		t.Error("e4 should be present after eviction")
	}
}

func TestIntentCache_InvalidateTab(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	c.Store("tab1", "e1", IntentEntry{Query: "test"})
	c.Store("tab1", "e2", IntentEntry{Query: "test2"})
	c.Store("tab2", "e1", IntentEntry{Query: "other"})

	c.InvalidateTab("tab1")

	_, ok := c.Lookup("tab1", "e1")
	if ok {
		t.Error("tab1/e1 should be gone after InvalidateTab")
	}
	_, ok = c.Lookup("tab2", "e1")
	if !ok {
		t.Error("tab2/e1 should still be present")
	}
}

func TestIntentCache_Size(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)
	if c.Size() != 0 {
		t.Errorf("Size = %d, want 0", c.Size())
	}

	c.Store("tab1", "e1", IntentEntry{Query: "a"})
	c.Store("tab1", "e2", IntentEntry{Query: "b"})
	c.Store("tab2", "e3", IntentEntry{Query: "c"})
	if c.Size() != 3 {
		t.Errorf("Size = %d, want 3", c.Size())
	}
}

func TestIntentCache_UpdateExisting(t *testing.T) {
	c := NewIntentCache(100, 5*time.Minute)

	c.Store("tab1", "e1", IntentEntry{Query: "old query"})
	c.Store("tab1", "e1", IntentEntry{Query: "new query"})

	got, ok := c.Lookup("tab1", "e1")
	if !ok {
		t.Fatal("Lookup should find updated entry")
	}
	if got.Query != "new query" {
		t.Errorf("Query = %q, want %q", got.Query, "new query")
	}
	if c.Size() != 1 {
		t.Errorf("Size = %d, want 1 (update should not add)", c.Size())
	}
}

func TestIntentCache_DefaultValues(t *testing.T) {
	// Zero/negative values should be replaced with defaults.
	c := NewIntentCache(0, 0)
	if c.maxRefs != 200 {
		t.Errorf("maxRefs = %d, want 200", c.maxRefs)
	}
	if c.ttl != 10*time.Minute {
		t.Errorf("ttl = %v, want 10m", c.ttl)
	}
}
