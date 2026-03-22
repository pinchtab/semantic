package semantic

// benchmark_study_test.go — Controlled benchmark study
//
//   20 queries × 10 page types comparing:
//     • LexicalMatcher (Jaccard + stopwords + role boost)
//     • EmbeddingMatcher (128-dim HashingEmbedder + cosine similarity)
//     • CombinedMatcher (0.6 lexical + 0.4 embedding)
//
//   Metrics reported:
//     • Acc@1 — correct element is the top-ranked result
//     • Acc@3 — correct element appears in top-3 results
//     • Mean Latency (µs) per matcher
//
//   Run:
//     go test ./internal/semantic/ -run TestBenchmarkStudy -v
//
//   Or with benchmark timing detail:
//     go test ./internal/semantic/ -run TestBenchmarkStudy -v -count 5

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// Data structures
// -----------------------------------------------------------------------

// studyCase is a single (page, query, expected-ref) triple.
type studyCase struct {
	page        string // human-readable page name
	query       string // natural language query
	expectedRef string // ref of the ground-truth element
	elements    []ElementDescriptor
}

// studyResult records one matcher's answer for one case.
type studyResult struct {
	matcherName string
	caseName    string
	page        string
	hit1        bool // Acc@1: best ref == expected
	hit3        bool // Acc@3: expected ref in top-3
	latencyNs   int64
	bestRef     string
	bestScore   float64
}

// -----------------------------------------------------------------------
// Ground-truth page element sets (10 pages × 2 queries each = 20 cases)
// -----------------------------------------------------------------------

