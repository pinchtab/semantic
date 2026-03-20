package semantic

import (
	"math"
	"testing"
)

func TestHashingEmbedder_SynonymVectorsCloser(t *testing.T) {
	embedder := NewHashingEmbedder(128)

	vecs, err := embedder.Embed([]string{"sign in", "log in", "elephant"})
	if err != nil {
		t.Fatal(err)
	}

	// Cosine similarity between "sign in" and "log in" should be higher
	// than between "sign in" and "elephant"
	simSynonym := cosineSim(vecs[0], vecs[1])
	simUnrelated := cosineSim(vecs[0], vecs[2])

	t.Logf("sim('sign in', 'log in') = %.4f", simSynonym)
	t.Logf("sim('sign in', 'elephant') = %.4f", simUnrelated)

	if simSynonym <= simUnrelated {
		t.Errorf("expected synonym embedding similarity (%.4f) > unrelated (%.4f)", simSynonym, simUnrelated)
	}
}

func TestHashingEmbedder_AbbrVectorsCloser(t *testing.T) {
	embedder := NewHashingEmbedder(128)

	vecs, err := embedder.Embed([]string{"btn", "button", "elephant"})
	if err != nil {
		t.Fatal(err)
	}

	simAbbr := cosineSim(vecs[0], vecs[1])
	simUnrelated := cosineSim(vecs[0], vecs[2])

	t.Logf("sim('btn', 'button') = %.4f", simAbbr)
	t.Logf("sim('btn', 'elephant') = %.4f", simUnrelated)

	if simAbbr <= simUnrelated {
		t.Errorf("expected abbreviation embedding similarity (%.4f) > unrelated (%.4f)", simAbbr, simUnrelated)
	}
}

// cosineSim computes cosine similarity between two float32 vectors.
func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ===========================================================================
// Score Distribution Analysis Test
// ===========================================================================

func TestHashingEmbedder_PhraseAwareSynonymInjection(t *testing.T) {
	emb := NewHashingEmbedder(256)

	// "look up bar" and "search bar" should be closer than
	// "look up bar" and "weather bar" because "look up" → "search" synonym.
	vecs, err := emb.Embed([]string{
		"textbox: Look up bar",
		"textbox: Search bar",
		"textbox: Weather bar",
	})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	lookUp, search, weather := vecs[0], vecs[1], vecs[2]

	simSyn := cosineSim(lookUp, search)
	simUnrelated := cosineSim(lookUp, weather)

	if simSyn <= simUnrelated {
		t.Errorf("phrase-aware synonym injection should make 'look up' closer to 'search' "+
			"(got syn=%.4f, unrelated=%.4f)", simSyn, simUnrelated)
	}
}
