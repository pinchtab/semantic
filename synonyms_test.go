package semantic

import (
	"testing"
)

func TestSynonymIndex_Bidirectional(t *testing.T) {
	// Every entry in uiSynonyms should be bidirectional.
	for canonical, synonyms := range uiSynonyms {
		for _, syn := range synonyms {
			if syns, ok := synonymIndex[syn]; !ok {
				t.Errorf("synonym %q (of %q) not in synonymIndex", syn, canonical)
			} else if !syns[canonical] {
				t.Errorf("synonymIndex[%q] does not map back to canonical %q", syn, canonical)
			}
		}
	}
}

func TestSynonymScore_SignInLogIn(t *testing.T) {
	qTokens := tokenize("sign in")
	dTokens := tokenize("Log in")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'sign in' vs 'Log in', got %f", score)
	}
}

func TestSynonymScore_RegisterCreateAccount(t *testing.T) {
	qTokens := tokenize("register")
	dTokens := tokenize("Create account")
	score := synonymScore(qTokens, dTokens)
	if score < 0.5 {
		t.Errorf("expected synonym score >= 0.5 for 'register' vs 'Create account', got %f", score)
	}
}

func TestSynonymScore_LookUpSearch(t *testing.T) {
	qTokens := tokenize("look up")
	dTokens := tokenize("Search")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'look up' vs 'Search', got %f", score)
	}
}

func TestSynonymScore_NavigationMainMenu(t *testing.T) {
	qTokens := tokenize("navigation")
	dTokens := tokenize("Main menu")
	score := synonymScore(qTokens, dTokens)
	if score < 0.3 {
		t.Errorf("expected synonym score >= 0.3 for 'navigation' vs 'Main menu', got %f", score)
	}
}

func TestSynonymScore_NoRelation(t *testing.T) {
	qTokens := tokenize("elephant")
	dTokens := tokenize("button")
	score := synonymScore(qTokens, dTokens)
	if score > 0.1 {
		t.Errorf("expected near-zero synonym score for unrelated terms, got %f", score)
	}
}

func TestExpandWithSynonyms_MultiWord(t *testing.T) {
	query := tokenize("sign in")
	desc := tokenize("log in")
	expanded := expandWithSynonyms(query, desc)
	// "sign in" is a synonym for "log in", so expansion should add "log" and "in"
	found := false
	for _, tok := range expanded {
		if tok == "log" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expanding 'sign in' against desc 'log in' should add 'log', got: %v", expanded)
	}
}

func TestBuildPhrases(t *testing.T) {
	tokens := []string{"sign", "in", "button"}
	phrases := buildPhrases(tokens, 3)
	if len(phrases) == 0 {
		t.Fatal("expected at least one phrase")
	}
	found := false
	for _, p := range phrases {
		if p.text == "sign in" {
			found = true
			break
		}
	}
	if !found {
		texts := make([]string, len(phrases))
		for i, p := range phrases {
			texts[i] = p.text
		}
		t.Errorf("expected phrase 'sign in', got: %v", texts)
	}
}

// ===========================================================================
// Context-Aware Stopword Tests
// ===========================================================================

func TestSynonymScore_NoDuplicateCounting(t *testing.T) {
	// "sign in" should match the phrase "sign in" in synonymIndex.
	// The score should NOT double-count "sign" and "in" individually
	// on top of the phrase match.
	score := synonymScore(
		[]string{"sign", "in"},
		[]string{"login", "button"},
	)
	// Phrase "sign in" → synonym "login" → present in desc → 1 match.
	// len(queryTokens) = 2, but only 1 phrase matched (both indices consumed).
	// Score = 1/2 = 0.5.
	if score > 0.55 {
		t.Errorf("synonymScore should not double-count phrase components, got %.3f", score)
	}
	if score < 0.45 {
		t.Errorf("synonymScore should recognise 'sign in' vs 'login', got %.3f", score)
	}
}

func TestExpandWithSynonyms_NoDuplicateTokens(t *testing.T) {
	expanded := expandWithSynonyms(
		[]string{"sign", "in"},
		[]string{"login", "button"},
	)
	seen := make(map[string]int)
	for _, tok := range expanded {
		seen[tok]++
	}
	for tok, cnt := range seen {
		if cnt > 1 {
			t.Errorf("token %q appears %d times in expanded set, expected at most 1", tok, cnt)
		}
	}
}

func TestSynonymIndex_LogOnBidirectional(t *testing.T) {
	// "log on" should map to login-family and vice versa.
	syns, ok := synonymIndex["log on"]
	if !ok {
		t.Fatal("synonymIndex should contain 'log on'")
	}
	if _, has := syns["login"]; !has {
		t.Error("'log on' should map to 'login'")
	}

	loginSyns, ok := synonymIndex["login"]
	if !ok {
		t.Fatal("synonymIndex should contain 'login'")
	}
	if _, has := loginSyns["log on"]; !has {
		t.Error("'login' should map back to 'log on'")
	}
}