func studyCases() []studyCase {
	// ---- Page 1: Login Form ------------------------------------------------
	login := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Sign In"},
		{Ref: "e1", Role: "textbox", Name: "Email address"},
		{Ref: "e2", Role: "textbox", Name: "Password"},
		{Ref: "e3", Role: "checkbox", Name: "Remember me"},
		{Ref: "e4", Role: "button", Name: "Sign In"},
		{Ref: "e5", Role: "link", Name: "Forgot your password?"},
		{Ref: "e6", Role: "link", Name: "Create account"},
		{Ref: "e7", Role: "button", Name: "Continue with Google"},
		{Ref: "e8", Role: "button", Name: "Continue with Apple"},
		{Ref: "e9", Role: "img", Name: "Company logo"},
	}

	// ---- Page 2: Registration Form -----------------------------------------
	register := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Create your account"},
		{Ref: "e1", Role: "textbox", Name: "First name"},
		{Ref: "e2", Role: "textbox", Name: "Last name"},
		{Ref: "e3", Role: "textbox", Name: "Email"},
		{Ref: "e4", Role: "textbox", Name: "Password"},
		{Ref: "e5", Role: "textbox", Name: "Confirm password"},
		{Ref: "e6", Role: "combobox", Name: "Date of birth"},
		{Ref: "e7", Role: "combobox", Name: "Country or region"},
		{Ref: "e8", Role: "checkbox", Name: "I agree to the Terms and Conditions"},
		{Ref: "e9", Role: "checkbox", Name: "Subscribe to marketing emails"},
		{Ref: "e10", Role: "button", Name: "Create account"},
		{Ref: "e11", Role: "link", Name: "Already have an account? Log in"},
	}

	// ---- Page 3: E-commerce Product Page -----------------------------------
	product := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Wireless Noise-Cancelling Headphones"},
		{Ref: "e1", Role: "text", Name: "$299.99"},
		{Ref: "e2", Role: "combobox", Name: "Color", Value: "Midnight Black"},
		{Ref: "e3", Role: "spinbutton", Name: "Quantity", Value: "1"},
		{Ref: "e4", Role: "button", Name: "Add to cart"},
		{Ref: "e5", Role: "button", Name: "Buy now"},
		{Ref: "e6", Role: "button", Name: "Add to wishlist"},
		{Ref: "e7", Role: "tab", Name: "Description"},
		{Ref: "e8", Role: "tab", Name: "Reviews"},
		{Ref: "e9", Role: "tab", Name: "Specifications"},
		{Ref: "e10", Role: "img", Name: "Product image front view"},
		{Ref: "e11", Role: "text", Name: "Free shipping on orders over $50"},
	}

	// ---- Page 4: Navigation Header -----------------------------------------
	nav := []ElementDescriptor{
		{Ref: "e0", Role: "img", Name: "Site logo"},
		{Ref: "e1", Role: "link", Name: "Home"},
		{Ref: "e2", Role: "link", Name: "Products"},
		{Ref: "e3", Role: "link", Name: "Pricing"},
		{Ref: "e4", Role: "link", Name: "Blog"},
		{Ref: "e5", Role: "link", Name: "About Us"},
		{Ref: "e6", Role: "link", Name: "Contact"},
		{Ref: "e7", Role: "search", Name: "Search"},
		{Ref: "e8", Role: "button", Name: "Search"},
		{Ref: "e9", Role: "button", Name: "Open cart"},
		{Ref: "e10", Role: "link", Name: "Sign in"},
		{Ref: "e11", Role: "button", Name: "Open navigation menu"},
	}

	// ---- Page 5: Analytics Dashboard ---------------------------------------
	dashboard := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Dashboard Overview"},
		{Ref: "e1", Role: "button", Name: "Export Report"},
		{Ref: "e2", Role: "button", Name: "Add Widget"},
		{Ref: "e3", Role: "combobox", Name: "Date range", Value: "Last 30 days"},
		{Ref: "e4", Role: "text", Name: "Total Revenue", Value: "$128,450"},
		{Ref: "e5", Role: "text", Name: "Active Users", Value: "8,302"},
		{Ref: "e6", Role: "text", Name: "Conversion Rate", Value: "3.4%"},
		{Ref: "e7", Role: "text", Name: "Avg Session Duration", Value: "4m 12s"},
		{Ref: "e8", Role: "button", Name: "Refresh Data"},
		{Ref: "e9", Role: "link", Name: "View detailed report"},
		{Ref: "e10", Role: "tab", Name: "Overview"},
		{Ref: "e11", Role: "tab", Name: "Revenue"},
		{Ref: "e12", Role: "tab", Name: "Users"},
		{Ref: "e13", Role: "button", Name: "Notifications"},
	}

	// ---- Page 6: Search Results Page ---------------------------------------
	search := []ElementDescriptor{
		{Ref: "e0", Role: "search", Name: "Search"},
		{Ref: "e1", Role: "button", Name: "Search"},
		{Ref: "e2", Role: "heading", Name: "Search Results for \"golang\""},
		{Ref: "e3", Role: "combobox", Name: "Sort by", Value: "Relevance"},
		{Ref: "e4", Role: "checkbox", Name: "Filter: Last 24 hours"},
		{Ref: "e5", Role: "checkbox", Name: "Filter: Images"},
		{Ref: "e6", Role: "checkbox", Name: "Filter: Videos"},
		{Ref: "e7", Role: "link", Name: "Next page"},
		{Ref: "e8", Role: "link", Name: "Previous page"},
		{Ref: "e9", Role: "button", Name: "Clear filters"},
		{Ref: "e10", Role: "text", Name: "About 4,230,000 results"},
	}

	// ---- Page 7: Admin Data Table ------------------------------------------
	table := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Order Management"},
		{Ref: "e1", Role: "search", Name: "Search orders"},
		{Ref: "e2", Role: "button", Name: "Create order"},
		{Ref: "e3", Role: "button", Name: "Export to CSV"},
		{Ref: "e4", Role: "combobox", Name: "Status filter", Value: "All"},
		{Ref: "e5", Role: "combobox", Name: "Rows per page", Value: "25"},
		{Ref: "e6", Role: "columnheader", Name: "Order ID"},
		{Ref: "e7", Role: "columnheader", Name: "Customer"},
		{Ref: "e8", Role: "columnheader", Name: "Total"},
		{Ref: "e9", Role: "columnheader", Name: "Status"},
		{Ref: "e10", Role: "button", Name: "Previous page"},
		{Ref: "e11", Role: "button", Name: "Next page"},
		{Ref: "e12", Role: "button", Name: "Bulk delete"},
		{Ref: "e13", Role: "checkbox", Name: "Select all orders"},
	}

	// ---- Page 8: Confirmation Modal ----------------------------------------
	modal := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Dashboard"},
		{Ref: "e1", Role: "button", Name: "New Project"},
		{Ref: "e2", Role: "dialog", Name: "Delete Project"},
		{Ref: "e3", Role: "heading", Name: "Delete Project"},
		{Ref: "e4", Role: "text", Name: "This will permanently delete the project and all its data. This action cannot be undone."},
		{Ref: "e5", Role: "textbox", Name: "Type project name to confirm"},
		{Ref: "e6", Role: "button", Name: "Delete project"},
		{Ref: "e7", Role: "button", Name: "Cancel"},
		{Ref: "e8", Role: "button", Name: "Close"},
		{Ref: "e9", Role: "navigation", Name: "Sidebar"},
	}

	// ---- Page 9: Settings / Preferences Page --------------------------------
	settings := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Account Settings"},
		{Ref: "e1", Role: "textbox", Name: "Display name"},
		{Ref: "e2", Role: "textbox", Name: "Email address"},
		{Ref: "e3", Role: "textbox", Name: "Phone number"},
		{Ref: "e4", Role: "combobox", Name: "Language", Value: "English"},
		{Ref: "e5", Role: "combobox", Name: "Timezone", Value: "UTC-5"},
		{Ref: "e6", Role: "switch", Name: "Email notifications"},
		{Ref: "e7", Role: "switch", Name: "Push notifications"},
		{Ref: "e8", Role: "switch", Name: "Dark mode"},
		{Ref: "e9", Role: "button", Name: "Save changes"},
		{Ref: "e10", Role: "button", Name: "Cancel"},
		{Ref: "e11", Role: "button", Name: "Delete account"},
		{Ref: "e12", Role: "link", Name: "Change password"},
	}

	// ---- Page 10: Checkout / Payment Page ----------------------------------
	checkout := []ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Checkout"},
		{Ref: "e1", Role: "textbox", Name: "Full name"},
		{Ref: "e2", Role: "textbox", Name: "Email"},
		{Ref: "e3", Role: "textbox", Name: "Shipping address"},
		{Ref: "e4", Role: "textbox", Name: "City"},
		{Ref: "e5", Role: "textbox", Name: "Postal code"},
		{Ref: "e6", Role: "combobox", Name: "Country"},
		{Ref: "e7", Role: "textbox", Name: "Card number"},
		{Ref: "e8", Role: "textbox", Name: "Expiry date"},
		{Ref: "e9", Role: "textbox", Name: "CVV"},
		{Ref: "e10", Role: "checkbox", Name: "Save card for future use"},
		{Ref: "e11", Role: "button", Name: "Place order"},
		{Ref: "e12", Role: "button", Name: "Back to cart"},
		{Ref: "e13", Role: "link", Name: "Apply coupon code"},
	}

	return []studyCase{
		// Page 1: Login — 2 queries
		{"Login Form", "sign in button", "e4", login},
		{"Login Form", "email input field", "e1", login},

		// Page 2: Registration — 2 queries
		{"Registration Form", "create account button", "e10", register},
		{"Registration Form", "confirm password field", "e5", register},

		// Page 3: E-commerce Product — 2 queries
		{"Product Page", "add to cart", "e4", product},
		{"Product Page", "product reviews tab", "e8", product},

		// Page 4: Navigation — 2 queries
		{"Navigation Header", "search box", "e7", nav},
		{"Navigation Header", "shopping cart", "e9", nav},

		// Page 5: Dashboard — 2 queries
		{"Analytics Dashboard", "export report", "e1", dashboard},
		{"Analytics Dashboard", "date range selector", "e3", dashboard},

		// Page 6: Search Results — 2 queries
		{"Search Results", "search input", "e0", search},
		{"Search Results", "next page link", "e7", search},

		// Page 7: Data Table — 2 queries
		{"Admin Data Table", "search orders", "e1", table},
		{"Admin Data Table", "export csv", "e3", table},

		// Page 8: Modal — 2 queries
		{"Confirmation Modal", "cancel button", "e7", modal},
		{"Confirmation Modal", "confirm deletion input", "e5", modal},

		// Page 9: Settings — 2 queries
		{"Settings Page", "save changes button", "e9", settings},
		{"Settings Page", "dark mode toggle", "e8", settings},

		// Page 10: Checkout — 2 queries
		{"Checkout Page", "place order button", "e11", checkout},
		{"Checkout Page", "card number field", "e7", checkout},
	}
}

