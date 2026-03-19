package semantic

import (
	"context"
	"fmt"
	"time"
)

// RecoveryConfig tunes the self-healing behaviour.
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

// DefaultRecoveryConfig returns a production-ready configuration.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		Enabled:              true,
		MaxRetries:           1,
		MinConfidence:        0.4,
		PreferHighConfidence: false,
	}
}

// RecoveryResult captures what the recovery engine did. It is embedded
// into the action response when recovery was attempted.
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

// SnapshotRefresher is a callback the handler provides so the recovery
// engine can force a fresh CDP accessibility tree fetch without importing
// the bridge or chromedp packages.
type SnapshotRefresher func(ctx context.Context, tabID string) error

// NodeIDResolver maps a ref string to a node ID from the current
// snapshot cache. Returns (nodeID, true) or (0, false).
type NodeIDResolver func(tabID, ref string) (int64, bool)

// ActionExecutor runs a single action and returns the result or error.
// This is the same signature as Bridge.ExecuteAction.
type ActionExecutor func(ctx context.Context, kind string, nodeID int64) (map[string]any, error)

// DescriptorBuilder converts raw snapshot data into ElementDescriptors.
// The handler provides this so the recovery engine stays decoupled from
// bridge internals.
type DescriptorBuilder func(tabID string) []ElementDescriptor

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
	Matcher     ElementMatcher
	IntentCache *IntentCache
	Refresh     SnapshotRefresher
	ResolveNode NodeIDResolver
	BuildDescs  DescriptorBuilder
}

// NewRecoveryEngine creates a RecoveryEngine with the given dependencies.
func NewRecoveryEngine(
	cfg RecoveryConfig,
	matcher ElementMatcher,
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

// ShouldAttempt returns true when recovery is enabled and the failure
// type is recoverable.
func (re *RecoveryEngine) ShouldAttempt(err error, ref string) bool {
	if !re.Config.Enabled || ref == "" {
		return false
	}
	ft := ClassifyFailure(err)
	return ft.Recoverable()
}

// Attempt tries to semantically re-locate a stale element and re-execute
// the action. It returns a RecoveryResult and, if successful, the action's
// result payload.
//
// Flow:
//  1. Classify the failure.
//  2. Reconstruct a search query from the IntentCache or the ref's last
//     known descriptor.
//  3. Force a fresh snapshot via the Refresh callback.
//  4. Build new descriptors and run the Matcher.
//  5. If a match exceeds MinConfidence, resolve its nodeID and call the
//     ActionExecutor.
//  6. Return the RecoveryResult.
func (re *RecoveryEngine) Attempt(
	ctx context.Context,
	tabID string,
	ref string,
	kind string,
	exec ActionExecutor,
) (RecoveryResult, map[string]any, error) {
	start := time.Now()

	ft := ClassifyFailure(fmt.Errorf("recovery trigger"))
	// We always accept the call at this point since ShouldAttempt already
	// gated entry. Re-classify for the result struct.
	rr := RecoveryResult{
		OriginalRef: ref,
		FailureType: ft.String(),
	}

	// 1. Build the search query from cached intent.
	query := re.reconstructQuery(tabID, ref)
	if query == "" {
		rr.Error = "no cached intent for ref " + ref
		rr.LatencyMs = time.Since(start).Milliseconds()
		return rr, nil, fmt.Errorf("recovery: %s", rr.Error)
	}

	// 2. Iterate up to MaxRetries.
	maxRetries := re.Config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rr.Attempts = attempt

		// 3. Force a fresh snapshot.
		if re.Refresh != nil {
			if err := re.Refresh(ctx, tabID); err != nil {
				lastErr = fmt.Errorf("refresh snapshot: %w", err)
				continue
			}
		}

		// 4. Build descriptors from the fresh snapshot.
		descs := re.BuildDescs(tabID)
		if len(descs) == 0 {
			lastErr = fmt.Errorf("empty snapshot after refresh")
			continue
		}

		// 5. Run the semantic matcher.
		result, err := re.Matcher.Find(ctx, query, descs, FindOptions{
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

		// Confidence gate.
		conf := CalibrateConfidence(result.BestScore)
		if re.Config.PreferHighConfidence && conf == "low" {
			lastErr = fmt.Errorf("match confidence too low: %s (%.2f)",
				conf, result.BestScore)
			continue
		}

		rr.NewRef = result.BestRef
		rr.Score = result.BestScore
		rr.Confidence = conf
		rr.Strategy = result.Strategy

		// 6. Resolve the new ref → nodeID.
		nodeID, ok := re.ResolveNode(tabID, result.BestRef)
		if !ok {
			lastErr = fmt.Errorf("new ref %s not in cache after refresh", result.BestRef)
			continue
		}

		// 7. Re-execute the action.
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

// AttemptWithClassification is like Attempt but accepts the pre-classified
// failure type so callers that already called ClassifyFailure don't
// re-compute it.
func (re *RecoveryEngine) AttemptWithClassification(
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

		result, err := re.Matcher.Find(ctx, query, descs, FindOptions{
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

		conf := CalibrateConfidence(result.BestScore)
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

// reconstructQuery builds the best possible search query from the
// IntentCache. It prefers an explicit query string, falls back to the
// element's composite descriptor.
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

// RecordIntent is a convenience for the handler layer to cache element
// intent after a successful /find or before an action execution.
func (re *RecoveryEngine) RecordIntent(tabID, ref string, entry IntentEntry) {
	if re.IntentCache != nil {
		re.IntentCache.Store(tabID, ref, entry)
	}
}
