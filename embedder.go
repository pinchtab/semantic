package semantic

// Embedder is the interface for converting text into dense vector
// representations. Implementations range from lightweight deterministic
// hashing (DummyEmbedder) to real ML models (future LocalEmbedder,
// OpenAIEmbedder).
type Embedder interface {
	// Embed converts a batch of text strings into float32 vectors.
	// All returned vectors must have the same dimensionality.
	Embed(texts []string) ([][]float32, error)

	// Strategy returns the name of the embedding strategy (e.g. "dummy", "local", "openai").
	Strategy() string
}

// DummyEmbedder generates deterministic fixed-dimension vectors using a
// simple hash of each input string. Useful for architecture testing
// without real ML dependencies.
type DummyEmbedder struct {
	Dim int // vector dimensionality (default 64)
}

// NewDummyEmbedder creates a DummyEmbedder with the given dimensionality.
func NewDummyEmbedder(dim int) *DummyEmbedder {
	if dim <= 0 {
		dim = 64
	}
	return &DummyEmbedder{Dim: dim}
}

// Strategy returns "dummy".
func (d *DummyEmbedder) Strategy() string { return "dummy" }

// Embed generates deterministic pseudo-vectors by hashing each character
// of the input string into the vector dimensions. The resulting vectors
// have useful properties: identical strings produce identical vectors,
// and strings with shared tokens produce vectors with non-zero cosine
// similarity.
func (d *DummyEmbedder) Embed(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = d.hashVec(text)
	}
	return result, nil
}

// hashVec produces a deterministic float32 vector from a string.
func (d *DummyEmbedder) hashVec(s string) []float32 {
	vec := make([]float32, d.Dim)
	for i, c := range s {
		idx := (i*31 + int(c)) % d.Dim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += float32(c) / 128.0
	}
	// Normalize to unit length for cosine similarity.
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		invNorm := float32(1.0 / sqrt64(norm))
		for j := range vec {
			vec[j] *= invNorm
		}
	}
	return vec
}

// sqrt64 is a simple float64 square root to avoid importing math in this file.
func sqrt64(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method, 20 iterations is ample for float64 precision.
	z := x / 2
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}
