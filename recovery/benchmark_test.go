package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pinchtab/semantic"
)

type BenchmarkScenario struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Description   string                       `json:"description"`
	OriginalQuery string                       `json:"original_query"`
	OriginalRef   string                       `json:"original_ref"`
	Before        []semantic.ElementDescriptor `json:"before"`
	After         []semantic.ElementDescriptor `json:"after"`
	ExpectedRef   *string                      `json:"expected_ref"`
	ExpectedAlt   []string                     `json:"expected_alt"`
	ExpectNoMatch bool                         `json:"expect_no_match"`
	Difficulty    string                       `json:"difficulty"`
}

func loadScenarios(t *testing.T) []BenchmarkScenario {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..")
	scenariosPath := filepath.Join(repoRoot, "tests", "benchmark", "corpus", "recovery-scenarios", "scenarios.json")

	data, err := os.ReadFile(scenariosPath)
	if err != nil {
		t.Fatalf("failed to read scenarios: %v", err)
	}

	var scenarios []BenchmarkScenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		t.Fatalf("failed to parse scenarios: %v", err)
	}

	return scenarios
}

func TestRecoveryBenchmark_Scenarios(t *testing.T) {
	scenarios := loadScenarios(t)
	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

	passed, failed := 0, 0

	for _, sc := range scenarios {
		t.Run(sc.ID, func(t *testing.T) {
			result := runBenchmarkScenario(t, matcher, sc)

			if result.pass {
				passed++
				t.Logf("PASS: recovered=%v got=%s expected=%s score=%.3f",
					result.recovered, result.gotRef, result.expectedRef, result.score)
			} else {
				failed++
				t.Errorf("FAIL: recovered=%v got=%s expected=%s score=%.3f error=%s",
					result.recovered, result.gotRef, result.expectedRef, result.score, result.err)
			}
		})
	}

	t.Logf("Summary: %d passed, %d failed out of %d scenarios", passed, failed, len(scenarios))
}

type scenarioResult struct {
	pass        bool
	recovered   bool
	gotRef      string
	expectedRef string
	score       float64
	confidence  string
	latencyMs   int64
	err         string
}

func runBenchmarkScenario(t *testing.T, matcher semantic.ElementMatcher, sc BenchmarkScenario) scenarioResult {
	result := scenarioResult{}

	if sc.ExpectedRef != nil {
		result.expectedRef = *sc.ExpectedRef
	}

	var origDesc semantic.ElementDescriptor
	for _, d := range sc.Before {
		if d.Ref == sc.OriginalRef {
			origDesc = d
			break
		}
	}

	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("test-tab", sc.OriginalRef, IntentEntry{
		Query:      sc.OriginalQuery,
		Descriptor: origDesc,
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	})

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			for i, d := range sc.After {
				if d.Ref == ref {
					return int64(1000 + i), true
				}
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor { return sc.After },
	)

	start := time.Now()

	err := fmt.Errorf("could not find node with id %s", sc.OriginalRef)

	if !re.ShouldAttempt(err, sc.OriginalRef) {
		result.err = "ShouldAttempt returned false"
		result.pass = sc.ExpectNoMatch
		result.latencyMs = time.Since(start).Milliseconds()
		return result
	}

	rr, _, recErr := re.AttemptWithClassification(
		context.Background(),
		"test-tab",
		sc.OriginalRef,
		"click",
		ClassifyFailure(err),
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			return map[string]any{"clicked": true}, nil
		},
	)

	result.latencyMs = time.Since(start).Milliseconds()
	result.recovered = rr.Recovered
	result.gotRef = rr.NewRef
	result.score = rr.Score
	result.confidence = rr.Confidence

	if recErr != nil {
		result.err = recErr.Error()
	}

	if sc.ExpectNoMatch {
		result.pass = !rr.Recovered
	} else if sc.ExpectedRef != nil {
		if rr.NewRef == *sc.ExpectedRef {
			result.pass = true
		} else {
			for _, alt := range sc.ExpectedAlt {
				if rr.NewRef == alt {
					result.pass = true
					break
				}
			}
		}
	}

	return result
}

func BenchmarkRecoveryEngine_Scenarios(b *testing.B) {
	scenarios := loadScenariosB(b)
	matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, sc := range scenarios {
			runBenchmarkScenarioB(b, matcher, sc)
		}
	}
}

func loadScenariosB(b *testing.B) []BenchmarkScenario {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..")
	scenariosPath := filepath.Join(repoRoot, "tests", "benchmark", "corpus", "recovery-scenarios", "scenarios.json")

	data, err := os.ReadFile(scenariosPath)
	if err != nil {
		b.Fatalf("failed to read scenarios: %v", err)
	}

	var scenarios []BenchmarkScenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		b.Fatalf("failed to parse scenarios: %v", err)
	}

	return scenarios
}

func runBenchmarkScenarioB(b *testing.B, matcher semantic.ElementMatcher, sc BenchmarkScenario) {
	var origDesc semantic.ElementDescriptor
	for _, d := range sc.Before {
		if d.Ref == sc.OriginalRef {
			origDesc = d
			break
		}
	}

	cache := NewIntentCache(100, 5*time.Minute)
	cache.Store("test-tab", sc.OriginalRef, IntentEntry{
		Query:      sc.OriginalQuery,
		Descriptor: origDesc,
		Score:      0.95,
		Confidence: "high",
		Strategy:   "combined",
	})

	re := NewRecoveryEngine(
		DefaultRecoveryConfig(),
		matcher,
		cache,
		func(_ context.Context, _ string) error { return nil },
		func(_, ref string) (int64, bool) {
			for i, d := range sc.After {
				if d.Ref == ref {
					return int64(1000 + i), true
				}
			}
			return 0, false
		},
		func(_ string) []semantic.ElementDescriptor { return sc.After },
	)

	err := fmt.Errorf("could not find node with id %s", sc.OriginalRef)

	_, _, _ = re.AttemptWithClassification(
		context.Background(),
		"test-tab",
		sc.OriginalRef,
		"click",
		ClassifyFailure(err),
		func(_ context.Context, kind string, nodeID int64) (map[string]any, error) {
			return map[string]any{"clicked": true}, nil
		},
	)
}
