package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pinchtab/semantic"
)

type Query struct {
	ID                    string   `json:"id"`
	QueryText             string   `json:"query"`
	RelevantRefs          []string `json:"relevant_refs"`
	PartiallyRelevantRefs []string `json:"partially_relevant_refs"`
	Difficulty            string   `json:"difficulty"`
	Tags                  []string `json:"tags"`
	Intent                string   `json:"intent,omitempty"`
	PageType              string   `json:"page_type,omitempty"`
	Threshold             *float64 `json:"threshold,omitempty"`
	TopK                  *int     `json:"top_k,omitempty"`
	ExpectNoMatch         bool     `json:"expect_no_match,omitempty"`
	MinScore              *float64 `json:"min_score,omitempty"`
	Notes                 string   `json:"notes,omitempty"`
}

type Corpus struct {
	ID        string
	Path      string
	Snapshot  []semantic.ElementDescriptor
	Queries   []Query
}

type Dataset struct {
	Root    string
	Corpora []Corpus
}

func LoadDataset(benchmarkRoot string) (*Dataset, error) {
	corpusDir := filepath.Join(benchmarkRoot, "corpus")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return nil, err
	}

	ds := &Dataset{Root: benchmarkRoot}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		corpusPath := filepath.Join(corpusDir, entry.Name())
		snapshotPath := filepath.Join(corpusPath, "snapshot.json")
		queriesPath := filepath.Join(corpusPath, "queries.json")

		if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(queriesPath); os.IsNotExist(err) {
			continue
		}

		corpus, err := loadCorpus(entry.Name(), corpusPath)
		if err != nil {
			return nil, err
		}

		ds.Corpora = append(ds.Corpora, *corpus)
	}

	return ds, nil
}

func loadCorpus(id, path string) (*Corpus, error) {
	snapshotPath := filepath.Join(path, "snapshot.json")
	queriesPath := filepath.Join(path, "queries.json")

	snapshotData, err := os.ReadFile(snapshotPath)
	if err != nil {
		return nil, err
	}

	var snapshot []semantic.ElementDescriptor
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		return nil, err
	}

	queriesData, err := os.ReadFile(queriesPath)
	if err != nil {
		return nil, err
	}

	var queries []Query
	if err := json.Unmarshal(queriesData, &queries); err != nil {
		return nil, err
	}

	return &Corpus{
		ID:       id,
		Path:     path,
		Snapshot: snapshot,
		Queries:  queries,
	}, nil
}

func (ds *Dataset) QueryCount() int {
	count := 0
	for _, c := range ds.Corpora {
		count += len(c.Queries)
	}
	return count
}

func (ds *Dataset) CorpusCount() int {
	return len(ds.Corpora)
}
