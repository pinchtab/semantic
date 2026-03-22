package engine

// Comprehensive evaluation tests.
// Multi-site accuracy benchmarks, score distribution validation,
// and cross-page-type evaluation suites.

import (
	"github.com/pinchtab/semantic/internal/types"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestComprehensiveEvaluation(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	type evalCase struct {
		category string
		query    string
		site     string
		wantRef  string
		wantName string
	}

	cases := []evalCase{
		// Exact matches
		{"exact", "Search Wikipedia", "wikipedia", "e1", "Search Wikipedia"},
		{"exact", "Log in", "wikipedia", "e8", "Log in"},
		{"exact", "Create account", "wikipedia", "e9", "Create account"},
		{"exact", "Sign in", "github_login", "e5", "Sign in"},
		{"exact", "Google Search", "google", "e2", "Google Search"},

		// Synonyms (the main weakness)
		{"synonym", "sign in", "wikipedia", "e8", "Log in"},
		{"synonym", "register", "wikipedia", "e9", "Create account"},
		{"synonym", "look up", "wikipedia", "e1", "Search Wikipedia"},
		{"synonym", "navigation", "wikipedia", "e10", "Main menu"},
		{"synonym", "login button", "github_login", "e5", "Sign in"},
		{"synonym", "authenticate", "github_login", "e5", "Sign in"},
		{"synonym", "dismiss", "ecommerce", "e15", "Close"},
		{"synonym", "download orders", "ecommerce", "e12", "Export Orders"},

		// Paraphrases
		{"paraphrase", "reset password", "github_login", "e6", "Forgot password?"},
		{"paraphrase", "email field", "github_login", "e3", "Username or email address"},

		// Partial / abbreviations
		{"partial", "qty", "ecommerce", "e11", "Quantity"},
	}

	results := make(map[string][]bool)
	var totalPass, totalFail int

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%s", tc.category, tc.query), func(t *testing.T) {
			result, err := matcher.Find(context.Background(), tc.query, sites[tc.site], types.FindOptions{
				Threshold: 0.1,
				TopK:      5,
			})
			if err != nil {
				t.Fatal(err)
			}

			pass := false
			for i, m := range result.Matches {
				if i >= 3 {
					break
				}
				if m.Ref == tc.wantRef {
					pass = true
					break
				}
			}

			results[tc.category] = append(results[tc.category], pass)
			if pass {
				totalPass++
				t.Logf("PASS: query=%q -> %s (score=%.3f)", tc.query, tc.wantName, result.BestScore)
			} else {
				totalFail++
				t.Logf("MISS: query=%q wanted %s (%s), got BestRef=%s (score=%.3f)",
					tc.query, tc.wantRef, tc.wantName, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%.3f name=%s", m.Ref, m.Score, m.Name)
				}
			}
		})
	}

	// Summary
	t.Logf("\n=== EVALUATION SUMMARY ===")
	t.Logf("Total: %d/%d (%.1f%%)", totalPass, totalPass+totalFail, 100*float64(totalPass)/float64(totalPass+totalFail))
	for cat, res := range results {
		passed := 0
		for _, r := range res {
			if r {
				passed++
			}
		}
		t.Logf("  %s: %d/%d (%.0f%%)", cat, passed, len(res), 100*float64(passed)/float64(len(res)))
	}
}

// Hashing Embedder Synonym Feature Tests

// Hashing Embedder Synonym Feature Tests

func TestScoreDistribution_BeforeVsExpected(t *testing.T) {
	// This test documents the expected improvement in scores
	// for the queries that were failing in the real-world evaluation
	type scoreCase struct {
		query    string
		desc     string
		minScore float64
		label    string
	}

	cases := []scoreCase{
		// These had very low scores before (from the issue)
		{"sign in", "link: Log in", 0.15, "synonym: sign in -> Log in"},
		{"register", "link: Create account", 0.10, "synonym: register -> Create account"},
		{"look up", "search: Search", 0.10, "synonym: look up -> Search"},
		{"navigation", "navigation: Main menu", 0.15, "synonym: navigation -> Main menu"},
		{"login button", "button: Sign in", 0.15, "synonym: login -> Sign in"},
		{"dismiss", "button: Close", 0.10, "synonym: dismiss -> Close"},
		{"download", "button: Export", 0.10, "synonym: download -> Export"},

		// Prefix/abbreviation cases
		{"btn submit", "button: Submit", 0.30, "prefix: btn -> button"},
		{"nav", "navigation: Main navigation", 0.15, "prefix: nav -> navigation"},

		// These should still work well (exact matches)
		{"submit button", "button: Submit", 0.50, "exact: submit button"},
		{"search box", "search: Search", 0.30, "exact: search"},
		{"email input", "textbox: Email", 0.20, "exact-ish: email input"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			score := LexicalScore(tc.query, tc.desc)
			status := "PASS"
			if score < tc.minScore {
				status = "FAIL"
			}
			t.Logf("[%s] LexicalScore(%q, %q) = %.4f (min: %.2f)", status, tc.query, tc.desc, score, tc.minScore)
			if score < tc.minScore {
				t.Errorf("score %.4f below minimum %.2f", score, tc.minScore)
			}
		})
	}
}

