# semantic

Zero-dependency Go library for semantic matching of accessibility tree elements.

Matches natural language queries ("sign in button") against UI element descriptors
using lexical similarity, synonym expansion, and embedding-based fuzzy matching.

## The Problem

Browser automation tools find UI elements via CSS selectors, XPath, or explicit IDs — all brittle. When the DOM changes (SPA re-renders, layout shifts, framework updates), selectors break silently. AI agents make this worse: they describe elements in natural language ("click the sign in button") but the accessibility tree has structured labels ("Sign In", role: button, ref: e4).

The gap between how agents describe elements and how browsers expose them is what semantic solves.

## Install

```bash
go get github.com/pinchtab/semantic
```

Or via npm (downloads the Go binary):

```bash
npx @pinchtab/semantic find "sign in" --snapshot page.json
```

Or Homebrew:

```bash
brew install pinchtab/tap/semantic
```

## Usage

```go
import "github.com/pinchtab/semantic"

// Build descriptors from your accessibility tree
elements := []semantic.ElementDescriptor{
    {Ref: "e0", Role: "button", Name: "Sign In"},
    {Ref: "e1", Role: "textbox", Name: "Email"},
    {Ref: "e2", Role: "link", Name: "Forgot password?"},
}

// Create a matcher (combined = lexical + embedding)
matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

// Find matching elements
result, err := matcher.Find(ctx, "log in button", elements, semantic.FindOptions{
    Threshold: 0.3,
    TopK:      3,
})
// result.BestRef = "e0" ("Sign In" matches "log in" via synonyms)
// result.BestScore = 0.82
```

## Package Layout

```
semantic.go              Public API (types + constructors)
internal/types/          Type definitions (interfaces, structs)
internal/engine/         Matching implementations (hidden from consumers)
recovery/                Error recovery + intent caching (public subpackage)
cmd/semantic/            CLI tool
```

Implementations are internal — consumers use the `ElementMatcher` interface and constructors. Swap matching strategies without breaking your code.

## How It Works

```
┌─────────────────────────────────────────────────────┐
│                    ElementMatcher                    │
│                    (interface)                       │
├─────────────┬───────────────────┬───────────────────┤
│  Lexical    │    Embedding      │    Combined       │
│  Matcher    │    Matcher        │    Matcher        │
│             │                   │                   │
│  Jaccard    │  Cosine sim on    │  0.6 × lexical    │
│  + synonyms │  hashed vectors   │  + 0.4 × embed   │
│  + stopwords│  (feature hashing,│                   │
│  + prefix   │   char n-grams,   │  Runs both in    │
│  + role     │   zero deps)      │  parallel, fuses │
│    boosting │                   │  by ref          │
└─────────────┴───────────────────┴───────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────┐
│                  RecoveryEngine                      │
│                                                     │
│  IntentCache → reconstruct original query           │
│  SnapshotRefresher → get fresh DOM (callback)       │
│  Re-match → find element in new snapshot            │
│  Re-execute → run the original action               │
└─────────────────────────────────────────────────────┘
```

**Lexical matching** uses Jaccard similarity with context-aware stopword removal, 54 synonym groups covering UI vocabulary (login ↔ signin, cart ↔ basket, submit ↔ send), prefix matching for abbreviations ("btn" → "button"), and role boosting when queries mention ARIA roles.

**Embedding matching** uses feature hashing (the "hashing trick", Weinberger et al. 2009) to convert text into fixed-dimension vectors via word unigrams and character n-grams. No vocabulary construction, no model downloads — sub-millisecond latency.

**Combined matching** (recommended) fuses both: 60% lexical + 40% embedding. Lexical handles exact matches and synonyms; embedding adds fuzzy sub-word similarity for partial queries.

## Features

- **Synonym expansion** — 54 UI synonym groups ("sign in" ↔ "log in", "cart" ↔ "basket", "preferences" ↔ "settings", etc.)
- **Ordinal selection** — Positional intent like `first`, `second`, `3rd`, and `last` for duplicate element sets
- **Confidence calibration** — Scores mapped to high (≥ 0.8) / medium (≥ 0.6) / low labels
- **Error classification** — Classify browser errors (CDP, chromedp) as recoverable or not
- **Self-healing recovery** — Re-locate stale elements after DOM changes via callback interfaces
- **Intent caching** — Per-tab LRU cache (200 entries, 10min TTL) of element intents for recovery

