# PLAN.md — semantic v0.1.0 Release

## Contact Surface: pinchtab → semantic

Pinchtab uses exactly **14 symbols** from semantic. Zero leak outside pinchtab.

### Types (8):
| Symbol | Used in | Leaks to HTTP? |
|---|---|---|
| `ElementDescriptor` | handlers — built from CDP nodes | No |
| `ElementMatch` | `findResponse.Matches` | Serialized to JSON in `/find` — but consumers see JSON, not the Go type |
| `ElementMatcher` | `Handlers.Matcher` field | No |
| `FindOptions` | passed to `Matcher.Find()` | No |
| `IntentEntry` | passed to `Recovery.RecordIntent()` | No |
| `IntentCache` | `Handlers.IntentCache` field | No |
| `RecoveryEngine` | `Handlers.Recovery` field | No |
| `RecoveryResult` | checked after recovery attempt | No |

### Functions (6):
| Symbol | Used in |
|---|---|
| `NewCombinedMatcher(embedder)` | handler init |
| `NewHashingEmbedder(dim)` | handler init |
| `NewIntentCache(maxRefs, ttl)` | handler init |
| `NewRecoveryEngine(config, matcher, ...)` | handler init |
| `DefaultRecoveryConfig()` | handler init |
| `ClassifyFailure(err)` | error handling in actions |

### Files that change in pinchtab (3):
```
internal/handlers/handlers.go   — init wiring
internal/handlers/actions.go    — ElementDescriptor, FindOptions, ClassifyFailure, RecoveryResult, IntentEntry
internal/handlers/find.go       — ElementDescriptor, ElementMatch, FindOptions, IntentEntry
```

### bridge & mcp
Zero type imports. Only use "semantic" in string descriptions/error messages.

---

## Repo Inventory: `github.com/pinchtab/semantic`

### Package layout
```
semantic/               ← root Go package (matching engine)
  match.go              ← ElementMatcher interface, FindOptions, FindResult, ElementMatch, MatchExplain
  descriptor.go         ← ElementDescriptor, Composite()
  calibration.go        ← CalibrateConfidence()
  lexical.go            ← lexicalMatcher, lexicalScore, tokenize, synonymScore
  embedding.go          ← embeddingMatcher, cosineSimilarity
  hashing.go            ← hashingEmbedder, tokenizeForEmbedding, hashFeature
  combined.go           ← combinedMatcher, NewCombinedMatcher
  synonyms.go           ← synonym table (54 groups), expandWithSynonyms, buildPhrases
  stopwords.go          ← isStopword, removeStopwords, removeStopwordsContextAware

recovery/               ← subpackage (error recovery + intent caching)
  engine.go             ← RecoveryEngine, RecoveryConfig, RecoveryResult, callback types
  cache.go              ← IntentCache, IntentEntry
  failure.go            ← FailureType, ClassifyFailure, classification rules

cmd/semantic/           ← CLI tool
  main.go               ← find, match, classify, version commands
```

### Test files (standalone: 3772 LOC, pinchtab copy: 6502 LOC)
```
Root package tests:
  combined_test.go         440 LOC  — CombinedMatcher unit tests + complex scenarios
  descriptor_test.go        95 LOC  — Composite(), CalibrateConfidence, ConfidenceLabel
  embedding_test.go        171 LOC  — DummyEmbedder, cosine similarity, EmbeddingMatcher
  hashing_test.go          261 LOC  — HashingEmbedder (synonyms, abbreviations, batch)
  lexical_test.go          307 LOC  — LexicalScore, LexicalMatcher, real-world elements
  stopwords_test.go        197 LOC  — stopword removal, context-aware preservation
  synonyms_test.go         159 LOC  — synonym index, scores, expansion
  testing_helpers_test.go   49 LOC  — dummyEmbedder test helper

Match evaluation tests:
  match_eval_test.go       271 LOC  — ComprehensiveEvaluation, ScoreDistribution, MultiSite
  match_edge_test.go       107 LOC  — edge cases (empty, gibberish, stopwords, long, single-char)
  match_partial_test.go     80 LOC  — partial matching (btn, nav, qty)
  match_synonym_test.go    335 LOC  — synonym resolution across strategies
  benchmark_test.go        156 LOC  — performance benchmarks

Recovery tests:
  recovery/engine_test.go  ~38k LOC — scenario tests, multi-retry, classification
```

