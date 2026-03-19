package semantic

import "strings"

// uiSynonyms maps common UI action/element terms to their synonyms.
// During lexical scoring the query tokens are expanded with synonyms
// so that "sign in" can match "log in", "register" can match
// "create account", etc.
//
// Keys are canonical terms (lowercase). Values are slices of
// equivalent terms that should be treated as matches. The mapping
// is bidirectional: if "login" maps to "signin", then "signin"
// also maps back to "login" via the reverse index built at init.
var uiSynonyms = map[string][]string{
	// Authentication & account actions
	"login":    {"signin", "log in", "sign in", "authenticate", "logon", "log on"},
	"logout":   {"signout", "log out", "sign out", "logoff"},
	"register": {"signup", "sign up", "create account", "join", "enroll"},
	"password": {"passcode", "passphrase", "pwd"},
	"username": {"userid", "user name", "user id", "login name"},
	"email":    {"e-mail", "mail", "email address"},
	"forgot":   {"reset", "recover", "lost"},

	// Navigation & search
	"search":   {"find", "lookup", "look up", "query", "filter"},
	"menu":     {"navigation", "nav", "sidebar", "hamburger"},
	"home":     {"homepage", "main page", "start", "landing"},
	"back":     {"return", "go back", "previous"},
	"next":     {"continue", "proceed", "forward", "advance"},
	"previous": {"prev", "back", "prior"},
	"close":    {"dismiss", "exit", "x", "cancel"},
	"open":     {"expand", "show", "reveal"},
	"settings": {"preferences", "options", "configuration", "config"},

	// Form actions
	"submit":   {"send", "confirm", "apply", "save", "done", "go"},
	"cancel":   {"abort", "discard", "nevermind"},
	"edit":     {"modify", "change", "update"},
	"delete":   {"remove", "erase", "trash", "discard"},
	"add":      {"create", "new", "insert", "plus"},
	"upload":   {"attach", "choose file", "browse"},
	"download": {"export", "save as", "get"},

	// UI elements
	"button":       {"btn", "cta"},
	"input":        {"field", "textbox", "text box", "text field"},
	"dropdown":     {"select", "combobox", "combo box", "picker", "listbox"},
	"checkbox":     {"check box", "tick", "toggle"},
	"link":         {"anchor", "hyperlink", "href"},
	"tab":          {"panel", "pane"},
	"modal":        {"dialog", "dialogue", "popup", "pop up", "overlay"},
	"notification": {"alert", "toast", "banner", "message"},
	"tooltip":      {"hint", "info", "help text"},
	"avatar":       {"profile picture", "profile pic", "user image", "photo"},

	// Shopping & e-commerce
	"cart":     {"basket", "bag", "shopping cart"},
	"checkout": {"pay", "payment", "purchase", "buy", "place order", "order", "proceed to payment"},
	"price":    {"cost", "amount", "total"},
	"quantity": {"qty", "count", "amount"},

	// Content
	"image":       {"img", "picture", "photo", "icon"},
	"video":       {"clip", "media", "player"},
	"title":       {"heading", "header", "headline"},
	"description": {"desc", "summary", "subtitle", "caption"},
	"list":        {"items", "collection", "grid"},

	// Common actions
	"click":   {"press", "tap", "hit", "select"},
	"scroll":  {"swipe", "slide"},
	"drag":    {"move", "reorder"},
	"copy":    {"duplicate", "clone"},
	"paste":   {"insert"},
	"undo":    {"revert", "rollback"},
	"redo":    {"repeat"},
	"refresh": {"reload", "update"},
	"share":   {"send", "forward"},
	"like":    {"favorite", "favourite", "heart", "star", "upvote"},
	"accept":  {"agree", "allow", "ok", "okay", "yes", "confirm"},
	"reject":  {"deny", "decline", "refuse", "no"},
}

// synonymIndex is the flattened bidirectional lookup: given any token
// it returns all equivalent tokens including those obtained by walking
// the reverse mapping.
var synonymIndex map[string]map[string]bool

func init() {
	synonymIndex = buildSynonymIndex(uiSynonyms)
}

