package engine

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

// Score Distribution Analysis Test

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

// Phase 3: HashingEmbedder tests

func TestHashingEmbedder_Strategy(t *testing.T) {
	e := NewHashingEmbedder(128)
	if e.Strategy() != "hashing" {
		t.Errorf("expected strategy=hashing, got %s", e.Strategy())
	}
}

func TestHashingEmbedder_DefaultDim(t *testing.T) {
	e := NewHashingEmbedder(0)
	vecs, err := e.Embed([]string{"test"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vecs[0]) != 128 {
		t.Errorf("expected default dim=128, got %d", len(vecs[0]))
	}
}

func TestHashingEmbedder_Deterministic(t *testing.T) {
	e := NewHashingEmbedder(128)
	v1, err := e.Embed([]string{"click the submit button"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	v2, err := e.Embed([]string{"click the submit button"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	if len(v1[0]) != 128 {
		t.Errorf("expected dim=128, got %d", len(v1[0]))
	}
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("HashingEmbedder not deterministic at dim %d: %f != %f", i, v1[0][i], v2[0][i])
		}
	}
}

func TestHashingEmbedder_Normalized(t *testing.T) {
	e := NewHashingEmbedder(128)
	vecs, err := e.Embed([]string{"button submit", "textbox username"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	for i, vec := range vecs {
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1.0) > 0.01 {
			t.Errorf("vector %d not unit-norm: norm=%f", i, norm)
		}
	}
}

func TestHashingEmbedder_EmptyInput(t *testing.T) {
	e := NewHashingEmbedder(64)
	vecs, err := e.Embed([]string{""})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vecs[0]) != 64 {
		t.Errorf("expected dim=64, got %d", len(vecs[0]))
	}
	// Empty string should produce a zero vector (no features to hash).
	var sum float64
	for _, v := range vecs[0] {
		sum += float64(v) * float64(v)
	}
	if sum > 0 {
		t.Error("empty input should produce zero vector")
	}
}

func TestHashingEmbedder_SimilarTexts(t *testing.T) {
	e := NewHashingEmbedder(256) // higher dim for less collision

	vecs, err := e.Embed([]string{
		"submit button",   // 0
		"submit form",     // 1 – shares "submit"
		"download report", // 2 – unrelated
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	simSameWord := CosineSimilarity(vecs[0], vecs[1])  // share "submit"
	simUnrelated := CosineSimilarity(vecs[0], vecs[2]) // no shared words

	if simSameWord <= simUnrelated {
		t.Errorf("texts sharing 'submit' should be more similar: same=%f, unrelated=%f",
			simSameWord, simUnrelated)
	}
}

func TestHashingEmbedder_SubwordSimilarity(t *testing.T) {
	e := NewHashingEmbedder(256)

	// Character n-grams should give nonzero similarity between "button" and "btn".
	vecs, err := e.Embed([]string{
		"button", // full word
		"btn",    // abbreviation – shares "bt" bigram via n-grams
		"search", // unrelated
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	simAbbrev := CosineSimilarity(vecs[0], vecs[1])
	simUnrelated := CosineSimilarity(vecs[0], vecs[2])

	// The abbreviation similarity might be small, but should be greater
	// than an unrelated word due to shared character n-grams.
	if simAbbrev <= simUnrelated {
		t.Errorf("abbreviation should be more similar: abbrev=%f, unrelated=%f",
			simAbbrev, simUnrelated)
	}
}

func TestHashingEmbedder_RoleFeatures(t *testing.T) {
	e := NewHashingEmbedder(128)

	// "button" is a role keyword; it should get an extra role feature.
	vecs, err := e.Embed([]string{
		"button submit",
		"button cancel",
		"textbox email",
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	// Both have "button" role features — should be more similar to each other
	// than to "textbox email" which has a different role keyword.
	simSameRole := CosineSimilarity(vecs[0], vecs[1])
	simDiffRole := CosineSimilarity(vecs[0], vecs[2])

	if simSameRole <= simDiffRole {
		t.Errorf("same-role elements should be more similar: same=%f, diff=%f",
			simSameRole, simDiffRole)
	}
}

func TestHashingEmbedder_BatchConsistency(t *testing.T) {
	e := NewHashingEmbedder(128)

	texts := []string{"login button", "search box", "navigation menu"}
	batchVecs, err := e.Embed(texts)
	if err != nil {
		t.Fatalf("batch embed error: %v", err)
	}

	// Each text embedded individually should match the batch result.
	for i, text := range texts {
		singleVecs, err := e.Embed([]string{text})
		if err != nil {
			t.Fatalf("single embed error: %v", err)
		}
		for j := range singleVecs[0] {
			if singleVecs[0][j] != batchVecs[i][j] {
				t.Errorf("batch[%d] != single at dim %d: %f != %f", i, j, batchVecs[i][j], singleVecs[0][j])
				break
			}
		}
	}
}

// Phase 3: CombinedMatcher tests
