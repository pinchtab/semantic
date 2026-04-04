package recovery

import (
	"context"
	"fmt"
	"math"
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
	Confidence  *ConfidenceTracker
	Search      *RecoverySearchTracker
}

const diffMatchConfidenceBonus = 0.03

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
		Confidence:  NewConfidenceTracker(200, 20),
		Search:      NewRecoverySearchTracker(),
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

	maxRetries := re.maxRetries()
	threshold := re.recoveryThreshold()
	prevDescs := re.initialSnapshot(tabID)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rr.Attempts = attempt

		var actionResult map[string]any
		prevDescs, actionResult, lastErr = re.attemptOnce(ctx, tabID, query, kind, threshold, prevDescs, exec, &rr)
		if lastErr == nil {
			rr.Recovered = true
			rr.LatencyMs = time.Since(start).Milliseconds()
			return rr, actionResult, nil
		}
	}

	rr.LatencyMs = time.Since(start).Milliseconds()
	if lastErr != nil {
		rr.Error = lastErr.Error()
	}
	return rr, nil, lastErr
}

func (re *RecoveryEngine) attemptOnce(
	ctx context.Context,
	tabID string,
	query string,
	kind string,
	threshold float64,
	prevDescs []semantic.ElementDescriptor,
	exec ActionExecutor,
	rr *RecoveryResult,
) ([]semantic.ElementDescriptor, map[string]any, error) {
	descs, diffDescs, err := re.refreshAndBuild(tabID, ctx, prevDescs)
	if err != nil {
		return prevDescs, nil, err
	}

	result, usedDiff, err := re.findWithDiffFirst(ctx, query, threshold, diffDescs, descs)
	if err != nil {
		return descs, nil, fmt.Errorf("matcher: %w", err)
	}
	if result.BestRef == "" || result.BestScore < threshold {
		re.recordConfidence(result.BestScore, false)
		return descs, nil, fmt.Errorf("no match above threshold %.2f (best: %.2f)", threshold, result.BestScore)
	}

	effectiveScore := result.BestScore
	if usedDiff {
		effectiveScore = math.Min(1.0, effectiveScore+diffMatchConfidenceBonus)
	}

	conf := semantic.CalibrateConfidence(effectiveScore)
	if re.Config.PreferHighConfidence && conf == "low" {
		re.recordConfidence(effectiveScore, false)
		return descs, nil, fmt.Errorf("match confidence too low: %s (%.2f)", conf, effectiveScore)
	}

	rr.NewRef = result.BestRef
	rr.Score = effectiveScore
	rr.Confidence = conf
	rr.Strategy = result.Strategy

	if re.ResolveNode == nil {
		re.recordConfidence(effectiveScore, false)
		return descs, nil, fmt.Errorf("node resolver not configured")
	}
	nodeID, ok := re.ResolveNode(tabID, result.BestRef)
	if !ok {
		re.recordConfidence(result.BestScore, false)
		return descs, nil, fmt.Errorf("new ref %s not in cache after refresh", result.BestRef)
	}
	if exec == nil {
		re.recordConfidence(effectiveScore, false)
		return descs, nil, fmt.Errorf("action executor not configured")
	}

	actionResult, execErr := exec(ctx, kind, nodeID)
	if execErr != nil {
		re.recordConfidence(effectiveScore, false)
		return descs, nil, execErr
	}

	re.recordConfidence(effectiveScore, true)
	return descs, actionResult, nil
}

func (re *RecoveryEngine) refreshAndBuild(
	tabID string,
	ctx context.Context,
	prevDescs []semantic.ElementDescriptor,
) ([]semantic.ElementDescriptor, []semantic.ElementDescriptor, error) {
	if re.Refresh != nil {
		if err := re.Refresh(ctx, tabID); err != nil {
			return prevDescs, nil, fmt.Errorf("refresh snapshot: %w", err)
		}
	}
	if re.BuildDescs == nil {
		return prevDescs, nil, fmt.Errorf("descriptor builder not configured")
	}

	descs := re.BuildDescs(tabID)
	if len(descs) == 0 {
		return prevDescs, nil, fmt.Errorf("empty snapshot after refresh")
	}

	diffDescs, _ := diffDescriptors(prevDescs, descs)
	return descs, diffDescs, nil
}

func (re *RecoveryEngine) findWithDiffFirst(
	ctx context.Context,
	query string,
	threshold float64,
	diffDescs []semantic.ElementDescriptor,
	descs []semantic.ElementDescriptor,
) (semantic.FindResult, bool, error) {
	findOpts := semantic.FindOptions{Threshold: threshold, TopK: 1}

	if len(diffDescs) > 0 {
		if re.Search != nil {
			re.Search.RecordDiffAttempt()
		}
		result, err := re.Matcher.Find(ctx, query, diffDescs, findOpts)
		if err != nil {
			return semantic.FindResult{}, false, err
		}
		if result.BestRef != "" && result.BestScore >= threshold {
			if re.Search != nil {
				re.Search.RecordDiffHit()
			}
			return result, true, nil
		}
	}

	if re.Search != nil {
		re.Search.RecordFullSearch()
	}
	result, err := re.Matcher.Find(ctx, query, descs, findOpts)
	return result, false, err
}

func (re *RecoveryEngine) maxRetries() int {
	maxRetries := re.Config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}
	return maxRetries
}

func (re *RecoveryEngine) recoveryThreshold() float64 {
	threshold := re.Config.MinConfidence
	if re.Confidence != nil {
		threshold = re.Confidence.OptimalThresholdWithDefault(re.Config.MinConfidence)
	}
	return threshold
}

func (re *RecoveryEngine) initialSnapshot(tabID string) []semantic.ElementDescriptor {
	if re.BuildDescs == nil {
		return nil
	}
	return re.BuildDescs(tabID)
}

func (re *RecoveryEngine) recordConfidence(score float64, succeeded bool) {
	if re.Confidence == nil {
		return
	}
	if score > 0 {
		re.Confidence.Record(score, succeeded)
	}
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

// ConfidenceStats returns adaptive-threshold diagnostics for observability.
func (re *RecoveryEngine) ConfidenceStats() ConfidenceStats {
	if re.Confidence == nil {
		return ConfidenceStats{CurrentThreshold: re.Config.MinConfidence}
	}
	return re.Confidence.Stats(re.Config.MinConfidence)
}

// SearchStats returns diff-first recovery search diagnostics.
func (re *RecoveryEngine) SearchStats() RecoverySearchStats {
	if re.Search == nil {
		return RecoverySearchStats{}
	}
	return re.Search.Stats()
}
