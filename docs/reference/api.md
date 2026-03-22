# API Reference

## Root Package (`github.com/pinchtab/semantic`)

### Types

| Type | Kind | Description |
|------|------|-------------|
| `ElementMatcher` | interface | Scores elements against a query |
| `Embedder` | interface | Converts text to dense vectors |
| `ElementDescriptor` | struct | Describes one accessibility tree node |
| `ElementMatch` | struct | A single scored match result |
| `FindOptions` | struct | Controls matching (threshold, top-k, weights) |
| `FindResult` | struct | Top matches from a Find call |
| `MatchExplain` | struct | Per-strategy score breakdown |

### Constructors

```go
// Recommended — fuses lexical + embedding (0.6/0.4 weights)
func NewCombinedMatcher(embedder Embedder) ElementMatcher

// Standalone matchers
func NewLexicalMatcher() ElementMatcher
func NewEmbeddingMatcher(e Embedder) ElementMatcher

// Built-in embedder (feature hashing, zero deps)
func NewHashingEmbedder(dim int) Embedder
```

### Functions

```go
// Score calibration
func CalibrateConfidence(score float64) string  // "high", "medium", "low"

// Low-level scoring
func LexicalScore(query, desc string) float64
func CosineSimilarity(a, b []float32) float64
```

### FindOptions Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Threshold` | `float64` | 0 | Minimum score to include |
| `TopK` | `int` | 3 | Maximum matches to return |
| `LexicalWeight` | `float64` | 0 | Per-request weight override |
| `EmbeddingWeight` | `float64` | 0 | Per-request weight override |
| `Explain` | `bool` | false | Include per-strategy breakdown |

---

## Recovery Package (`github.com/pinchtab/semantic/recovery`)

### Types

| Type | Kind | Description |
|------|------|-------------|
| `RecoveryEngine` | struct | Self-healing element re-location |
| `RecoveryConfig` | struct | Tuning (enabled, min confidence, max retries) |
| `RecoveryResult` | struct | Outcome of a recovery attempt |
| `IntentCache` | struct | Per-tab LRU cache of element intents |
| `IntentEntry` | struct | Cached identity of an element |
| `FailureType` | type | Classified error category |

### Constructors

```go
func NewRecoveryEngine(cfg, matcher, cache, refresh, resolve, buildDescs) *RecoveryEngine
func DefaultRecoveryConfig() RecoveryConfig
func NewIntentCache(maxRefs int, ttl time.Duration) *IntentCache
```

### Functions

```go
func ClassifyFailure(err error) FailureType
```

### RecoveryEngine Methods

```go
func (re *RecoveryEngine) ShouldAttempt(err error, ref string) bool
func (re *RecoveryEngine) Attempt(ctx, tabID, ref, kind, exec) (RecoveryResult, map[string]any, error)
func (re *RecoveryEngine) AttemptWithClassification(ctx, tabID, ref, kind, ft, exec) (RecoveryResult, map[string]any, error)
func (re *RecoveryEngine) RecordIntent(tabID, ref string, entry IntentEntry)
```

### Callback Types

```go
type SnapshotRefresher func(ctx context.Context, tabID string) error
type NodeIDResolver func(tabID, ref string) (int64, bool)
type ActionExecutor func(ctx context.Context, kind string, nodeID int64) (map[string]any, error)
type DescriptorBuilder func(tabID string) []semantic.ElementDescriptor
```
