# Benchmark Corpus

Ground truth data for semantic matching optimization.

## Structure

Each corpus entry is a directory containing:
- `snapshot.json` - AX tree snapshot from real website
- `queries.json` - Annotated queries with ground truth

## Query Format

```json
{
  "id": "github-001",
  "query": "star this repository",
  "relevant_refs": ["e10"],
  "partially_relevant_refs": ["e8", "e9"],
  "difficulty": "easy",
  "tags": ["action", "button"],
  "notes": "Star button in repo actions"
}
```

### Fields

| Field | Description |
|-------|-------------|
| `relevant_refs` | Correct matches (ideal top-1) |
| `partially_relevant_refs` | Acceptable but not ideal |
| `difficulty` | easy, medium, hard |
| `tags` | Query characteristics |

## Metrics

- **MRR** (Mean Reciprocal Rank): 1/rank of first relevant result
- **P@1**: Is top-1 result relevant?
- **P@3**: How many of top-3 are relevant?
- **Margin**: Score gap between relevant and irrelevant

## Expansion Groups

### Expansion 1: Complex Query Patterns (2026-04)

Added corpora for underrepresented query types:

- **implicit-domain-intent/**: GitHub-like repo page with 56 elements. Tests implicit intents like "clone this repo", "check CI status", "switch branch", "save for later". 18 queries, 8 hard.

- **form-state-controls/**: Settings page with checkboxes, radios, toggles, comboboxes. Tests stateful controls like "keep me logged in", "enable 2FA", "subscribe to newsletter". 18 queries, 8 hard.

- **ambiguous-layout-context/**: Multi-section page with duplicate labels (3x Search, 2x Save, 2x Cancel, 2x Login, 2x Home, 2x Help). Tests positional and section disambiguation. 17 queries, 7 hard.

Also added `tests/benchmark/cases/negative-threshold.json` with 14 no-match and threshold calibration cases.

### Expansion 2: Enterprise UI Patterns (2026-04)

Added corpora for complex enterprise UI scenarios:

- **table-grid/**: Invoice table with 50+ elements. Tests row-level context, repeated buttons (Edit, Delete, More), ordinal references ("second invoice", "last row"), and bulk operations. 24 queries, 8 hard.

- **overlays-menus-dialogs/**: Multi-layer UI with modal dialogs, dropdown menus, context menus, notifications. Tests duplicate controls across scopes ("cancel in modal", "save on page not dialog"), menu item selection, and overlay disambiguation. 24 queries, 8 hard.

- **icon-aria-labels/**: Icon-only controls across toolbar, media player, navigation. Tests sparse accessible names, icon descriptions ("kebab menu", "hamburger", "pencil edit"), and section context for repeated icons. 25 queries, 6 hard.

## Sources

Snapshots should be captured from real websites using pinchtab:
```bash
curl -X POST http://localhost:9867/snapshot \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"tab_id": "..."}' > snapshot.json
```