### Pinchtab-only tests (need porting):
```
benchmark_study_test.go   887 LOC  — structured accuracy study (20 queries × 10 page types)
improvements_test.go     1360 LOC  — superset of match_*_test + stopword/synonym/hashing tests
semantic_test.go          981 LOC  — core unit tests (some overlap with standalone)
```

### CI / Release (already built)
```
.github/workflows/
  ci-go.yml               — test + lint on push/PR
  ci-e2e.yml              — E2E tests on PR (Docker)
  ci-npm.yml              — npm dry-run on PR
  release-prepare.yml     — dry-run GoReleaser (manual dispatch)
  release-publish.yml     — GoReleaser on v* tag (binaries + homebrew PR)
  release-manual-publish.yml — manual full release
  reusable-*.yml          — shared workflow templates

.goreleaser.yaml          — builds linux/darwin/windows × amd64/arm64, homebrew tap PR
```

### npm wrapper (`@pinchtab/semantic`)
```
npm/
  package.json            — v0.1.0, postinstall downloads Go binary
  bin/semantic            — shell wrapper
  scripts/postinstall.js  — downloads binary from GitHub releases, SHA256 verification
```

### E2E tests
```
tests/e2e/
  Dockerfile, docker-compose.yml, run.sh, lib.sh
  cases/                  — test case definitions
  results/                — output directory
scripts/e2e.sh            — runner script
testdata/
  snapshots/              — real page snapshots
  queries/                — query fixtures
```

### Docs
```
README.md                 — usage, install (go get, npm, homebrew), API examples
docs/DESIGN.md            — architecture decisions (hashing vs embeddings, Jaccard vs TF-IDF, etc.)
docs/IMPLEMENTATION.md    — implementation details
```

### Git state
- **Remote:** `https://github.com/pinchtab/semantic.git`
- **Commits:** 22 (latest: `d868429 cleanup: remove redundant comments`)
- **No tags yet** — v0.1.0 will be the first
- **All tests pass:** `go test ./...` green, `go vet` clean

---

## Current divergence: standalone vs pinchtab

| Aspect | Standalone | Pinchtab `internal/semantic/` |
|---|---|---|
| Types | **Unexported** (`combinedMatcher`) | **Exported** (`CombinedMatcher`) |
| Constructor returns | `ElementMatcher` interface | `*CombinedMatcher` concrete pointer |
| `LexicalScore` | Unexported | Exported |
| `CosineSimilarity` | Unexported | Exported |
| Weight fields | Unexported | Exported (`LexicalWeight`, `EmbeddingWeight`) |
| `DummyEmbedder` | Test-only, unexported | Exported (used in handler tests) |
| Recovery | Clean `recovery/` subpackage | Flat, same package |
| `Attempt()` | Delegates to `attemptRecovery()` (cleaner) | Inline (115 more LOC, more comments) |
| `DescriptorBuilder` | Uses `semantic.ElementDescriptor` | Uses `ElementDescriptor` (same pkg) |
| Tests | 3772 LOC | 6502 LOC (superset) |

**Algorithms are identical.** The divergence is purely API surface and code organization. Standalone has the cleaner structure (recovery as subpackage, shared `attemptRecovery`).

---

## Steps

### Phase 1: Export API in standalone

Make the standalone match pinchtab's needs. No algorithm changes.

- [ ] 1. Export matcher types: `CombinedMatcher`, `LexicalMatcher`, `EmbeddingMatcher`, `HashingEmbedder`
- [ ] 2. Constructor return types → concrete pointers (`*CombinedMatcher`, etc.)
- [ ] 3. Export `LexicalScore`, `CosineSimilarity`
- [ ] 4. Export `CombinedMatcher.LexicalWeight`, `CombinedMatcher.EmbeddingWeight`
- [ ] 5. Add exported `DummyEmbedder` (move from test helper to `hashing.go` or new `dummy.go`)

Files to edit: `combined.go`, `lexical.go`, `embedding.go`, `hashing.go`, `testing_helpers_test.go` → `dummy.go`

### Phase 2: Sync recovery subpackage

Standalone recovery is already cleaner. Just verify field parity:

