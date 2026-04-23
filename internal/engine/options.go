package engine

import (
	"math"

	"github.com/pinchtab/semantic/internal/types"
)

func sanitizeFindOptions(opts types.FindOptions, elementCount int, defaultTopK int) types.FindOptions {
	if defaultTopK <= 0 {
		defaultTopK = 3
	}

	if opts.TopK <= 0 {
		opts.TopK = defaultTopK
	}
	if elementCount >= 0 && opts.TopK > elementCount {
		opts.TopK = elementCount
	}

	opts.Threshold = sanitizeThreshold(opts.Threshold)
	return opts
}

func sanitizeThreshold(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return 0
	}
	if threshold < 0 {
		return 0
	}
	if threshold > 1 {
		return 1
	}
	return threshold
}