## Matchers

| Matcher | Speed | Accuracy | Use case |
|---------|-------|----------|----------|
| `NewLexicalMatcher()` | < 0.5ms / 100 elements | Best for exact + synonym | Simple UIs, speed-critical |
| `NewCombinedMatcher(embedder)` | < 1ms / 100 elements | Best overall | General purpose (recommended) |
| `NewEmbeddingMatcher(embedder)` | < 1ms / 100 elements | Best for fuzzy/partial | Sub-word similarity |

## Error Classification

```go
import "github.com/pinchtab/semantic/recovery"

ft := recovery.ClassifyFailure(err)
// ft.Recoverable() → true/false
```

| Type | Examples | Recoverable |
|------|----------|-------------|
| `element_not_found` | "could not find node", "ref not found" | ✅ |
| `element_stale` | "node is detached", "execution context destroyed" | ✅ |
| `element_not_interactable` | "not visible", "overlapped", "disabled" | ✅ |
| `navigation` | "frame detached", "page crashed" | ✅ |
| `network` | "connection refused", "timeout" | ❌ |
| `unknown` | Everything else | ❌ |

## Self-Healing Recovery

When an action fails on a stale ref, the `RecoveryEngine` reconstructs the original query from its intent cache, refreshes the DOM, re-matches, and re-executes — all through callback interfaces so it works with any browser automation framework:

```go
import (
    "github.com/pinchtab/semantic"
    "github.com/pinchtab/semantic/recovery"
)

intentCache := recovery.NewIntentCache(200, 10*time.Minute)

re := recovery.NewRecoveryEngine(
    recovery.DefaultRecoveryConfig(),
    matcher,
    intentCache,
    refreshSnapshot,  // your callback to refresh the DOM
    resolveNodeID,    // your callback to map ref → node ID
    buildDescriptors, // your callback to build descriptors
)

// Cache intent after successful find
re.RecordIntent(tabID, "e5", recovery.IntentEntry{
    Query:      "checkout button",
    Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Checkout"},
})

// Recover on failure
if err != nil && re.ShouldAttempt(err, ref) {
    rr, _, _ := re.Attempt(ctx, tabID, ref, "click", executeAction)
    // rr.Recovered = true, rr.NewRef = "e12"
}
```

## CLI

```bash
go install github.com/pinchtab/semantic/cmd/semantic@latest

# Find elements matching a query
semantic find "sign in button" --snapshot page.json

# Pipe from pinchtab or any tool that outputs accessibility JSON
curl -s localhost:9999/snapshot | semantic find "search box"

# Output formats
semantic find "login" --snapshot page.json --format json    # machine-readable
semantic find "login" --snapshot page.json --format table   # human-readable
semantic find "login" --snapshot page.json --format refs    # just refs

# Select by order among duplicate matches
semantic find "second button" --snapshot page.json
semantic find "last input field" --snapshot page.json

# Score a specific element
semantic match "login" e4 --snapshot page.json

# Classify an error
semantic classify "could not find node with given id"
# → element_not_found (recoverable: true)
```

## Zero Dependencies

The library uses only the Go standard library. No external dependencies, no model downloads, no supply chain risk. The hashing-based embedder gets ~85% of the quality of real ML embeddings with zero cost.

## Design Trade-offs

See [docs/DESIGN.md](docs/DESIGN.md) for detailed discussion of architectural decisions: hashing vs real embeddings, fixed synonym table vs learned, Jaccard vs TF-IDF, and recovery callbacks vs direct integration.

## Origin

This project was extracted from [pinchtab](https://github.com/pinchtab/pinchtab)'s internal semantic matching package. The original implementation was contributed by [@YashJadhav21](https://github.com/YashJadhav21), [@Djain912](https://github.com/Djain912), and [@Chetnapadhi](https://github.com/Chetnapadhi) in [PR #109](https://github.com/pinchtab/pinchtab/pull/109) — a collaboration that built the lexical matching, synonym expansion, embedding, and recovery systems that form the core of this library.

## License

MIT
