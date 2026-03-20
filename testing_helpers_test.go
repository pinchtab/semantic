package semantic

import "math"

// dummyEmbedder generates deterministic fixed-dimension vectors using a
// simple hash of each input string. Useful for testing without real ML
// dependencies.
type dummyEmbedder struct {
	dim int
}

func newDummyEmbedder(dim int) *dummyEmbedder {
	if dim <= 0 {
		dim = 64
	}
	return &dummyEmbedder{dim: dim}
}

func (d *dummyEmbedder) Strategy() string { return "test" }

func (d *dummyEmbedder) Embed(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = d.hashVec(text)
	}
	return result, nil
}

func (d *dummyEmbedder) hashVec(s string) []float32 {
	vec := make([]float32, d.dim)
	for i, c := range s {
		idx := (i*31 + int(c)) % d.dim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += float32(c) / 128.0
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		invNorm := float32(1.0 / math.Sqrt(norm))
		for j := range vec {
			vec[j] *= invNorm
		}
	}
	return vec
}
