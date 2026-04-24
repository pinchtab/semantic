package benchmark

import "fmt"

func RunLint(cfg LintConfig) (*LintResult, error) {
	root := FindBenchmarkRoot()
	result := &LintResult{}

	ds, err := LoadDataset(root)
	if err != nil {
		result.Errors++
		result.Messages = append(result.Messages, fmt.Sprintf("ERROR: failed to load dataset: %v", err))
		return result, nil
	}

	ids := make(map[string]string)
	for _, c := range ds.Corpora {
		for _, q := range c.Queries {
			if existing, ok := ids[q.ID]; ok {
				result.Errors++
				result.Messages = append(result.Messages,
					fmt.Sprintf("ERROR: duplicate ID '%s' in %s (first seen in %s)", q.ID, c.ID, existing))
			} else {
				ids[q.ID] = c.ID
			}
		}
	}

	for _, c := range ds.Corpora {
		refs := make(map[string]bool)
		for _, d := range c.Snapshot {
			refs[d.Ref] = true
		}
		for _, q := range c.Queries {
			for _, r := range q.RelevantRefs {
				if !refs[r] {
					result.Errors++
					result.Messages = append(result.Messages,
						fmt.Sprintf("ERROR: [%s] relevant_ref '%s' not found in snapshot", q.ID, r))
				}
			}
		}
	}

	validDiff := map[string]bool{"easy": true, "medium": true, "hard": true}
	for _, c := range ds.Corpora {
		for _, q := range c.Queries {
			if q.Difficulty != "" && !validDiff[q.Difficulty] {
				result.Errors++
				result.Messages = append(result.Messages,
					fmt.Sprintf("ERROR: invalid difficulty '%s' for query '%s'", q.Difficulty, q.ID))
			}
		}
	}

	if result.Errors == 0 && result.Warnings == 0 {
		result.Messages = append(result.Messages, "All checks passed")
	}

	return result, nil
}

func PrintLintResult(result *LintResult, cfg LintConfig) {
	for _, msg := range result.Messages {
		fmt.Println(msg)
	}
	fmt.Printf("\nErrors: %d, Warnings: %d\n", result.Errors, result.Warnings)
}
