---
name: semantic
description: "Use this skill when a task needs semantic element matching: find UI elements by natural language query, match accessibility tree nodes, score element similarity, classify browser automation failures, or recover from stale element refs. Triggers on 'find element', 'semantic match', 'accessibility matching', 'element recovery'."
metadata:
  openclaw:
    requires:
      bins:
        - semantic
    homepage: https://github.com/pinchtab/semantic
    install:
      - kind: brew
        formula: pinchtab/tap/semantic
        bins: [semantic]
      - kind: go
        package: github.com/pinchtab/semantic/cmd/semantic@latest
        bins: [semantic]
      - kind: npm
        package: "@pinchtab/semantic"
        bins: [semantic]
---

# Semantic Element Matching

Semantic is a zero-dependency Go library + CLI for matching natural language queries against accessibility tree elements.

## CLI Usage

### Find elements by query

```bash
# From a snapshot file
semantic find "sign in button" --snapshot page.json

# From stdin (pipe from pinchtab)
pinchtab snap --json | semantic find "login"

# Control output
semantic find "submit" --threshold 0.5 --top-k 5 --format json
semantic find "email input" --strategy lexical --format refs
```

### Score a specific element

```bash
semantic match "login button" e4 --snapshot page.json
```

### Classify an error

```bash
semantic classify "could not find node with given id"
# → element_not_found (recoverable: true)

semantic classify "net::ERR_CONNECTION_REFUSED"
# → network (recoverable: false)
```

## Snapshot Format

The CLI expects a JSON array of elements:

```json
[
  {"ref": "e0", "role": "button", "name": "Sign In"},
  {"ref": "e1", "role": "textbox", "name": "Email"},
  {"ref": "e2", "role": "link", "name": "Forgot Password"}
]
```

## Output Formats

- `--format table` (default): Human-readable table
- `--format json`: Machine-readable JSON with scores and confidence
- `--format refs`: Just ref strings, one per line (for piping)

## Strategies

- `combined` (default): 60% lexical + 40% embedding — best overall
- `lexical`: Jaccard similarity + synonym expansion + role boosting
- `embedding`: Cosine similarity on hashing-based dense vectors

## Library Usage (Go)

```go
import "github.com/pinchtab/semantic"

matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
result, _ := matcher.Find(ctx, "login button", elements, semantic.FindOptions{
    Threshold: 0.3,
    TopK:      3,
})
fmt.Println(result.BestRef, result.BestScore)
```
