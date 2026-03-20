# semantic

Zero-dependency Go library for semantic matching of accessibility tree elements.

Matches natural language queries ("sign in button") against UI element descriptors
using lexical similarity, synonym expansion, and embedding-based fuzzy matching.

## Install

```bash
go get github.com/pinchtab/semantic
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

## Features

- **Lexical matching** — Jaccard similarity with stopword removal, context-aware filtering, prefix matching
- **Synonym expansion** — 60+ UI synonym groups ("sign in" ↔ "log in", "cart" ↔ "basket", etc.)
- **Embedding matching** — Feature-hashing embedder with character n-grams for sub-word similarity
- **Combined matching** — Weighted fusion of lexical + embedding for best accuracy
- **Confidence calibration** — Score → high/medium/low labels
- **Error classification** — Classify browser errors as recoverable or not
- **Self-healing recovery** — Re-locate stale elements after DOM changes via callback interfaces
- **Intent caching** — Per-tab LRU cache of element intents for recovery

## Matchers

| Matcher | Speed | Accuracy | Use case |
|---------|-------|----------|----------|
| `NewLexicalMatcher()` | Fastest | Good for exact/synonym matches | Simple UIs, speed-critical |
| `NewCombinedMatcher(embedder)` | Fast | Best overall | General purpose (recommended) |
| `NewEmbeddingMatcher(embedder)` | Fast | Good for fuzzy matches | Sub-word similarity |

## CLI

```bash
go install github.com/pinchtab/semantic/cmd/semantic@latest

# Find elements matching a query
semantic find "sign in button" --snapshot page.json

# Pipe from pinchtab
curl -s localhost:9999/snapshot | semantic find "search box"

# Classify an error
semantic classify "could not find node with given id"
# → element_not_found (recoverable: true)
```

## Zero Dependencies

The library uses only Go standard library. No external dependencies, no supply chain risk.

## Origin

This project was extracted from [pinchtab](https://github.com/pinchtab/pinchtab)'s internal semantic matching package. The original implementation was contributed by [@YashJadhav21](https://github.com/YashJadhav21), [@Djain912](https://github.com/Djain912), and [@Chetnapadhi](https://github.com/Chetnapadhi) in a collaboration that built the lexical matching, synonym expansion, embedding, and recovery systems that form the core of this library.

## License

MIT
