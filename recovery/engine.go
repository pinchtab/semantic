package recovery

import (
	"context"
	"fmt"
	"time"

	"github.com/pinchtab/semantic"
)

type RecoveryConfig struct {
	// Enabled globally enables/disables recovery. Default true.
	Enabled bool

	// MaxRetries is the maximum number of recovery re-match attempts
	// before giving up. Default 1.
	MaxRetries int

	// MinConfidence is the minimum score the semantic re-match must
	// achieve for the recovery attempt to proceed. Default 0.4.
	MinConfidence float64

	// PreferHighConfidence when true will only auto-recover if the
	// confidence label is "high" or "medium". Default false (will
	// attempt recovery even at "low" confidence).
	PreferHighConfidence bool
}

func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		Enabled:              true,
		MaxRetries:           1,
		MinConfidence:        0.4,
		PreferHighConfidence: false,
	}
}

type RecoveryResult struct {
	// Recovered is true if the action succeeded after re-matching.
	Recovered bool `json:"recovered"`

	// OriginalRef is the ref the agent originally requested.
	OriginalRef string `json:"original_ref"`

	// NewRef is the ref that the semantic re-match found.
	NewRef string `json:"new_ref,omitempty"`

	// Score is the semantic similarity score of the new match.
	Score float64 `json:"score,omitempty"`

	// Confidence is "high", "medium", or "low".
	Confidence string `json:"confidence,omitempty"`

	// Strategy is the matcher strategy that produced the new ref.
	Strategy string `json:"strategy,omitempty"`

	// FailureType classifies the original error.
	FailureType string `json:"failure_type"`

	// Attempts is how many re-match attempts were made.
	Attempts int `json:"attempts"`

	// LatencyMs is the total wall-clock time spent on recovery.
	LatencyMs int64 `json:"latency_ms"`

	// Error is non-empty when recovery was attempted but failed.
	Error string `json:"error,omitempty"`
}

type SnapshotRefresher func(ctx context.Context, tabID string) error

type NodeIDResolver func(tabID, ref string) (int64, bool)

type ActionExecutor func(ctx context.Context, kind string, nodeID int64) (map[string]any, error)

type DescriptorBuilder func(tabID string) []semantic.ElementDescriptor

// RecoveryEngine orchestrates self-healing when an action fails because
// the target element's ref is stale or the DOM has changed.
//
// Integration pattern (used in handlers/actions.go):
//
//	result, err := bridge.ExecuteAction(...)
//	if err != nil && recovery.ShouldAttempt(err, ref) {
//	    rr := recovery.Attempt(ctx, tabID, ref, kind, ...)
//	    ... use rr ...
//	}
type RecoveryEngine struct {
	Config      RecoveryConfig
	Matcher     semantic.ElementMatcher
	IntentCache *IntentCache
	Refresh     SnapshotRefresher
	ResolveNode NodeIDResolver
	BuildDescs  DescriptorBuilder
}

func NewRecoveryEngine(
	cfg RecoveryConfig,
	matcher semantic.ElementMatcher,
	cache *IntentCache,
	refresh SnapshotRefresher,
	resolve NodeIDResolver,
	buildDescs DescriptorBuilder,
) *RecoveryEngine {
	return &RecoveryEngine{
		Config:      cfg,
		Matcher:     matcher,
		IntentCache: cache,
		Refresh:     refresh,
		ResolveNode: resolve,
		BuildDescs:  buildDescs,
	}
}

func (re *RecoveryEngine) ShouldAttempt(err error, ref string) bool {
	if !re.Config.Enabled || ref == "" {
		return false
	}
	ft := ClassifyFailure(err)
	return ft.Recoverable()
}

