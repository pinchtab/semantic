# PLAN.md — Semantic: Extraction & Standalone Library

## What This Is

Extract `pinchtab/internal/semantic` into a standalone Go library + CLI tool.

**Module:** `github.com/pinchtab/semantic`

The package provides zero-dependency semantic matching for accessibility tree elements:
lexical (Jaccard + synonyms + stopwords), embedding (hashing trick, cosine similarity),
combined (weighted fusion), self-healing recovery, intent caching, and confidence calibration.

**154 existing tests, ~6.5k lines of code, zero external dependencies.**

---

## Package Layout

**Root package.** Import as:

```go
import "github.com/pinchtab/semantic"
```

Use as `semantic.NewCombinedMatcher()`, `semantic.ElementDescriptor{}`, etc.

This follows the standard Go convention for single-purpose libraries (same pattern as
`github.com/go-chi/chi`, `go.uber.org/zap`, `github.com/charmbracelet/bubbletea`).
No `pkg/` subdirectory — the Go community moved away from that years ago. The module
IS the package. The CLI lives in `cmd/` which is the standard Go layout for binaries
alongside a library.

## Repo Structure

```
github.com/pinchtab/semantic/
├── cmd/
│   └── semantic/              # CLI binary
│       └── main.go
├── match.go                   # ElementMatcher interface, FindOptions, FindResult
├── descriptor.go              # ElementDescriptor
├── lexical.go                 # LexicalMatcher (Jaccard + stopwords + synonyms + prefix)
├── embedding.go               # EmbeddingMatcher, Embedder interface
├── hashing.go                 # HashingEmbedder (feature hashing, zero deps)
├── combined.go                # CombinedMatcher (weighted fusion)
├── calibration.go             # CalibrateConfidence
├── stopwords.go               # Stopword sets + context-aware removal
├── synonyms.go                # UI synonym table + bidirectional index
├── recovery.go                # RecoveryEngine + types (SnapshotRefresher, etc.)
├── failure.go                 # FailureType classification
├── intent_cache.go            # IntentCache (per-tab LRU with TTL)
├── match_test.go              # Unit tests: descriptor, calibration, stopwords, matchers
├── lexical_test.go            # Lexical matcher tests (synonyms, prefix, context-aware)
├── embedding_test.go          # Embedding/hashing tests
├── combined_test.go           # Combined matcher tests
├── recovery_test.go           # Recovery engine tests
├── benchmark_test.go          # Benchmark study (20 queries × 10 page types)
├── testdata/
│   ├── snapshots/             # Real accessibility tree snapshots (JSON)
│   │   ├── github-repo.json
│   │   ├── google-search.json
│   │   ├── wikipedia-article.json
│   │   ├── ecommerce-product.json
│   │   ├── login-page.json
│   │   └── dashboard.json
│   └── queries/               # Query fixtures for E2E
│       ├── basic.json         # Simple single-element queries
│       ├── synonyms.json      # Synonym resolution queries
│       └── adversarial.json   # Edge cases, empty snapshots, all-stopword queries
├── tests/
│   └── e2e/
│       ├── docker-compose.yml
│       ├── Dockerfile.runner
│       ├── run.sh             # E2E orchestrator
│       └── cases/             # E2E test case definitions
│           ├── find-basic.sh
│           ├── find-synonyms.sh
│           ├── find-threshold.sh
│           ├── find-topk.sh
│           ├── recovery-stale.sh
│           └── cli-formats.sh
├── scripts/
│   ├── check.sh               # Pre-commit: fmt + vet + lint + test
│   └── e2e.sh                 # E2E runner (local + CI)
├── dev                        # Dev tool (like pinchtab's)
├── .github/
│   └── workflows/
│       ├── ci-go.yml
│       ├── ci-e2e.yml
│       ├── reusable-go.yml
│       └── reusable-e2e.yml
├── .golangci.yml
├── go.mod
├── LICENSE
└── README.md
```

---

## Phase 1: Foundation (Library)

Move the code, make it compile, tests pass.

### 1.1 — Init module + move source files

