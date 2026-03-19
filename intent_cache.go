package semantic

import (
	"sync"
	"time"
)

// IntentEntry captures the semantic identity of an element at the time an
// action was requested. When the element's ref becomes stale, the
// recovery engine uses this cached intent to build a semantic search
// query against the fresh snapshot.
type IntentEntry struct {
	// Query is the original natural-language query (if the action was
	// preceded by /find). Otherwise empty.
	Query string

	// Descriptor holds role, name, and value of the element at
	// action time.
	Descriptor ElementDescriptor

	// Score and Confidence from the last /find (if available).
	Score      float64
	Confidence string
	Strategy   string

	// CachedAt is the wall-clock time the entry was stored.
	CachedAt time.Time
}

// IntentCache is a thread-safe, per-tab LRU cache of element intents.
// Key hierarchy: tabID → ref → IntentEntry.
type IntentCache struct {
	mu      sync.RWMutex
	tabs    map[string]map[string]IntentEntry
	maxRefs int           // max entries per tab
	ttl     time.Duration // entry expiry
}

// NewIntentCache returns an IntentCache that evicts entries older than
// ttl and limits each tab to maxRefs entries.
func NewIntentCache(maxRefs int, ttl time.Duration) *IntentCache {
	if maxRefs <= 0 {
		maxRefs = 200
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &IntentCache{
		tabs:    make(map[string]map[string]IntentEntry),
		maxRefs: maxRefs,
		ttl:     ttl,
	}
}

// Store records (or updates) the intent for a given (tabID, ref) pair.
func (c *IntentCache) Store(tabID, ref string, entry IntentEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry.CachedAt.IsZero() {
		entry.CachedAt = time.Now()
	}

	tab, ok := c.tabs[tabID]
	if !ok {
		tab = make(map[string]IntentEntry)
		c.tabs[tabID] = tab
	}

	// Evict if at capacity.
	if len(tab) >= c.maxRefs {
		oldest := ""
		var oldestT time.Time
		for r, e := range tab {
			if oldest == "" || e.CachedAt.Before(oldestT) {
				oldest = r
				oldestT = e.CachedAt
			}
		}
		if oldest != "" {
			delete(tab, oldest)
		}
	}

	tab[ref] = entry
}

// Lookup returns the cached intent for the ref, or (IntentEntry{}, false).
func (c *IntentCache) Lookup(tabID, ref string) (IntentEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tab, ok := c.tabs[tabID]
	if !ok {
		return IntentEntry{}, false
	}
	entry, ok := tab[ref]
	if !ok {
		return IntentEntry{}, false
	}
	if time.Since(entry.CachedAt) > c.ttl {
		return IntentEntry{}, false
	}
	return entry, true
}

// InvalidateTab removes all cached intents for a tab.
func (c *IntentCache) InvalidateTab(tabID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tabs, tabID)
}

// Size returns the total number of cached entries across all tabs.
func (c *IntentCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := 0
	for _, tab := range c.tabs {
		n += len(tab)
	}
	return n
}