// studyHardCases returns 10 intentionally-challenging query/element pairs
// designed to reveal differentiation between matchers:
//
//	Group A (query uses an abbreviation, 0 lexical word overlap):
//	  expects embedding to win via character n-gram similarity.
//
//	Group B (query uses a paraphrase / synonym):
//	  expects both matchers to struggle, revealing the ceiling of
//	  surface-form-only matching.
//
//	Group C (ambiguous — multiple equally-plausible elements):
//	  expects combined to win via score averaging.
func studyHardCases() []studyCase {
	// Reuse page element sets from studyCases.
	cases := studyCases()

	// Helper: find element slice for a given page name.
	pageElems := map[string][]ElementDescriptor{}
	for _, c := range cases {
		if _, ok := pageElems[c.page]; !ok {
			pageElems[c.page] = c.elements
		}
	}

	product := pageElems["Product Page"]
	checkout := pageElems["Checkout Page"]
	login := pageElems["Login Form"]
	register := pageElems["Registration Form"]
	settings := pageElems["Settings Page"]
	table := pageElems["Admin Data Table"]
	modal := pageElems["Confirmation Modal"]

	return []studyCase{
		// ── Group A: Abbreviations (lexical = 0, embedding has n-gram overlap) ──

		// "specs"     ↔ "Specifications" — shares "^sp","spe","pec","spec" (4-gram)
		{"Product Page [HARD]", "specs tab", "e9", product},

		// "qty"       ↔ "Quantity" — shares "^q","ty","y$"
		{"Product Page [HARD]", "qty input", "e3", product},

		// "addr"      ↔ "address" — shares "^a","ad","dd","dr"
		{"Checkout Page [HARD]", "addr field", "e3", checkout},

		// "pwd"       ↔ "Password" — shares "^p","d$" (weak but present)
		{"Login Form [HARD]", "pwd textbox", "e2", login},

		// "notifs"    ↔ "notifications" — shares "not","oti"
		{"Settings Page [HARD]", "toggle email notifs", "e6", settings},

		// ── Group B: Paraphrases / synonyms (no character overlap, both expected to struggle) ──

		// "download"  ↔ "Export to CSV"  — no shared characters
		{"Admin Data Table [HARD]", "download table data", "e3", table},

		// "proceed"   ↔ "Place order"    — no shared characters
		{"Checkout Page [HARD]", "proceed to payment", "e11", checkout},

		// "sign up"   ↔ "Create account" — no shared characters
		{"Registration Form [HARD]", "sign up now", "e10", register},

		// "dismiss"   ↔ "Cancel"         — no shared characters
		{"Confirmation Modal [HARD]", "dismiss dialog", "e7", modal},

		// ── Group C: Ambiguous (multiple "button" elements, exact name helps) ──

		// "dark theme" ↔ "Dark mode" switch — "dark" word-matches exactly
		{"Settings Page [HARD]", "dark theme switch", "e8", settings},
	}
}