// Attempt re-locates a stale element and re-executes the action.
func (re *RecoveryEngine) Attempt(
	ctx context.Context,
	tabID string,
	ref string,
	kind string,
	exec ActionExecutor,
) (RecoveryResult, map[string]any, error) {
	ft := ClassifyFailure(fmt.Errorf("recovery trigger"))
	return re.attemptRecovery(ctx, tabID, ref, kind, ft, exec)
}

// AttemptWithClassification is like Attempt but takes a pre-classified FailureType.
func (re *RecoveryEngine) AttemptWithClassification(
	ctx context.Context,
	tabID string,
	ref string,
	kind string,
	ft FailureType,
	exec ActionExecutor,
) (RecoveryResult, map[string]any, error) {
	return re.attemptRecovery(ctx, tabID, ref, kind, ft, exec)
}

// attemptRecovery is the shared retry loop for Attempt and AttemptWithClassification.
func (re *RecoveryEngine) attemptRecovery(
	ctx context.Context,
	tabID string,
	ref string,
	kind string,
	ft FailureType,
	exec ActionExecutor,
) (RecoveryResult, map[string]any, error) {
	start := time.Now()

	rr := RecoveryResult{
		OriginalRef: ref,
		FailureType: ft.String(),
	}

	query := re.reconstructQuery(tabID, ref)
	if query == "" {
		rr.Error = "no cached intent for ref " + ref
		rr.LatencyMs = time.Since(start).Milliseconds()
		return rr, nil, fmt.Errorf("recovery: %s", rr.Error)
	}

	maxRetries := re.Config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rr.Attempts = attempt

		if re.Refresh != nil {
			if err := re.Refresh(ctx, tabID); err != nil {
				lastErr = fmt.Errorf("refresh snapshot: %w", err)
				continue
			}
		}

		descs := re.BuildDescs(tabID)
		if len(descs) == 0 {
			lastErr = fmt.Errorf("empty snapshot after refresh")
			continue
		}

		result, err := re.Matcher.Find(ctx, query, descs, semantic.FindOptions{
			Threshold: re.Config.MinConfidence,
			TopK:      1,
		})
		if err != nil {
			lastErr = fmt.Errorf("matcher: %w", err)
			continue
		}
		if result.BestRef == "" || result.BestScore < re.Config.MinConfidence {
			lastErr = fmt.Errorf("no match above threshold %.2f (best: %.2f)",
				re.Config.MinConfidence, result.BestScore)
			continue
		}

		conf := semantic.CalibrateConfidence(result.BestScore)
		if re.Config.PreferHighConfidence && conf == "low" {
			lastErr = fmt.Errorf("match confidence too low: %s (%.2f)",
				conf, result.BestScore)
			continue
		}

		rr.NewRef = result.BestRef
		rr.Score = result.BestScore
		rr.Confidence = conf
		rr.Strategy = result.Strategy

		nodeID, ok := re.ResolveNode(tabID, result.BestRef)
		if !ok {
			lastErr = fmt.Errorf("new ref %s not in cache after refresh", result.BestRef)
			continue
		}

		actionResult, execErr := exec(ctx, kind, nodeID)
		rr.LatencyMs = time.Since(start).Milliseconds()
		if execErr != nil {
			lastErr = execErr
			continue
		}

		rr.Recovered = true
		return rr, actionResult, nil
	}

	rr.LatencyMs = time.Since(start).Milliseconds()
	if lastErr != nil {
		rr.Error = lastErr.Error()
	}
	return rr, nil, lastErr
}

func (re *RecoveryEngine) reconstructQuery(tabID, ref string) string {
	if re.IntentCache == nil {
		return ""
	}
	entry, ok := re.IntentCache.Lookup(tabID, ref)
	if !ok {
		return ""
	}
	if entry.Query != "" {
		return entry.Query
	}
	return entry.Descriptor.Composite()
}

func (re *RecoveryEngine) RecordIntent(tabID, ref string, entry IntentEntry) {
	if re.IntentCache != nil {
		re.IntentCache.Store(tabID, ref, entry)
	}
}
