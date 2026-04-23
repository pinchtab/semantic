# Contributing

## Setup

```bash
git clone https://github.com/pinchtab/semantic
cd semantic
./dev doctor
```

Doctor checks Go version, golangci-lint, dependencies, build, tests, and git hooks. It offers to install anything missing.

## Development Commands

```bash
./dev test          # run unit tests
./dev test race     # with race detector
./dev coverage      # test coverage report
./dev lint          # golangci-lint
./dev check         # all checks (fmt + vet + lint + test)
./dev build         # build CLI binary
./dev bench         # run corpus benchmark suite
```

## Project Structure

```
semantic.go              Public API (type aliases + constructors)
semantic_test.go         API-level smoke tests

internal/
  types/types.go         Type definitions (interfaces, structs)
  engine/                Matching implementations (hidden)
    combined.go          Fused lexical + embedding
    lexical.go           Jaccard + synonyms + role boosting
    embedding.go         Cosine similarity on dense vectors
    hashing.go           Feature hashing embedder
    synonyms.go          54 UI synonym groups
    stopwords.go         Context-aware stopword removal

recovery/                Public subpackage
  engine.go              RecoveryEngine
  cache.go               IntentCache (per-tab LRU)
  failure.go             FailureType classification

cmd/semantic/main.go     CLI tool
```

## Key Rules

- **Implementations go in `internal/engine/`** — never export internal types
- **Types go in `internal/types/`** — re-exported via aliases in `semantic.go`
- **Public API changes require updating `semantic.go`** — the thin wrapper layer
- **Zero dependencies** — no external imports in the matching engine
- **Tests go with the code** — `internal/engine/*_test.go` for unit tests, `semantic_test.go` for API tests

## Pre-commit Hook

Runs automatically on staged `.go` files:
1. `gofmt` — formatting check
2. `golangci-lint` — lint check

Install with `./dev doctor` or manually: `./scripts/install-hooks.sh`

## Adding a Synonym Group

Edit `internal/engine/synonyms.go`, add to the `synonymGroups` slice:

```go
{"new_word", "existing_synonym", "another_synonym"},
```

Then run `./dev test` to verify no regressions.

## Adding a Matcher

1. Create `internal/engine/my_matcher.go` implementing `types.ElementMatcher`
2. Add tests in `internal/engine/my_matcher_test.go`
3. Add constructor in `semantic.go`: `func NewMyMatcher() ElementMatcher`
4. Run `./dev check`
