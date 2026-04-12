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

## Sources

Snapshots should be captured from real websites using pinchtab:
```bash
curl -X POST http://localhost:9867/snapshot \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"tab_id": "..."}' > snapshot.json
```
