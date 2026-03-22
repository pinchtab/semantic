# Semantic

Zero-dependency Go library for semantic matching of accessibility tree elements.

## What Semantic Does

Matches natural language queries like "sign in button" against structured UI element descriptors using lexical similarity, synonym expansion, and embedding-based fuzzy matching.

Browser automation tools find UI elements via CSS selectors, XPath, or IDs — all brittle. When the DOM changes (SPA re-renders, layout shifts, framework updates), selectors break. AI agents make it worse: they describe elements in natural language but the accessibility tree has structured labels.

Semantic bridges that gap.

## Key Properties

- **Zero dependencies** — only Go standard library
- **Stateless** — every `Find()` call is independent, thread-safe by default
- **Sub-millisecond** — < 1ms for 100 elements with combined matcher
- **Self-healing** — recovery engine re-locates stale elements after DOM changes