// -----------------------------------------------------------------------
// Core evaluation logic
// -----------------------------------------------------------------------

func runMatcher(
	name string,
	matcher ElementMatcher,
	cases []studyCase,
) []studyResult {
	ctx := context.Background()
	opts := FindOptions{Threshold: 0.0, TopK: 3}
	results := make([]studyResult, 0, len(cases))

	for _, c := range cases {
		start := time.Now()
		fr, err := matcher.Find(ctx, c.query, c.elements, opts)
		elapsed := time.Since(start).Nanoseconds()

		if err != nil {
			results = append(results, studyResult{
				matcherName: name,
				caseName:    fmt.Sprintf("%s | %q", c.page, c.query),
				page:        c.page,
				latencyNs:   elapsed,
			})
			continue
		}

		// Acc@1
		hit1 := fr.BestRef == c.expectedRef

		// Acc@3 — expected ref in any of the top-3 matches
		hit3 := hit1
		if !hit3 {
			for _, m := range fr.Matches {
				if m.Ref == c.expectedRef {
					hit3 = true
					break
				}
			}
		}

		results = append(results, studyResult{
			matcherName: name,
			caseName:    fmt.Sprintf("%s | %q", c.page, c.query),
			page:        c.page,
			hit1:        hit1,
			hit3:        hit3,
			latencyNs:   elapsed,
			bestRef:     fr.BestRef,
			bestScore:   fr.BestScore,
		})
	}

	return results
}

// -----------------------------------------------------------------------
// Report generation helpers
// -----------------------------------------------------------------------

func percent(n, total int) string {
	if total == 0 {
		return " 0.0%"
	}
	return fmt.Sprintf("%5.1f%%", float64(n)/float64(total)*100)
}

func meanLatencyUs(results []studyResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum int64
	for _, r := range results {
		sum += r.latencyNs
	}
	return float64(sum) / float64(len(results)) / 1000.0
}

func acc(results []studyResult, k int) (int, int) {
	hits := 0
	for _, r := range results {
		if k == 1 && r.hit1 {
			hits++
		}
		if k == 3 && r.hit3 {
			hits++
		}
	}
	return hits, len(results)
}

