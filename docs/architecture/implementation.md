# Implementation Details

Low-level details of how each component works. Useful for contributors and consumers who want to understand scoring internals.

## ElementDescriptor

The input unit. Each element from the accessibility tree becomes a descriptor:

```go
type ElementDescriptor struct {
    Ref         string
    Role        string
    Name        string
    Value       string
    Label       string
    Placeholder string
    Alt         string
    Title       string
    TestID      string
    Text        string
    Tag         string
}
```

The `Composite()` method produces a single searchable string such as `"button: Sign In"`. Natural-language matchers score against this composite; structured locators score explicit fields directly.

Structured locator queries are parsed before generic semantic matching:

- `role:<role> [name]`
- `text:<text>`
- `label:<label>`
- `placeholder:<text>`
- `alt:<text>`
- `title:<text>`
- `testid:<id>`
- `first:<selector>`, `last:<selector>`, `nth:<n>:<selector>`

`nth:<n>` is 1-based: `nth:1` selects the first ordered candidate, `nth:2` selects the second, and `nth:0` is not the first match.

Exact normalized field matches outrank substring matches. Role locators prefer `Role` and fall back to the implicit role inferred from `Tag` when the explicit role is empty or generic. `find:<query>` and `semantic:<query>` force natural-language matching.

## Lexical Matcher

The workhorse. Scores elements using extended Jaccard similarity.

### Tokenization

Both query and element composite are lowercased and split on non-alphanumeric boundaries.

`"Sign In"` → `["sign", "in"]`

### Stopword Removal

Common English words ("the", "a", "is") are removed to reduce noise. Context-aware: "in" is normally a stopword, but in "sign in" it's preserved because it appears in a known synonym phrase.

### Synonym Expansion

54 synonym groups cover common UI vocabulary. Expansion is conservative: only one synonym per token is applied, and only if it actually appears in the target description.

### Prefix Matching

Handles abbreviations. "btn" matches "button" (score contribution: 0.20). "nav" matches "navigation".

### Role Boosting

If the query mentions a role keyword ("button", "link", "textbox") and the element has that role, the score gets a boost (up to +0.25). This prevents "sign in button" from matching a "Sign In" link over a "Sign In" button.

### Scoring Formula

```
base = jaccard(queryTokens, descTokens)
     + synonymWeight × synonymScore                 // up to 0.30
     + prefixWeight × prefixScore                   // up to 0.20
     + roleBoost × min(roleMatches, cap)            // up to 0.25

final = clamp(base, 0, 1)
```

## Embedding Matcher

### HashingEmbedder

Converts text into fixed-dimension vectors (default: 128 dimensions) by hashing features into a compact vector space.

Features per element:
1. **Word-level** (`w:sign`, `w:in`) — captures exact word overlap
2. **Character n-grams** 2-4 chars (`n:^si`, `n:sig`, `n:ign`, `n:gn$`) — captures sub-word similarity
3. **Role-aware** (`role:button`) — boosted weight for known UI roles
4. **Synonym injection** — known synonyms get word-level features at 0.3× weight

Vectors are L2-normalized for cosine similarity comparison. The signed hashing trick (alternating +1/-1 signs) preserves inner-product properties.

No vocabulary construction needed. No model files. Sub-millisecond per text.

## Combined Matcher

Runs lexical and embedding in parallel (goroutines), then merges:

```
score(element) = 0.6 × lexicalScore + 0.4 × embeddingScore
```

The internal threshold for each sub-matcher is halved so candidates marginal in one but strong in the other survive to fusion.

## Intent Cache

Per-tab LRU cache (default: 200 entries, 10-minute TTL). Stores the semantic identity of elements at action time:

```go
recovery.IntentEntry{
    Query:      "checkout button",
    Descriptor: semantic.ElementDescriptor{Ref: "e5", Role: "button", Name: "Checkout"},
    Score:      0.87,
    Confidence: "high",
}
```

When e5 becomes stale, the recovery engine uses this cached intent to reconstruct the search query and find the element's new ref.

## Recovery Flow

```
1. Check IntentCache for the stale ref's original query
2. Call SnapshotRefresher to get a fresh DOM
3. Run the matcher against the new snapshot
4. If match exceeds MinConfidence (default: 0.4):
   a. Call NodeIDResolver to map new ref → node ID
   b. Call ActionExecutor to re-run the original action
   c. Return RecoveryResult{Recovered: true, NewRef: "e12"}
5. If no match or below threshold:
   Return failure with diagnostics
```

## Performance

| Operation | Typical Latency | Scale |
|-----------|----------------|-------|
| Lexical Find | < 0.5ms | 100 elements |
| Combined Find | < 1ms | 100 elements |
| Hashing Embed | < 0.1ms | per text |
| Recovery Attempt | 5-50ms | depends on refresh |

The dominant cost in recovery is the snapshot refresh (network round-trip to browser), not the semantic matching itself.