- `go mod init github.com/pinchtab/semantic`
- Copy all `.go` files from `pinchtab/internal/semantic/`
- Rename files for clarity:
  - `matcher.go` → `match.go`
  - `lexical_matcher.go` → `lexical.go`
  - `embedding_matcher.go` → `embedding.go`
  - `hashing_embedder.go` → `hashing.go`
  - `combined_matcher.go` → `combined.go`
  - `embedder.go` — merge DummyEmbedder into `embedding.go`, Embedder interface into `match.go`
- Change `package semantic` — already correct (root package)
- Reorganize test files:
  - `semantic_test.go` → split into `match_test.go`, `lexical_test.go`, `embedding_test.go`
  - `improvements_test.go` → merge into `lexical_test.go` + `combined_test.go`
  - `recovery_test.go` → keep
  - `benchmark_study_test.go` → `benchmark_test.go`

### 1.2 — Verify

- `go build ./...`
- `go test ./... -count=1 -race`
- `go vet ./...`
- `gofmt -l .`
- All 154 tests pass, zero changes to logic

### 1.3 — Lint config

- `.golangci.yml` matching pinchtab's style
- `golangci-lint run`

**Exit criteria:** Library compiles, all tests pass, lint clean.

---

## Phase 2: CLI Tool

`cmd/semantic/main.go` — a standalone binary for matching against accessibility snapshots.

### 2.1 — Core CLI commands

```
semantic find <query> [flags]          # Find elements matching a query
semantic match <query> <ref> [flags]   # Score a specific element against a query
semantic classify <error-message>      # Classify a failure type
semantic version                       # Print version
```

### 2.2 — `semantic find`

The main command. Reads an accessibility snapshot (JSON) and finds matching elements.

```
# From file
semantic find "sign in button" --snapshot page.json

# From stdin (pipe from pinchtab, curl, etc.)
curl -s localhost:9999/snapshot | semantic find "search box"

# Control matching
semantic find "login" --snapshot page.json --threshold 0.5 --top-k 5
semantic find "login" --snapshot page.json --strategy lexical
semantic find "login" --snapshot page.json --strategy combined

# Output formats
semantic find "login" --snapshot page.json --format json     # machine-readable
semantic find "login" --snapshot page.json --format table    # human-readable (default)
semantic find "login" --snapshot page.json --format refs     # just refs, one per line
```