// buildSynonymIndex builds a bidirectional synonym lookup from the
// canonical synonym table.
func buildSynonymIndex(table map[string][]string) map[string]map[string]bool {
	idx := make(map[string]map[string]bool)

	ensure := func(key string) {
		if idx[key] == nil {
			idx[key] = make(map[string]bool)
		}
	}

	for canonical, synonyms := range table {
		ensure(canonical)
		for _, syn := range synonyms {
			idx[canonical][syn] = true
		}

		for _, syn := range synonyms {
			ensure(syn)
			idx[syn][canonical] = true
			for _, other := range synonyms {
				if other != syn {
					idx[syn][other] = true
				}
			}
		}
	}

	// Handle multi-word entries: also index individual words to
	// the compound form. E.g. "sign in" indexes under "sign" and "in"
	// as joined tokens so that tokenized "sign" + "in" can resolve.
	// This is handled during expansion, not here.

	return idx
}

// expandWithSynonyms takes a set of query tokens and returns an expanded
// set that includes synonym matches. Multi-word synonyms are split into
// individual tokens during expansion.
//
// The expansion is conservative: only ONE synonym expansion per token is
// applied (the one that appears in the target description tokens) to
// avoid combinatorial explosion.
func expandWithSynonyms(queryTokens []string, descTokens []string) []string {
	descSet := make(map[string]bool, len(descTokens))
	for _, dt := range descTokens {
		descSet[dt] = true
	}

	// Also join consecutive query tokens to check multi-word entries.
	// E.g. query ["sign", "in"] -> check "sign in"
	queryPhrases := buildPhrases(queryTokens, 3) // up to 3-word phrases

	expanded := make([]string, 0, len(queryTokens)*2)
	usedIndices := make(map[int]bool) // track which query tokens were consumed by phrase expansion

	// First pass: try multi-word phrase expansion.
	for _, phrase := range queryPhrases {
		if syns, ok := synonymIndex[phrase.text]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				for _, st := range synTokens {
					if descSet[st] {
						// This synonym has tokens in the description — add them.
						expanded = append(expanded, synTokens...)
						for idx := phrase.startIdx; idx <= phrase.endIdx; idx++ {
							usedIndices[idx] = true
						}
						break
					}
				}
			}
		}
	}

	// Second pass: single-token expansion for tokens not consumed by phrases.
	for i, qt := range queryTokens {
		if usedIndices[i] {
			continue
		}
		expanded = append(expanded, qt)
		if syns, ok := synonymIndex[qt]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				for _, st := range synTokens {
					if descSet[st] {
						expanded = append(expanded, synTokens...)
						break
					}
				}
			}
		}
	}

	return expanded
}

// phrase represents a multi-word phrase built from consecutive tokens.
type phrase struct {
	text     string
	startIdx int
	endIdx   int
}

// buildPhrases generates all consecutive n-gram phrases (up to maxN words)
// from a token list.
func buildPhrases(tokens []string, maxN int) []phrase {
	var phrases []phrase
	for n := 2; n <= maxN && n <= len(tokens); n++ {
		for i := 0; i <= len(tokens)-n; i++ {
			phrases = append(phrases, phrase{
				text:     strings.Join(tokens[i:i+n], " "),
				startIdx: i,
				endIdx:   i + n - 1,
			})
		}
	}
	return phrases
}

// synonymScore computes an additional similarity contribution from
// synonym matching between query and description tokens. Returns a
// value in [0, 1] representing how much of the query was satisfied
// via synonyms.
func synonymScore(queryTokens, descTokens []string) float64 {
	if len(queryTokens) == 0 || len(descTokens) == 0 {
		return 0
	}

	descSet := make(map[string]bool, len(descTokens))
	for _, dt := range descTokens {
		descSet[dt] = true
	}

	matched := 0
	consumedIdx := make(map[int]bool)

	// Phrase matching first (higher priority — avoids double-counting components).
	queryPhrases := buildPhrases(queryTokens, 3)
	for _, p := range queryPhrases {
		if syns, ok := synonymIndex[p.text]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				allPresent := true
				for _, st := range synTokens {
					if !descSet[st] {
						allPresent = false
						break
					}
				}
				if allPresent {
					matched++
					for idx := p.startIdx; idx <= p.endIdx; idx++ {
						consumedIdx[idx] = true
					}
					break
				}
			}
		}
	}

	// Single-token matching for tokens not consumed by phrases.
	for i, qt := range queryTokens {
		if consumedIdx[i] {
			continue
		}
		if descSet[qt] {
			continue
		}
		if syns, ok := synonymIndex[qt]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				allPresent := true
				for _, st := range synTokens {
					if !descSet[st] {
						allPresent = false
						break
					}
				}
				if allPresent {
					matched++
					break
				}
			}
		}
	}

	return float64(matched) / float64(len(queryTokens))
}
