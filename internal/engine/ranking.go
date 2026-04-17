package engine

import (
	"math"

	"github.com/pinchtab/semantic/internal/types"
)

// rankedMatchLess defines deterministic ordering for scored matches.
func rankedMatchLess(
	aScore float64,
	aDesc types.ElementDescriptor,
	aOrder int,
	bScore float64,
	bDesc types.ElementDescriptor,
	bOrder int,
) bool {
	scoreDiff := aScore - bScore
	if math.Abs(scoreDiff) > 1e-9 {
		return scoreDiff > 0
	}

	if aDesc.Positional.Depth != bDesc.Positional.Depth {
		return aDesc.Positional.Depth > bDesc.Positional.Depth
	}

	if aDesc.Positional.SiblingIndex != bDesc.Positional.SiblingIndex {
		return aDesc.Positional.SiblingIndex < bDesc.Positional.SiblingIndex
	}

	if aOrder != bOrder {
		return aOrder < bOrder
	}

	return aDesc.Ref < bDesc.Ref
}
