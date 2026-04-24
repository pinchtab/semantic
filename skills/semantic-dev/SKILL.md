---
name: semantic-dev
description: Develop and contribute to the Semantic project. Use when working on semantic source code, adding features, fixing bugs, running tests, or preparing PRs. Triggers on "work on semantic", "semantic development", "contribute to semantic", "fix semantic bug", "add semantic feature".
---

# Semantic Development

Semantic is a zero-dependency Go library for matching natural language queries against accessibility tree elements.

## Project Location

```bash
cd ~/dev/semantic
```

## Dev Commands

```bash
# Before opening a PR (runs all checks + e2e + benchmark)
./dev pr

# Quick iteration
./dev test              # unit tests
./dev check             # fmt + vet + lint + test race

# Benchmarking
./dev bench             # corpus benchmark
./dev baseline          # create baseline (first time)
./dev baseline check    # check for regressions

# Other
./dev build             # build ./semantic binary
./dev e2e               # e2e tests (Docker)
./dev lint corpus       # validate benchmark data
./dev calibrate         # find optimal thresholds
./dev tune              # grid-search weights
```

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
