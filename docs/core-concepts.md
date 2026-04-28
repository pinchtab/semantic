# Core Concepts

## Element Descriptors

The input unit. Each element from the accessibility tree becomes a descriptor:

```go
semantic.ElementDescriptor{
    Ref:         "e4",      // element reference
    Role:        "button",  // explicit/accessibility role
    Name:        "Sign In", // accessible name
    Value:       "",        // current value (for inputs)
    Label:       "Email",   // associated label text
    Placeholder: "Search",  // placeholder text
    Alt:         "Logo",    // image alt text
    Title:       "Help",    // title attribute text
    TestID:      "submit",  // test id attribute
    Text:        "Sign In", // visible text
    Tag:         "button",  // HTML tag for implicit role fallback
}
```

`Composite()` produces a single searchable string: `"button: Sign In"`.

Structured locators are parsed before natural-language matching:

| Query | Meaning |
|-------|---------|
| `role:button Sign In` | Role plus optional accessible name |
| `text:Sign In` | Visible text |
| `label:Email` | Associated label text |
| `placeholder:Search` | Placeholder text |
| `alt:Logo` | Image alt text |
| `title:Help` | Title text |
| `testid:submit` | Test id |
| `first:role:button`, `last:role:button`, `nth:1:role:button` | Ordered candidate selection |

Use `find:<query>` or `semantic:<query>` to force natural-language matching when a query starts with a locator-like prefix.

## Matchers

All matchers implement the `ElementMatcher` interface:

```go
type ElementMatcher interface {
    Find(ctx, query, elements, opts) (FindResult, error)
    Strategy() string
}
```

| Matcher | Speed | Best For |
|---------|-------|----------|
| `NewCombinedMatcher(embedder)` | < 1ms / 100 elements | General purpose (recommended) |
| `NewLexicalMatcher()` | < 0.5ms / 100 elements | Exact + synonym matching |
| `NewEmbeddingMatcher(embedder)` | < 1ms / 100 elements | Fuzzy / partial queries |

## Embedders

Embedders convert text to dense vectors for cosine similarity matching:

```go
type Embedder interface {
    Embed(texts []string) ([][]float32, error)
    Strategy() string
}
```

The built-in `NewHashingEmbedder(dim)` uses feature hashing — zero dependencies, no model files.

## Confidence Levels

Scores are mapped to labels:

| Level | Score | Meaning |
|-------|-------|---------|
| high | ≥ 0.8 | Safe to auto-execute |
| medium | ≥ 0.6 | Probably correct |
| low | < 0.6 | Uncertain |

## Recovery

When a browser action fails on a stale element ref, the `RecoveryEngine` (in `recovery/` subpackage) can self-heal:

1. Look up the original query from the `IntentCache`
2. Refresh the DOM via a callback
3. Re-match the element in the new snapshot
4. Re-execute the original action

Recovery uses callback interfaces — it works with any browser automation framework.

## Failure Classification

Errors are classified into recoverable and non-recoverable types:

| Type | Recoverable | Examples |
|------|-------------|----------|
| `element_not_found` | ✅ | "could not find node" |
| `element_stale` | ✅ | "node is detached" |
| `element_not_interactable` | ✅ | "not visible", "disabled" |
| `navigation` | ✅ | "frame detached" |
| `network` | ❌ | "connection refused" |
| `unknown` | ❌ | Everything else |
