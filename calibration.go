package semantic

// CalibrateConfidence maps a similarity score to "high", "medium", or "low".
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