**Snapshot JSON format** (matches pinchtab's `/snapshot` output):
```json
[
  {"ref": "e0", "role": "button", "name": "Sign In"},
  {"ref": "e1", "role": "textbox", "name": "Email", "value": ""},
  {"ref": "e2", "role": "link", "name": "Forgot password?"}
]
```

The CLI parses this into `[]ElementDescriptor` and runs the matcher.

### 2.3 — `semantic match`

Score a single element (useful for debugging/scripting):

```
semantic match "login button" e0 --snapshot page.json
# Output: ref=e0 score=0.87 confidence=high strategy=combined
```

### 2.4 — `semantic classify`

Classify error messages for recovery decisions:

```
semantic classify "could not find node with given id"
# Output: element_not_found (recoverable: true)

echo "connection refused" | semantic classify -
# Output: network (recoverable: false)
```

### 2.5 — Snapshot parsing

Support two input formats:
1. **Array format** — `[{"ref":"e0","role":"button","name":"Submit"}, ...]`
2. **Pinchtab snapshot format** — the full accessibility tree with nested nodes (flatten to array)

Auto-detect based on structure.

**Exit criteria:** CLI builds, all commands work, `--help` is clear.

---

## Phase 3: Test Data

Real-world accessibility snapshots for testing.

### 3.1 — Capture snapshots

Use pinchtab to capture real snapshots from common page types:

```
pinchtab --headless
curl localhost:9999/navigate -d '{"url":"https://github.com/pinchtab/pinchtab"}'
curl localhost:9999/snapshot > testdata/snapshots/github-repo.json
```

Target pages (diverse UI patterns):
- **github-repo.json** — complex nav, tabs, buttons, links
- **google-search.json** — search box, results, filters
- **wikipedia-article.json** — content-heavy, table of contents
- **ecommerce-product.json** — cart, price, quantity, reviews
- **login-page.json** — form inputs, submit, forgot password
- **dashboard.json** — sidebar nav, cards, charts, settings

### 3.2 — Query fixtures

JSON files with expected results:
```json
[
  {
    "query": "sign in button",
    "snapshot": "login-page.json",
    "expect": {
      "best_ref_role": "button",
      "min_score": 0.6,
      "confidence": ["high", "medium"]
    }
  }
]
```

**Exit criteria:** 6+ real snapshots, 30+ query fixtures across basic/synonym/adversarial.

---

## Phase 4: E2E Tests (Docker)

Containerized E2E tests that exercise the CLI binary end-to-end.

### 4.1 — Docker setup

```
tests/e2e/
├── docker-compose.yml      # Single service: runner
├── Dockerfile.runner       # Go binary + test data + bash runner
├── run.sh                  # Orchestrator (runs all cases, reports results)
└── cases/
    ├── find-basic.sh       # Basic find queries against each snapshot
    ├── find-synonyms.sh    # Synonym resolution (sign in ↔ log in, etc.)
    ├── find-threshold.sh   # Threshold edge cases
    ├── find-topk.sh        # TopK limiting
    ├── find-formats.sh     # JSON / table / refs output formats
    ├── find-stdin.sh       # Pipe snapshot via stdin
    ├── classify.sh         # Error classification
    └── match-single.sh     # Single element scoring
```

**Dockerfile.runner:**
```dockerfile
FROM golang:1.26-alpine
WORKDIR /app
COPY . .
RUN go build -o /usr/local/bin/semantic ./cmd/semantic
COPY testdata/ /testdata/
COPY tests/e2e/ /e2e/
ENTRYPOINT ["/bin/sh", "/e2e/run.sh"]
```

**docker-compose.yml:**
```yaml
services:
  runner:
    build:
      context: ../..
      dockerfile: tests/e2e/Dockerfile.runner
    volumes:
      - ../../tests/e2e/results:/results
```

### 4.2 — E2E test pattern

Each case file is a bash script that:
1. Runs `semantic` commands against test data
2. Validates output (exit codes, JSON fields, score ranges)
3. Reports pass/fail with descriptive names

```bash
#!/bin/bash
# find-basic.sh — Basic find queries

test_find_button() {
  result=$(semantic find "submit button" --snapshot /testdata/snapshots/login-page.json --format json)
  score=$(echo "$result" | jq -r '.matches[0].score')
  ref=$(echo "$result" | jq -r '.matches[0].ref')

  assert_gte "$score" "0.6" "submit button should score >= 0.6"
  assert_not_empty "$ref" "should return a ref"
}
```

### 4.3 — Results reporting

Same pattern as pinchtab:
- `tests/e2e/results/summary-*.txt` — machine-readable (`passed=N`, `failed=N`)
- `tests/e2e/results/report-*.md` — human-readable failure details
- Exit code 0 if all pass, 1 if any fail

**Exit criteria:** E2E suite runs in Docker, exercises all CLI commands, 50+ test cases.

---

## Phase 5: Dev Tool + CI

### 5.1 — `./dev` script

```bash
./dev                    # Show available commands
./dev test               # Run unit tests
./dev lint               # Run linter
./dev check              # fmt + vet + lint + test (pre-commit)
./dev build              # Build CLI binary
./dev e2e                # Run E2E suite (Docker)
./dev e2e fast           # Run E2E subset (no Docker rebuild)
./dev benchmark          # Run benchmark study
./dev snapshot <url>     # Capture a snapshot (requires pinchtab running)
```

### 5.2 — CI Workflows

Following pinchtab naming convention:

**`ci-go.yml`** — PR + push to main:
- Format check (gofmt)
- Vet
- Build (library + CLI)
- Unit tests with coverage
- Lint (golangci-lint)

**`ci-e2e.yml`** — PR + manual dispatch:
- Build Docker image
- Run full E2E suite
- Upload results as artifacts

**`reusable-go.yml`** — Building block for CI + future release workflow.

**`reusable-e2e.yml`** — Building block for CI + future release workflow.

### 5.3 — Pre-commit hook

`scripts/check.sh` — runs fmt + vet + lint + test. Fast enough for pre-commit.

**Exit criteria:** `./dev` works, CI green on first PR, pre-commit hook catches issues.

---

## Phase 6: Integration (Pinchtab)

Wire pinchtab to use the external library.

### 6.1 — Add dependency

```
cd ~/dev/pinchtab
go get github.com/pinchtab/semantic@latest
```

### 6.2 — Update imports

The import path changes, but every call site stays identical — the package
name is still `semantic`:

```go
// Before
import "github.com/pinchtab/pinchtab/internal/semantic"

// After
import "github.com/pinchtab/semantic"
```

Files to update:
- `internal/handlers/handlers.go`
- `internal/handlers/actions.go`
- `internal/handlers/find.go`
- `internal/handlers/find_test.go`

### 6.3 — Delete internal package

Remove `internal/semantic/` from pinchtab. All 154 tests now live in the semantic repo.

### 6.4 — Verify

- `go build ./...`
- `go test ./...`
- Full E2E suite
- The handlers produce identical results

**Exit criteria:** Pinchtab builds + all tests pass with external dependency. `internal/semantic/` deleted.

---

## Library Usage

This is how pinchtab uses the semantic package — and how any other project would.

### Basic: Find elements by natural language query

```go
import "github.com/pinchtab/semantic"

// Build descriptors from your accessibility tree (or any element list).
elements := []semantic.ElementDescriptor{
    {Ref: "e0", Role: "button", Name: "Sign In"},
    {Ref: "e1", Role: "textbox", Name: "Email", Value: ""},
    {Ref: "e2", Role: "link", Name: "Forgot password?"},
    {Ref: "e3", Role: "button", Name: "Create Account"},
}

// Create a matcher. CombinedMatcher fuses lexical + embedding for best results.
matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

// Find matching elements.
result, err := matcher.Find(ctx, "login button", elements, semantic.FindOptions{
    Threshold: 0.3, // minimum similarity score
    TopK:      3,    // return up to 3 matches
})

// result.BestRef = "e0"       (the Sign In button)
// result.BestScore = 0.82     (high confidence — "login" ↔ "sign in" via synonyms)
// result.Matches = [{Ref:"e0", Score:0.82}, {Ref:"e3", Score:0.45}, ...]
```

### Choose your matcher strategy

```go
// Lexical only — fast, exact + synonym matching, zero allocations per query.
lexical := semantic.NewLexicalMatcher()

// Embedding only — sub-word similarity via feature hashing.
embedding := semantic.NewEmbeddingMatcher(semantic.NewHashingEmbedder(128))

// Combined (recommended) — weighted fusion: 60% lexical + 40% embedding.
combined := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

// All implement the same interface.
var m semantic.ElementMatcher = combined
```

### Self-healing recovery

When an element reference goes stale (DOM changed, SPA re-rendered), the
recovery engine re-matches against a fresh snapshot using cached intent:

```go
// Set up the recovery engine with callbacks into your system.
intentCache := semantic.NewIntentCache(200, 10*time.Minute)

recovery := semantic.NewRecoveryEngine(
    semantic.DefaultRecoveryConfig(),
    matcher,
    intentCache,
    // SnapshotRefresher — your function to refresh the element tree.
    func(ctx context.Context, tabID string) error {
        return refreshAccessibilityTree(tabID)
    },
    // NodeIDResolver — maps a ref string to a backend node ID.
    func(tabID, ref string) (int64, bool) {
        return lookupNodeID(tabID, ref)
    },
    // DescriptorBuilder — builds element descriptors from current state.
    func(tabID string) []semantic.ElementDescriptor {
        return buildDescriptorsFromSnapshot(tabID)
    },
)

// Before executing an action, cache the element's intent.
recovery.RecordIntent(tabID, "e5", semantic.IntentEntry{
    Query:      "submit order button",
    Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Place Order"},
    Score:      0.91,
    Confidence: "high",
})

// When an action fails, check if recovery is possible.
err := executeAction(tabID, "e5", "click")
if err != nil && recovery.ShouldAttempt(err, "e5") {
    rr, result, err := recovery.Attempt(ctx, tabID, "e5", "click", executeAction)
    // rr.Recovered = true
    // rr.NewRef = "e12"        (element moved, new ref)
    // rr.Score = 0.87
    // rr.Confidence = "high"
}
```

### Classify failure types

```go
err := errors.New("could not find node with given id")
ft := semantic.ClassifyFailure(err)
// ft = semantic.FailureElementNotFound
// ft.Recoverable() = true
// ft.String() = "element_not_found"

err2 := errors.New("connection refused")
ft2 := semantic.ClassifyFailure(err2)
// ft2 = semantic.FailureNetwork
// ft2.Recoverable() = false
```

### Confidence calibration

```go
semantic.CalibrateConfidence(0.92) // "high"   (>= 0.8)
semantic.CalibrateConfidence(0.65) // "medium" (>= 0.6)
semantic.CalibrateConfidence(0.35) // "low"    (< 0.6)
```

### How pinchtab wires it up

This is the actual pattern from `pinchtab/internal/handlers/handlers.go`:

```go
func New(bridge BridgeAPI, cfg *Config) *Handlers {
    matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
    intentCache := semantic.NewIntentCache(200, 10*time.Minute)

    h := &Handlers{
        Bridge:      bridge,
        Matcher:     matcher,
        IntentCache: intentCache,
    }

    h.Recovery = semantic.NewRecoveryEngine(
        semantic.DefaultRecoveryConfig(),
        matcher,
        intentCache,
        func(ctx context.Context, tabID string) error {
            h.refreshSnapshot(ctx, tabID)
            return nil
        },
        func(tabID, ref string) (int64, bool) {
            cache := h.Bridge.GetRefCache(tabID)
            if cache == nil {
                return 0, false
            }
            return cache.Refs[ref], cache.Refs[ref] != 0
        },
        func(tabID string) []semantic.ElementDescriptor {
            nodes := h.getSnapshotNodes(tabID)
            descs := make([]semantic.ElementDescriptor, len(nodes))
            for i, n := range nodes {
                descs[i] = semantic.ElementDescriptor{
                    Ref: n.Ref, Role: n.Role,
                    Name: n.Name, Value: n.Value,
                }
            }
            return descs
        },
    )

    return h
}
```

The pattern: create matcher + cache, wire recovery with three callbacks that
bridge into your system (refresh, resolve, build). Everything else is the
library's job.

---

## Execution Order

| Phase | What | Estimate | Depends On |
|-------|------|----------|------------|
| 1 | Foundation (move code, tests pass) | 1-2h | — |
| 2 | CLI tool | 3-4h | Phase 1 |
| 3 | Test data (snapshots + fixtures) | 2-3h | Phase 2 |
| 4 | E2E tests (Docker) | 3-4h | Phase 2, 3 |
| 5 | Dev tool + CI | 2-3h | Phase 1 |
| 6 | Pinchtab integration | 1h | Phase 1 (can start early) |

**Total: ~2-3 days.**

Phases 1 + 5 can happen first (library + CI), then 2 + 3 + 4 (CLI + tests).
Phase 6 can start as soon as Phase 1 is tagged (even v0.1.0).

---

## Library Usage Examples

### Import

```go
import "github.com/pinchtab/semantic"
```

Root package — no `pkg/` subdirectory. This is the standard Go convention for
single-purpose libraries (same pattern as `go-chi/chi`, `gorilla/mux`, `uber-go/zap`).
Import gives you `semantic.NewCombinedMatcher()` — clean, no stutter.

### Basic: Find elements matching a query

```go
// Build descriptors from your accessibility tree (or any element source)
elements := []semantic.ElementDescriptor{
    {Ref: "e0", Role: "button", Name: "Sign In"},
    {Ref: "e1", Role: "textbox", Name: "Email", Value: ""},
    {Ref: "e2", Role: "link", Name: "Forgot password?"},
    {Ref: "e3", Role: "button", Name: "Create Account"},
}

// Create a matcher (combined = lexical + embedding, best accuracy)
matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

// Find elements
result, err := matcher.Find(ctx, "log in button", elements, semantic.FindOptions{
    Threshold: 0.3,
    TopK:      3,
})

// result.BestRef = "e0" (Sign In matches "log in" via synonyms)
// result.BestScore = 0.82
// result.Matches = [{Ref: "e0", Score: 0.82}, ...]
```

### Lexical-only matching (fastest, zero allocations beyond the result)

```go
matcher := semantic.NewLexicalMatcher()
result, _ := matcher.Find(ctx, "submit", elements, semantic.FindOptions{
    Threshold: 0.4,
    TopK:      1,
})
```

### Confidence calibration

```go
conf := semantic.CalibrateConfidence(result.BestScore)
// "high" (>= 0.8), "medium" (>= 0.6), or "low"
```

### Error classification for recovery decisions

```go
ft := semantic.ClassifyFailure(err)
// ft == semantic.FailureElementNotFound
// ft.Recoverable() == true
// ft.String() == "element_not_found"
```

### Self-healing recovery (how pinchtab uses it)

The recovery engine re-locates elements when refs go stale (SPA re-renders,
DOM changes, navigation). It uses callback interfaces so it stays decoupled
from any specific browser automation library:

```go
intentCache := semantic.NewIntentCache(200, 10*time.Minute)

recovery := semantic.NewRecoveryEngine(
    semantic.DefaultRecoveryConfig(),
    matcher,
    intentCache,
    // SnapshotRefresher — your callback to get a fresh DOM snapshot
    func(ctx context.Context, tabID string) error {
        return refreshSnapshot(ctx, tabID)
    },
    // NodeIDResolver — map ref → node ID from your snapshot cache
    func(tabID, ref string) (int64, bool) {
        return cache.Resolve(tabID, ref)
    },
    // DescriptorBuilder — convert your snapshot nodes to descriptors
    func(tabID string) []semantic.ElementDescriptor {
        nodes := getSnapshotNodes(tabID)
        descs := make([]semantic.ElementDescriptor, len(nodes))
        for i, n := range nodes {
            descs[i] = semantic.ElementDescriptor{
                Ref: n.Ref, Role: n.Role, Name: n.Name, Value: n.Value,
            }
        }
        return descs
    },
)

// Before actions, cache the element's intent
recovery.RecordIntent(tabID, "e5", semantic.IntentEntry{
    Query:      "checkout button",
    Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Checkout"},
})

// When an action fails, attempt recovery
if err != nil && recovery.ShouldAttempt(err, ref) {
    rr, result, err := recovery.Attempt(ctx, tabID, ref, "click", executeAction)
    if rr.Recovered {
        // Action succeeded with new ref rr.NewRef (score: rr.Score)
    }
}
```

### Intent caching (standalone)

```go
cache := semantic.NewIntentCache(200, 10*time.Minute)
cache.Store("tab1", "e5", semantic.IntentEntry{
    Query:      "submit button",
    Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Submit"},
})
entry, ok := cache.Lookup("tab1", "e5")
```

---

## Design Decisions

### Package at root (not `pkg/semantic/`)
Import as `semantic.NewCombinedMatcher()` — clean, no stutter. The module IS the package.
This follows the Go standard for single-purpose libraries. `pkg/` is a legacy anti-pattern
that the Go community has moved away from.

### CLI as trust boundary
If `semantic find` produces correct results against real snapshots, the library is correct.
E2E tests exercise the CLI, which exercises the library. No separate integration test layer needed.

### Recovery stays in the library
`RecoveryEngine` uses callback interfaces (`SnapshotRefresher`, `NodeIDResolver`, etc.),
so it's fully decoupled from pinchtab internals. Pinchtab injects its implementations.
Any browser automation tool can use recovery by providing its own callbacks.

### No external dependencies
The library stays zero-dep (stdlib only). The CLI adds only `flag` (stdlib).
This is a feature — no supply chain risk, no version conflicts.

### Snapshot format = pinchtab's format
The CLI reads the same JSON that `/snapshot` produces. No translation layer.
