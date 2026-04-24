package benchmark

import (
	"encoding/json"
	"fmt"
	"sort"
)

func RunCatalog(cfg CatalogConfig) (*CatalogResult, error) {
	root := FindBenchmarkRoot()
	ds, err := LoadDataset(root)
	if err != nil {
		return nil, err
	}

	result := &CatalogResult{
		ByTag:        make(map[string]int),
		ByDifficulty: make(map[string]int),
	}

	for _, c := range ds.Corpora {
		tags := make(map[string]bool)
		for _, q := range c.Queries {
			result.TotalQueries++
			result.ByDifficulty[q.Difficulty]++
			for _, t := range q.Tags {
				tags[t] = true
				result.ByTag[t]++
			}
		}
		var tagList []string
		for t := range tags {
			tagList = append(tagList, t)
		}
		sort.Strings(tagList)
		result.Corpora = append(result.Corpora, CorpusSummary{
			ID:      c.ID,
			Queries: len(c.Queries),
			Tags:    tagList,
		})
	}

	return result, nil
}

func PrintCatalogResult(result *CatalogResult, cfg CatalogConfig) {
	if cfg.Format == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n  Corpora: %d\n", len(result.Corpora))
	fmt.Printf("  Total Queries: %d\n\n", result.TotalQueries)

	fmt.Printf("  %-30s %8s\n", "Corpus", "Queries")
	fmt.Printf("  %-30s %8s\n", "------", "-------")
	for _, c := range result.Corpora {
		fmt.Printf("  %-30s %8d\n", c.ID, c.Queries)
	}

	switch cfg.By {
	case "difficulty":
		fmt.Printf("\n  By Difficulty:\n")
		for d, n := range result.ByDifficulty {
			fmt.Printf("    %-10s %4d\n", d, n)
		}
	case "tag":
		fmt.Printf("\n  By Tag:\n")
		for t, n := range result.ByTag {
			fmt.Printf("    %-20s %4d\n", t, n)
		}
	}
	fmt.Printf("\n")
}
