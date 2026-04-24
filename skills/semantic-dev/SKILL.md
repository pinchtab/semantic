---
name: semantic-dev
description: Develop and contribute to the Semantic project. Use when working on semantic source code, adding features, fixing bugs, running tests, or preparing PRs. Triggers on "work on semantic", "semantic development", "contribute to semantic", "fix semantic bug", "add semantic feature".
---

# Semantic Development

Zero-dependency Go library for matching natural language queries against accessibility tree elements.

## Essential Commands

**Before any PR:**
```bash
./dev pr                # runs: check + e2e + lint corpus + bench
```

**During development:**
```bash
./dev test              # unit tests (fast)
./dev check             # fmt + vet + lint + test race (full validation)
./dev build             # build ./semantic CLI binary
```

**Quality regression checks:**
```bash
./dev baseline check    # compare quality against baseline
./dev runtime           # compare performance against baseline
```

**When quality changes intentionally:**
```bash
./dev baseline update   # accept new quality baseline (after review)
```

## When to Use Each

| Scenario | Command |
|----------|---------|
| Made code changes, quick sanity | `./dev test` |
| Ready to commit | `./dev check` |
| Before opening PR | `./dev pr` |
| Changed scoring/matching logic | `./dev baseline check` |
| Performance-sensitive changes | `./dev runtime` |
| Tuning weights | `./dev tune` then `./dev bench` |

## Architecture

```
semantic.go               Public API (type aliases + constructors)
semantic_test.go           API-level smoke tests

internal/
  types/types.go           Type definitions (interfaces, structs)
  engine/                  All matching implementations (hidden)
    combined.go              Fused lexical + embedding matcher
    lexical.go               Jaccard similarity + synonyms + role boosting
    embedding.go             Cosine similarity on dense vectors
    hashing.go               Feature hashing embedder (zero-dep)
    synonyms.go              54 synonym groups for UI vocabulary
    stopwords.go             Context-aware stopword removal

recovery/                  Public subpackage
  engine.go                  RecoveryEngine (semantic re-matching)
  cache.go                   IntentCache (per-tab LRU)
  failure.go                 FailureType classification

cmd/semantic/main.go       CLI tool (find, match, classify)
```

## Key Design Decisions

- **`internal/`** — implementations are hidden. Consumers only see `ElementMatcher` interface + constructors.
- **`recovery/`** — public subpackage. Pinchtab imports both `semantic` and `semantic/recovery`.
- **Zero dependencies** — hashing embedder, no ML models, no network calls.
- **Stateless** — every `Find()` call is independent. Thread-safe by default.

## Workflow: New Feature or Bug Fix

1. **Run doctor first:**
   ```bash
   ./dev doctor
   ```

2. **Make changes** — implementations go in `internal/engine/`, types in `internal/types/`

3. **Run checks:**
   ```bash
   ./dev check   # fmt + vet + lint + test with race
   ```

4. **Pre-commit hook** runs gofmt + golangci-lint automatically on staged files.

## Public API Surface

Only these symbols are visible to consumers:

```go
// Types (from internal/types via aliases)
semantic.ElementMatcher      // interface
semantic.Embedder            // interface
semantic.ElementDescriptor   // struct
semantic.ElementMatch        // struct
semantic.FindOptions         // struct
semantic.FindResult          // struct

// Constructors
semantic.NewCombinedMatcher(embedder)  → ElementMatcher
semantic.NewHashingEmbedder(dim)       → Embedder
semantic.NewLexicalMatcher()           → ElementMatcher
semantic.NewEmbeddingMatcher(e)        → ElementMatcher

// Functions
semantic.CalibrateConfidence(score)    → string
semantic.LexicalScore(query, desc)     → float64
semantic.CosineSimilarity(a, b)        → float64

// Recovery subpackage
recovery.IntentCache, recovery.IntentEntry
recovery.RecoveryEngine, recovery.RecoveryResult
recovery.ClassifyFailure, recovery.DefaultRecoveryConfig
```

## Testing

- **167 tests** across 3 packages (root, engine, recovery)
- `internal/engine/` has unit tests for all matchers + benchmark suite
- Root has API-level smoke tests
- `recovery/` has scenario tests (SPA re-render, checkout, login, etc.)

## Release Process

1. Update `npm/package.json` version
2. `git tag v0.X.0 && git push origin v0.X.0`
3. CI builds binaries (GoReleaser) + opens homebrew tap PR
4. npm publish manually or via `release-manual-publish.yml`
