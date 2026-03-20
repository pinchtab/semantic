package recovery

import (
	"sync"
	"time"

	"github.com/pinchtab/semantic"
)

// IntentEntry captures what the agent meant by a ref at action time.
type IntentEntry struct {
	// Query is the original natural-language query (if the action was
	// preceded by /find). Otherwise empty.
	Query string

	// Descriptor holds role, name, and value of the element at
	// action time.
	Descriptor semantic.ElementDescriptor

	// Score and Confidence from the last /find (if available).
	Score      float64
	Confidence string
	Strategy   string

	// CachedAt is the wall-clock time the entry was stored.
	CachedAt time.Time
}

// IntentCache is a thread-safe, per-tab LRU cache of element intents.
type IntentCache struct {
	mu      sync.RWMutex
	tabs    map[string]map[string]IntentEntry
	maxRefs int           // max entries per tab
	ttl     time.Duration // entry expiry
}

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

func (c *IntentCache) InvalidateTab(tabID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tabs, tabID)
}

func (c *IntentCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := 0
	for _, tab := range c.tabs {
		n += len(tab)
	}
	return n
}
