# PLAN.md — semantic v0.1.0 Release

## Progress

### ✅ Phase 1: Export API — DONE
- Exported all types (CombinedMatcher, LexicalMatcher, etc.)
- Constructor return types → concrete pointers
- Exported LexicalScore, CosineSimilarity, weight fields
- Commit: `ffe2648`

### ✅ Phase 2: Recovery sync — DONE  
- Verified IntentEntry, RecoveryResult, ClassifyFailure all match pinchtab
- RecoveryEngine method signatures match pinchtab's usage
- No changes needed — already aligned

### ✅ Phase 3: Port missing tests — DONE
- Only 2 truly missing: TestBenchmarkStudy, TestCombinedMatcher_WeightsApplied
- Ported both. 167 tests pass with -race
- Commit: `ae027e4`

### ✅ Phase 4: Internal packages refactor — DONE (unplanned, Luigi's request)
- Moved all implementations to `internal/engine/`
- Types in `internal/types/`
- Root `semantic.go` is thin public API (type aliases + constructor wrappers)
- Consumers cannot depend on internals
- 167 tests still pass with -race
- Commit: `1529ca2`

### ⬜ Phase 5: Tag v0.1.0
- [ ] Update README — document new package layout + public API
- [ ] `git tag v0.1.0 && git push origin v0.1.0`
- [ ] CI `release-publish.yml` fires → GoReleaser builds binaries + homebrew tap PR
- [ ] npm publish `@pinchtab/semantic@0.1.0` (npm/package.json already at 0.1.0)

### ⬜ Phase 6: Integrate into pinchtab
- [ ] `go get github.com/pinchtab/semantic@v0.1.0`
- [ ] Update imports in 3 files (handlers.go, actions.go, find.go)
- [ ] Remap recovery types: `semantic.IntentCache` → `recovery.IntentCache`, etc.
- [ ] Delete `internal/semantic/` (17 files, 6502 LOC)
- [ ] Run pinchtab tests
- [ ] Tag pinchtab release

---

## Current Structure

```
semantic.go              ← public API (8 type aliases + 8 functions)
semantic_test.go         ← API-level smoke tests

internal/
  types/types.go         ← type definitions (interfaces, structs, CalibrateConfidence)
  engine/                ← ALL matching implementations (hidden)
    combined.go            lexical.go
    embedding.go           hashing.go
    synonyms.go            stopwords.go
    + 13 test files (167 tests)

recovery/                ← public subpackage (intent cache, failure classification)
  engine.go              cache.go              failure.go
  engine_test.go

cmd/semantic/main.go     ← CLI tool (find, match, classify)
```

## Contact Surface: pinchtab → semantic (14 symbols)

```
semantic.ElementDescriptor    semantic.NewCombinedMatcher
semantic.ElementMatch         semantic.NewHashingEmbedder
semantic.ElementMatcher       recovery.NewIntentCache
semantic.FindOptions          recovery.NewRecoveryEngine
                              recovery.DefaultRecoveryConfig
recovery.IntentCache          recovery.ClassifyFailure
recovery.IntentEntry
recovery.RecoveryEngine
recovery.RecoveryResult
```

Files that change in pinchtab: `handlers.go`, `actions.go`, `find.go`
