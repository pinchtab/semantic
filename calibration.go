package semantic

// CalibrateConfidence maps a numeric similarity score to a human-readable
// confidence label. This function is shared across all matcher strategies
// to ensure consistent labelling.
func CalibrateConfidence(score float64) string {
	switch {
	case score >= 0.8:
		return "high"
	case score >= 0.6:
		return "medium"
	default:
		return "low"
	}
}
