# Semantic Matching Benchmark

Corpus-based benchmarks for measuring and optimizing semantic element matching.

## Quick Start

```bash
cd tests/benchmark

# Run full corpus benchmark
./scripts/run-corpus-benchmark.sh

# Compare strategies
./scripts/run-corpus-benchmark.sh --strategy lexical
./scripts/run-corpus-benchmark.sh --strategy embedding
./scripts/run-corpus-benchmark.sh --strategy combined
```

## Metrics

| Metric | Description | Target |
|--------|-------------|--------|
| **MRR** | Mean Reciprocal Rank (1/position of first correct) | > 0.90 |
| **P@1** | Precision at rank 1 (% correct first match) | > 0.90 |
| **P@3** | Precision at rank 3 (% relevant in top 3) | > 0.50 |
| **Latency P50** | Median query time | < 50ms |
| **Latency P95** | 95th percentile query time | < 100ms |

## Corpus Structure

```
corpus/
├── github-repo/
│   ├── snapshot.json    # Real AX tree snapshot
│   └── queries.json     # Annotated queries with ground truth
├── login-form/
├── ecommerce/
├── dashboard/
└── search-results/
```

### Query Format

```json
{
  "id": "github-001",
  "query": "star this repository",
  "relevant_refs": ["e10"],
  "partially_relevant_refs": ["e8", "e9"],
  "difficulty": "easy",
  "tags": ["action", "button"]
}
```

## Current Results (combined strategy)

```
Queries:     50
MRR:         0.88
P@1:         0.87
P@3:         0.34
Latency P50: 31 ms
Latency P95: 52 ms

By Difficulty:
  easy:   34 queries, P@1 = 0.95
  medium: 14 queries, P@1 = 0.78
  hard:    2 queries, P@1 = 0.00
```

## Optimization Targets

The 6 current misses are "hard" cases requiring:
- Synonym expansion (save for later → wishlist)
- Implicit actions (clone → Code button)
- Domain knowledge (CI status → Actions tab)

## Scripts

| Script | Purpose |
|--------|---------|
| `run-corpus-benchmark.sh` | Main benchmark with MRR/P@K metrics |
| `run-benchmark.sh` | Simple pass/fail test runner |

## Adding to Corpus

1. Capture snapshot from real website:
   ```bash
   curl -X POST http://localhost:9867/snapshot \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"tab_id": "..."}' > snapshot.json
   ```

2. Create `queries.json` with annotated ground truth

3. Run benchmark to establish baseline

## CI Integration

```yaml
- name: Run Benchmark
  run: |
    cd tests/benchmark
    ./scripts/run-corpus-benchmark.sh
    # Fail if P@1 drops below threshold
    P1=$(jq -r '.metrics.p_at_1' results/corpus_*.json)
    if (( $(echo "$P1 < 0.85" | bc -l) )); then
      echo "P@1 regression: $P1 < 0.85"
      exit 1
    fi
```
