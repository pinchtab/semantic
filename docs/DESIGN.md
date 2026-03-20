# Design Decisions

Detailed discussion of architectural trade-offs in semantic.

## Hashing vs Real Embeddings

We chose feature hashing (Weinberger et al. 2009) over sentence-transformers or OpenAI embeddings. This sacrifices some fuzzy matching quality — estimated 10-15% lower recall on paraphrase queries — but eliminates all external dependencies, model downloads, and inference latency.

For UI element matching where the vocabulary is constrained (button labels, link text, form fields), hashing captures enough signal. The `Embedder` interface is designed for extension: a future `ONNXEmbedder` backed by MiniLM could be swapped in by projects that can accept the dependency.

The `HashingEmbedder` combines three feature types:
1. **Word unigrams** (`w:sign`, `w:in`) — exact word overlap
2. **Character n-grams** (`n:^si`, `n:sig`, `n:ign`, `n:gn$`) — sub-word similarity for typos and abbreviations
3. **Role-aware features** (`role:button`) — boosted weight for known ARIA roles

Vectors are L2-normalized for cosine similarity. The signed hashing trick (alternating +1/-1 signs) preserves inner-product properties.

## Fixed Synonym Table vs Learned

The 54 synonym groups are hand-curated for UI vocabulary:

- **Authentication**: login ↔ signin ↔ log in ↔ sign in ↔ authenticate
- **Navigation**: search ↔ find ↔ lookup ↔ look up ↔ query ↔ filter
- **Actions**: submit ↔ send ↔ confirm ↔ apply ↔ save ↔ done
- **Elements**: dropdown ↔ select ↔ combobox ↔ picker
- **Commerce**: cart ↔ basket ↔ bag ↔ shopping cart

A learned approach (word2vec, GloVe) would handle novel synonyms better but requires model files and more complex tokenization. The fixed table is transparent, debuggable, and covers the common cases well. It can be extended without changing the algorithm.

Expansion is conservative: only one synonym per token is applied, and only if it actually appears in the target element description.

## Jaccard vs TF-IDF

Jaccard similarity treats all tokens equally. TF-IDF would weight rare tokens higher (e.g., "checkout" is more informative than "button"). We partially compensate with role boosting and synonym scoring.

Full TF-IDF would require maintaining document frequency statistics across snapshots, adding state that the current stateless design avoids. Each `Find()` call is independent — no corpus needed.

## Recovery Callbacks vs Direct Integration

The `RecoveryEngine` could directly call Chrome DevTools Protocol to refresh snapshots and execute actions. Instead, it uses callback interfaces:

- `SnapshotRefresher` — forces a fresh DOM snapshot
- `NodeIDResolver` — maps ref string → backend node ID
- `DescriptorBuilder` — converts snapshot nodes to `ElementDescriptor` values

This adds a small amount of wiring code in the consumer but keeps the library independent of any specific browser automation framework. It also makes the recovery engine fully testable with mock callbacks.

## Combined Matcher Weighting

The combined matcher uses 60% lexical + 40% embedding. Lexical gets higher weight because:

1. It handles exact matches perfectly (score 1.0)
2. Synonym expansion covers the most common vocabulary mismatches
3. Role boosting provides important structural signal

Embedding adds value for:
- Partial queries ("check" → "Checkout")
- Typos and abbreviations
- Novel vocabulary not in the synonym table

The internal threshold for each sub-matcher is halved (50% of the requested threshold) so candidates that are marginal in one matcher but strong in the other aren't lost before fusion.

## Confidence Calibration

Scores are mapped to labels rather than raw numbers because different consumers need different things:

- **high** (≥ 0.8): Safe to auto-execute. Used by pinchtab for direct action.
- **medium** (≥ 0.6): Probably correct. Could prompt for confirmation.
- **low** (< 0.6): Uncertain. Should present alternatives.

The thresholds were calibrated against the benchmark study (20 queries × 10 page types) to minimize false positives at "high" confidence.

## Stateless Design

Every `Find()` call is independent. No state is maintained between calls (except the optional `IntentCache` for recovery). This means:

- Thread-safe by default
- No initialization cost
- No corpus or index to build
- Predictable memory usage

The trade-off is that we can't learn from previous queries or build per-page statistics. For the target use case (real-time element matching in browser automation), statelessness is the right default.

## Future Directions

- **Learned embeddings** — optional `Embedder` implementation backed by ONNX MiniLM for projects that can accept the dependency
- **Contextual scoring** — weight elements by DOM proximity, visibility, or z-index when available
- **Multi-language synonyms** — extend the synonym table for non-English UIs
- **Adaptive thresholds** — learn per-page-type confidence thresholds from historical match success rates
- **Snapshot diff** — when refreshing for recovery, diff against the previous snapshot to narrow the search space