func printSeparator(t *testing.T, char string, width int) {
	t.Log(strings.Repeat(char, width))
}

// -----------------------------------------------------------------------
// Main study test
// -----------------------------------------------------------------------

func TestBenchmarkStudy(t *testing.T) {
	cases := studyCases()

	matchers := []struct {
		name    string
		matcher ElementMatcher
	}{
		{"Lexical", NewLexicalMatcher()},
		{"Embedding", NewEmbeddingMatcher(NewHashingEmbedder(128))},
		{"Combined", NewCombinedMatcher(NewHashingEmbedder(128))},
	}

	// Run all matchers
	allResults := make(map[string][]studyResult, len(matchers))
	for _, m := range matchers {
		allResults[m.name] = runMatcher(m.name, m.matcher, cases)
	}

	// ----------------------------------------------------------------
	// REPORT HEADER
	// ----------------------------------------------------------------
	const W = 72
	t.Log("")
	printSeparator(t, "═", W)
	t.Log("  SEMANTIC MATCHING — CONTROLLED BENCHMARK STUDY")
	t.Logf("  %d queries × %d page types  |  3 matchers  |  TopK=3  |  Threshold=0",
		len(cases), 10)
	printSeparator(t, "═", W)

	// ----------------------------------------------------------------
	// OVERALL SUMMARY TABLE
	// ----------------------------------------------------------------
	t.Log("")
	t.Log("  OVERALL RESULTS")
	t.Log("")
	t.Logf("  %-12s  %7s  %7s  %12s", "Matcher", "Acc@1", "Acc@3", "Latency (µs)")
	t.Log("  " + strings.Repeat("-", 46))

	type summaryRow struct {
		name    string
		acc1    int
		acc3    int
		total   int
		latency float64
	}
	rows := make([]summaryRow, 0, len(matchers))

	for _, m := range matchers {
		res := allResults[m.name]
		h1, tot := acc(res, 1)
		h3, _ := acc(res, 3)
		lat := meanLatencyUs(res)
		rows = append(rows, summaryRow{m.name, h1, h3, tot, lat})
		t.Logf("  %-12s  %s  %s  %10.1f µs",
			m.name, percent(h1, tot), percent(h3, tot), lat)
	}

	t.Log("  " + strings.Repeat("-", 46))
	t.Log("")

	// ----------------------------------------------------------------
	// PER-PAGE BREAKDOWN
	// ----------------------------------------------------------------
	// Collect unique page names in order
	pages := make([]string, 0, 10)
	seen := map[string]bool{}
	for _, c := range cases {
		if !seen[c.page] {
			pages = append(pages, c.page)
			seen[c.page] = true
		}
	}

	printSeparator(t, "─", W)
	t.Log("  PER-PAGE BREAKDOWN  (Acc@1 / Acc@3)  ")
	printSeparator(t, "─", W)
	t.Log("")
	t.Logf("  %-26s  %-14s  %-14s  %-14s", "Page", "Lexical", "Embedding", "Combined")
	t.Log("  " + strings.Repeat("-", 66))

	for _, page := range pages {
		// Filter results for this page
		cols := make([]string, 0, 3)
		for _, m := range matchers {
			var pageRes []studyResult
			for _, r := range allResults[m.name] {
				if r.page == page {
					pageRes = append(pageRes, r)
				}
			}
			h1, tot := acc(pageRes, 1)
			h3, _ := acc(pageRes, 3)
			cols = append(cols, fmt.Sprintf("%s / %s", percent(h1, tot), percent(h3, tot)))
		}
		t.Logf("  %-26s  %-14s  %-14s  %-14s", page, cols[0], cols[1], cols[2])
	}
	t.Log("")

	// ----------------------------------------------------------------
	// DETAILED CASE-BY-CASE TABLE
	// ----------------------------------------------------------------
	printSeparator(t, "─", W)
	t.Log("  CASE-BY-CASE RESULTS  (✓ = hit, ✗ = miss)  ")
	printSeparator(t, "─", W)
	t.Log("")
	t.Logf("  %-6s  %-28s  %-8s  %-8s  %-8s  %-8s  %-8s  %-8s",
		"#", "Query", "Lex@1", "Lex@3", "Emb@1", "Emb@3", "Com@1", "Com@3")
	t.Log("  " + strings.Repeat("-", 88))

	mark := func(hit bool) string {
		if hit {
			return "  ✓"
		}
		return "  ✗"
	}

	for i, c := range cases {
		shortQ := c.query
		if len(shortQ) > 27 {
			shortQ = shortQ[:24] + "..."
		}

		lex := allResults["Lexical"][i]
		emb := allResults["Embedding"][i]
		com := allResults["Combined"][i]

		t.Logf("  %3d.   %-28s  %-8s  %-8s  %-8s  %-8s  %-8s  %-8s",
			i+1, shortQ,
			mark(lex.hit1), mark(lex.hit3),
			mark(emb.hit1), mark(emb.hit3),
			mark(com.hit1), mark(com.hit3),
		)
	}
	t.Log("")

	// ----------------------------------------------------------------
	// MISSED CASES ANALYSIS
	// ----------------------------------------------------------------
	printSeparator(t, "─", W)
	t.Log("  MISSED CASES ANALYSIS  (Acc@1 misses)")
	printSeparator(t, "─", W)
	t.Log("")

	for _, m := range matchers {
		missCount := 0
		for _, r := range allResults[m.name] {
			if !r.hit1 {
				missCount++
			}
		}
		if missCount == 0 {
			t.Logf("  %s: perfect score — no Acc@1 misses", m.name)
			continue
		}
		t.Logf("  %s misses (%d):", m.name, missCount)
		for i, r := range allResults[m.name] {
			if r.hit1 {
				continue
			}
			t.Logf("    [%2d] %-28s  expected=%-4s  got=%-4s  score=%.3f",
				i+1, fmt.Sprintf("%q", cases[i].query),
				cases[i].expectedRef, r.bestRef, r.bestScore)
		}
	}
	t.Log("")

	// ----------------------------------------------------------------
	// MATCHER COMPARISON: cases where they DISAGREE
	// ----------------------------------------------------------------
	printSeparator(t, "─", W)
	t.Log("  DISAGREMENT ANALYSIS  (where Combined beats both)")
	printSeparator(t, "─", W)
	t.Log("")

	improvements := 0
	for i := range cases {
		lex := allResults["Lexical"][i]
		emb := allResults["Embedding"][i]
		com := allResults["Combined"][i]

		if !lex.hit1 && !emb.hit1 && com.hit1 {
			improvements++
			t.Logf("  [%2d] %q — Combined rescued (lex✗ emb✗ com✓)",
				i+1, cases[i].query)
		}
		if lex.hit1 && emb.hit1 && !com.hit1 {
			t.Logf("  [%2d] %q — Combined degraded (lex✓ emb✓ com✗) !",
				i+1, cases[i].query)
		}
	}
	if improvements == 0 {
		t.Log("  No unique rescues by Combined (or none needed)")
	}
	t.Log("")

	// ----------------------------------------------------------------
	// LATENCY COMPARISON
	// ----------------------------------------------------------------
	printSeparator(t, "─", W)
	t.Log("  LATENCY SUMMARY")
	printSeparator(t, "─", W)
	t.Log("")

	// Sort by latency ascending
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].latency < rows[j].latency
	})

	baseline := rows[0].latency
	for _, row := range rows {
		overhead := ""
		if row.latency > baseline {
			overhead = fmt.Sprintf("  (+%.1fx)", row.latency/baseline)
		}
		t.Logf("  %-12s  %8.2f µs%s", row.name, row.latency, overhead)
	}
	t.Log("")

	// ----------------------------------------------------------------
	// FINAL VERDICT
	// ----------------------------------------------------------------
	printSeparator(t, "═", W)
	t.Log("  VERDICT")
	printSeparator(t, "─", W)
	t.Log("")

	// Find best Acc@1
	bestAcc1Name := ""
	bestAcc1 := -1
	for _, m := range matchers {
		h1, tot := acc(allResults[m.name], 1)
		pct := h1 * 100 / tot
		if pct > bestAcc1 {
			bestAcc1 = pct
			bestAcc1Name = m.name
		}
	}

	lexH1, tot := acc(allResults["Lexical"], 1)
	embH1, _ := acc(allResults["Embedding"], 1)
	comH1, _ := acc(allResults["Combined"], 1)
	lexH3, _ := acc(allResults["Lexical"], 3)
	embH3, _ := acc(allResults["Embedding"], 3)
	comH3, _ := acc(allResults["Combined"], 3)

	t.Logf("  Best Acc@1:  %s (%d/%d = %s)", bestAcc1Name, comH1, tot, percent(comH1, tot))
	t.Log("")
	t.Logf("  Acc@1 — Lexical: %s  Embedding: %s  Combined: %s",
		percent(lexH1, tot), percent(embH1, tot), percent(comH1, tot))
	t.Logf("  Acc@3 — Lexical: %s  Embedding: %s  Combined: %s",
		percent(lexH3, tot), percent(embH3, tot), percent(comH3, tot))
	t.Log("")
	t.Logf("  Mean latency — Lexical: %.1fµs  Embedding: %.1fµs  Combined: %.1fµs",
		meanLatencyUs(allResults["Lexical"]),
		meanLatencyUs(allResults["Embedding"]),
		meanLatencyUs(allResults["Combined"]))
	t.Log("")
	printSeparator(t, "═", W)
	t.Log("")

	// ================================================================
	// PART II — HARD CASES (abbreviations, synonyms, paraphrases)
	// ================================================================
	hardCases := studyHardCases()

	allHard := make(map[string][]studyResult, len(matchers))
	for _, m := range matchers {
		allHard[m.name] = runMatcher(m.name, m.matcher, hardCases)
	}

	t.Log("")
	printSeparator(t, "═", W)
	t.Log("  PART II — HARD CASES  (abbreviations, synonyms, paraphrases)")
	t.Logf("  %d queries  |  3 matcher groups:  A=abbrev  B=synonym  C=ambiguous",
		len(hardCases))
	printSeparator(t, "═", W)

	// Overall hard summary
	t.Log("")
	t.Log("  HARD OVERALL")
	t.Log("")
	t.Logf("  %-12s  %7s  %7s  %12s", "Matcher", "Acc@1", "Acc@3", "Latency (µs)")
	t.Log("  " + strings.Repeat("-", 46))
	for _, m := range matchers {
		res := allHard[m.name]
		h1, tot := acc(res, 1)
		h3, _ := acc(res, 3)
		lat := meanLatencyUs(res)
		t.Logf("  %-12s  %s  %s  %10.1f µs",
			m.name, percent(h1, tot), percent(h3, tot), lat)
	}
	t.Log("  " + strings.Repeat("-", 46))
	t.Log("")

	// Case-by-case hard results
	printSeparator(t, "─", W)
	t.Log("  HARD CASE-BY-CASE  (✓ = hit, ✗ = miss)")
	printSeparator(t, "─", W)
	t.Log("")
	t.Logf("  %-3s  %-3s  %-30s  %-8s  %-8s  %-8s  %-8s  %-8s  %-8s",
		"#", "Grp", "Query", "Lex@1", "Lex@3", "Emb@1", "Emb@3", "Com@1", "Com@3")
	t.Log("  " + strings.Repeat("-", 90))

	groups := []string{"A", "A", "A", "A", "A", "B", "B", "B", "B", "C"}
	for i, c := range hardCases {
		grp := groups[i]
		shortQ := c.query
		if len(shortQ) > 29 {
			shortQ = shortQ[:26] + "..."
		}
		lex := allHard["Lexical"][i]
		emb := allHard["Embedding"][i]
		com := allHard["Combined"][i]

		t.Logf("  %3d  %-3s  %-30s  %-8s  %-8s  %-8s  %-8s  %-8s  %-8s",
			i+1, grp, shortQ,
			mark(lex.hit1), mark(lex.hit3),
			mark(emb.hit1), mark(emb.hit3),
			mark(com.hit1), mark(com.hit3),
		)
	}
	t.Log("")

	// Per-group analysis
	printSeparator(t, "─", W)
	t.Log("  GROUP ANALYSIS")
	printSeparator(t, "─", W)
	t.Log("")
	t.Logf("  %-20s  %-14s  %-14s  %-14s", "Group", "Lexical Acc@1", "Embedding Acc@1", "Combined Acc@1")
	t.Log("  " + strings.Repeat("-", 66))

	groupDefs := []struct {
		label string
		ids   []int // 0-indexed
		desc  string
	}{
		{"A (abbreviations)", []int{0, 1, 2, 3, 4}, "expected: Emb ≥ Lex"},
		{"B (synonyms)", []int{5, 6, 7, 8}, "expected: both struggle"},
		{"C (ambiguous)", []int{9}, "expected: Combined wins"},
	}

	for _, gd := range groupDefs {
		for _, mname := range []string{"Lexical", "Embedding", "Combined"} {
			var subset []studyResult
			for _, idx := range gd.ids {
				subset = append(subset, allHard[mname][idx])
			}
			_ = subset
		}
		lexSub := filterByIdx(allHard["Lexical"], gd.ids)
		embSub := filterByIdx(allHard["Embedding"], gd.ids)
		comSub := filterByIdx(allHard["Combined"], gd.ids)
		lh1, lt := acc(lexSub, 1)
		eh1, et := acc(embSub, 1)
		ch1, ct := acc(comSub, 1)
		t.Logf("  %-20s  %-14s  %-15s  %-14s   %s",
			gd.label,
			percent(lh1, lt), percent(eh1, et), percent(ch1, ct),
			gd.desc)
	}
	t.Log("")

	// Combined vs Individual: misses that Combined rescues
	printSeparator(t, "─", W)
	t.Log("  COMBINED RESCUES  (in hard cases)")
	printSeparator(t, "─", W)
	t.Log("")
	rescueCount := 0
	for i := range hardCases {
		lex := allHard["Lexical"][i]
		emb := allHard["Embedding"][i]
		com := allHard["Combined"][i]
		if !lex.hit1 && !emb.hit1 && com.hit1 {
			rescueCount++
			t.Logf("  RESCUE [%d] %q — Lex✗ Emb✗ Com✓", i+1, hardCases[i].query)
		}
		if !lex.hit1 && emb.hit1 && com.hit1 {
			t.Logf("  EMB WIN [%d] %q — Lex✗ Emb✓ Com✓", i+1, hardCases[i].query)
		}
		if lex.hit1 && !emb.hit1 && com.hit1 {
			t.Logf("  LEX WIN [%d] %q — Lex✓ Emb✗ Com✓", i+1, hardCases[i].query)
		}
		if lex.hit1 && !emb.hit1 && !com.hit1 {
			t.Logf("  COM FAIL[%d] %q — Lex✓ Emb✗ Com✗  ← embedding dragged score down", i+1, hardCases[i].query)
		}
	}
	if rescueCount == 0 {
		t.Log("  No unique Combined rescues in hard cases")
	}
	t.Log("")

	// Score comparison for hard cases
	printSeparator(t, "─", W)
	t.Log("  SCORE DETAIL  (best score returned by each matcher per hard case)")
	printSeparator(t, "─", W)
	t.Log("")
	t.Logf("  %-3s  %-30s  %-6s  %-8s  %-8s  %-8s  %-8s",
		"#", "Query", "Want", "Lex score", "Emb score", "Com score", "Winner")
	t.Log("  " + strings.Repeat("-", 78))
	for i, c := range hardCases {
		lex := allHard["Lexical"][i]
		emb := allHard["Embedding"][i]
		com := allHard["Combined"][i]
		shortQ := c.query
		if len(shortQ) > 29 {
			shortQ = shortQ[:26] + "..."
		}
		var winner string
		if !lex.hit1 && emb.hit1 {
			winner = "Embedding"
		} else if lex.hit1 && !emb.hit1 {
			winner = "Lexical"
		} else if lex.hit1 && emb.hit1 {
			winner = "both"
		} else {
			winner = "none"
		}
		t.Logf("  %3d  %-30s  %-6s  %9.3f  %9.3f  %9.3f  %s",
			i+1, shortQ, c.expectedRef,
			lex.bestScore, emb.bestScore, com.bestScore, winner)
	}
	t.Log("")

	// Final combined verdict
	printSeparator(t, "═", W)
	t.Log("  FINAL VERDICT — EASY + HARD COMBINED")
	printSeparator(t, "─", W)
	t.Log("")

	allTotal := len(cases) + len(hardCases)
	for _, m := range matchers {
		combined := append(allResults[m.name], allHard[m.name]...)
		h1, _ := acc(combined, 1)
		h3, _ := acc(combined, 3)
		lat := meanLatencyUs(combined)
		t.Logf("  %-12s  Acc@1=%s  Acc@3=%s  µs=%.1f  (total %d queries)",
			m.name, percent(h1, allTotal), percent(h3, allTotal), lat, allTotal)
	}
	t.Log("")
	printSeparator(t, "═", W)
	t.Log("")
}

// filterByIdx returns results at specified 0-based indices.
func filterByIdx(results []studyResult, indices []int) []studyResult {
	out := make([]studyResult, 0, len(indices))
	for _, i := range indices {
		if i < len(results) {
			out = append(out, results[i])
		}
	}
	return out
}