- [ ] 6. Verify `IntentEntry` fields match (they do: Query, Descriptor, Score, Confidence, Strategy, CachedAt)
- [ ] 7. Verify `RecoveryResult` fields match
- [ ] 8. Verify `ClassifyFailure` patterns match
- [ ] 9. Verify `RecoveryEngine` method signatures match what pinchtab calls

### Phase 3: Port missing tests

- [ ] 10. Diff `benchmark_study_test.go` (887 LOC) — port to standalone root package
- [ ] 11. Diff `improvements_test.go` (1360 LOC) vs existing `match_*_test.go` files — port only missing cases
- [ ] 12. Diff `semantic_test.go` (981 LOC) vs existing unit tests — port gaps
- [ ] 13. Run full suite: `go test ./... -count=1 -race`
- [ ] 14. Run `scripts/check.sh` (format, vet, lint, tests)

### Phase 4: Tag v0.1.0

- [ ] 15. Update `npm/package.json` version to `0.1.0` (already set)
- [ ] 16. Final README review — document exported API
- [ ] 17. `git tag v0.1.0 && git push origin v0.1.0`
- [ ] 18. CI `release-publish.yml` fires → GoReleaser builds binaries + opens homebrew tap PR
- [ ] 19. npm publish `@pinchtab/semantic@0.1.0` (manual or via `release-manual-publish.yml`)

### Phase 5: Integrate into pinchtab

- [ ] 20. `cd ~/dev/pinchtab && go get github.com/pinchtab/semantic@v0.1.0`
- [ ] 21. Update imports in `handlers.go`:
    ```go
    // Before
    "github.com/pinchtab/pinchtab/internal/semantic"
    
    // After
    "github.com/pinchtab/semantic"
    "github.com/pinchtab/semantic/recovery"
    ```
- [ ] 22. Update imports in `actions.go` and `find.go` (same pattern)
- [ ] 23. Remap recovery types:
    ```
    semantic.IntentCache       → recovery.IntentCache
    semantic.IntentEntry       → recovery.IntentEntry
    semantic.RecoveryEngine    → recovery.RecoveryEngine
    semantic.RecoveryResult    → recovery.RecoveryResult
    semantic.ClassifyFailure   → recovery.ClassifyFailure
    semantic.DefaultRecoveryConfig → recovery.DefaultRecoveryConfig
    semantic.NewRecoveryEngine → recovery.NewRecoveryEngine
    semantic.NewIntentCache    → recovery.NewIntentCache
    ```
    Core types stay as `semantic.X`:
    ```
    semantic.ElementDescriptor  (unchanged)
    semantic.ElementMatch       (unchanged)
    semantic.ElementMatcher     (unchanged)
    semantic.FindOptions        (unchanged)
    semantic.NewCombinedMatcher (unchanged)
    semantic.NewHashingEmbedder (unchanged)
    ```
- [ ] 24. Delete `internal/semantic/` (17 files, 6502 LOC)
- [ ] 25. Run pinchtab tests: `go test ./... -count=1 -race`
- [ ] 26. Run pinchtab CI: `scripts/check.sh`
- [ ] 27. Commit, tag pinchtab release

---

## Responsibility split

| Concern | Owner |
|---|---|
| Matching (lexical, embedding, combined) | `semantic` |
| Synonyms, stopwords, calibration | `semantic` |
| Hashing embedder | `semantic` |
| Recovery engine, failure classification | `semantic/recovery` |
| Intent cache | `semantic/recovery` |
| CLI tool (`semantic find/match/classify`) | `semantic/cmd` |
| Selector parsing (`find:`, `css:`, `ref:`) | `pinchtab` |
| CDP node → `ElementDescriptor` conversion | `pinchtab` |
| HTTP handlers (`/find`, `/action`) | `pinchtab` |
| Browser automation, tabs, snapshots | `pinchtab` |
| MCP tools | `pinchtab` |

## Open decisions

- [x] `ElementMatch` in `/find` response → keep (simple DTO, doesn't leak Go type to HTTP consumers)
- [x] Recovery as subpackage → keep (standalone already has it right)
- [ ] `DummyEmbedder` → export in root package or keep in `recovery/`? → **root package** (handlers use it)
- [ ] Homebrew tap PR auto-merge → needs `HOMEBREW_TAP_GITHUB_TOKEN` secret in repo
- [ ] npm publish → manual first release, then automate