// Stopword Edge Cases

// Stopword Edge Cases

func TestMultiSiteEvaluation(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	type testCase struct {
		category string
		query    string
		site     string
		wantRef  string
		wantName string
	}

	allCases := []testCase{
		// === EXACT MATCHES ===
		{"exact", "Search Wikipedia", "wikipedia", "e1", "Search Wikipedia"},
		{"exact", "Log in", "wikipedia", "e8", "Log in"},
		{"exact", "Create account", "wikipedia", "e9", "Create account"},
		{"exact", "Sign in", "github_login", "e5", "Sign in"},
		{"exact", "Password", "github_login", "e4", "Password"},
		{"exact", "Google Search", "google", "e2", "Google Search"},
		{"exact", "Cart", "ecommerce", "e3", "Cart"},
		{"exact", "Add to Cart", "ecommerce", "e4", "Add to Cart"},

		// === SYNONYMS ===
		{"synonym", "sign in", "wikipedia", "e8", "Log in"},
		{"synonym", "register", "wikipedia", "e9", "Create account"},
		{"synonym", "look up", "wikipedia", "e1", "Search Wikipedia"},
		{"synonym", "navigation", "wikipedia", "e10", "Main menu"},
		{"synonym", "login", "github_login", "e5", "Sign in"},
		{"synonym", "authenticate", "github_login", "e5", "Sign in"},
		{"synonym", "dismiss", "ecommerce", "e15", "Close"},
		{"synonym", "download orders", "ecommerce", "e12", "Export Orders"},
		{"synonym", "purchase", "ecommerce", "e7", "Buy Now"},

		// === PARAPHRASES ===
		{"paraphrase", "reset password", "github_login", "e6", "Forgot password?"},
		{"paraphrase", "search input", "google", "e1", "Search"},
		{"paraphrase", "email field", "github_login", "e3", "Username or email address"},
		{"paraphrase", "shopping bag", "ecommerce", "e3", "Cart"},

		// === PARTIAL/ABBREVIATIONS ===
		{"partial", "qty", "ecommerce", "e11", "Quantity"},
		{"partial", "nav menu", "ecommerce", "e13", "Main navigation"},

		// === EDGE CASES ===
		{"edge", "top right login link", "wikipedia", "e8", "Log in"},
	}

	catResults := make(map[string]struct{ pass, total int })

	for _, tc := range allCases {
		t.Run(fmt.Sprintf("%s_%s_%s", tc.site, tc.category, strings.ReplaceAll(tc.query, " ", "_")), func(t *testing.T) {
			result, err := matcher.Find(context.Background(), tc.query, sites[tc.site], types.FindOptions{
				Threshold: 0.1,
				TopK:      5,
			})
			if err != nil {
				t.Fatal(err)
			}

			pass := false
			for i, m := range result.Matches {
				if i >= 3 {
					break
				}
				if m.Ref == tc.wantRef {
					pass = true
					break
				}
			}

			cr := catResults[tc.category]
			cr.total++
			if pass {
				cr.pass++
				t.Logf("query=%q -> %s score=%.3f", tc.query, tc.wantName, result.BestScore)
			} else {
				t.Logf("MISS query=%q wanted=%s (%s) got=%s score=%.3f",
					tc.query, tc.wantRef, tc.wantName, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  ref=%s score=%.3f name=%s", m.Ref, m.Score, m.Name)
				}
			}
			catResults[tc.category] = cr
		})
	}

	// Print summary table
	t.Logf("\n╔══════════════════════════════════════════════════╗")
	t.Logf("║        MULTI-SITE EVALUATION SUMMARY            ║")
	t.Logf("╠══════════════════════════════════════════════════╣")
	totalP, totalT := 0, 0
	for _, cat := range []string{"exact", "synonym", "paraphrase", "partial", "edge"} {
		cr := catResults[cat]
		pct := 0.0
		if cr.total > 0 {
			pct = 100 * float64(cr.pass) / float64(cr.total)
		}
		t.Logf("║  %-14s  %d/%d  (%.0f%%)                       ║", cat, cr.pass, cr.total, pct)
		totalP += cr.pass
		totalT += cr.total
	}
	pct := 0.0
	if totalT > 0 {
		pct = 100 * float64(totalP) / float64(totalT)
	}
	t.Logf("╠══════════════════════════════════════════════════╣")
	t.Logf("║  TOTAL           %d/%d  (%.0f%%)                      ║", totalP, totalT, pct)
	t.Logf("╚══════════════════════════════════════════════════╝")
}

// ---------------------------------------------------------------------------
// Round-2 bug-fix tests
// ---------------------------------------------------------------------------
