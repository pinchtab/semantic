package engine

import "strings"

// uiSynonyms maps UI terms to their equivalents. Bidirectional via synonymIndex.
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
	"tooltip":      {"hint", "info", "help text"},

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
        // Accessibility & ARIA
        "skip":       {"skip link", "skip to content", "bypass"},
        "focus":      {"focused", "active", "selection"},
        "hidden":     {"invisible", "concealed", "obscured"},
        "visible":    {"shown", "displayed", "appearing"},
        "disabled":   {"unavailable", "inactive", "grayed out"},
        "enabled":    {"available", "active", "clickable"},
        "required":   {"mandatory", "compulsory", "needed"},
        "optional":   {"not required", "choice"},
        "invalid":    {"error", "wrong", "incorrect"},
        "valid":      {"correct", "okay", "passed"},
        "loading":    {"busy", "processing", "wait"},
        "ready":      {"loaded", "complete", "done"},

        // Data & tables
        "table":      {"grid", "datagrid", "spreadsheet"},
        "row":        {"record", "entry", "line"},
        "column":     {"field", "attribute", "property"},
        "header":     {"head", "top row", "column header"},
        "footer":     {"bottom", "summary row"},
        "sort":       {"order", "arrange", "rank"},
        "filter":     {"narrow", "refine", "search within"},

        // Media controls
        "play":       {"start", "begin", "run"},
        "pause":      {"stop", "halt", "freeze"},
        "stop":       {"end", "terminate", "finish"},
        "mute":       {"silence", "sound off", "audio off"},
        "unmute":     {"sound on", "audio on", "enable sound"},
        "volume":     {"sound", "audio level", "loudness"},
        "fullscreen": {"full screen", "maximize", "expand"},

        // File operations
        "file":       {"document", "attachment"},
        "folder":     {"directory", "catalog"},
        "rename":     {"rename file", "change name"},
        "move":       {"relocate", "transfer"},
        "zip":        {"compress", "archive", "package"},
        "unzip":      {"extract", "decompress", "open archive"},

        // Communication
        "message":    {"msg", "note", "communication"},
        "reply":      {"respond", "answer", "react"},
        "forward":    {"send on", "pass along"},
        "compose":    {"write", "create message", "draft"},
        "inbox":      {"messages", "mail inbox"},
        "sent":       {"sent items", "outbox"},
        "draft":      {"drafts", "unfinished"},

        // User profile & settings
        "profile":    {"account", "user profile", "my account"},
        "avatar":     {"profile picture", "profile pic", "user image", "photo", "gravatar"},
        "bio":        {"biography", "about me", "description"},
        "timezone":   {"time zone", "local time"},
        "language":   {"locale", "lang", "translation"},
        "theme":      {"appearance", "look", "skin", "mode"},
        "dark":       {"dark mode", "night mode"},
        "light":      {"light mode", "day mode"},

        // Notifications & alerts
        "notification": {"alert", "toast", "banner", "message", "ping"},
        "badge":        {"counter", "indicator", "dot", "mark"},
        "unread":       {"new", "unseen", "fresh"},
        "read":         {"seen", "opened", "viewed"},
        "dismiss":      {"close", "clear", "remove notification"},

        // Pagination & navigation
        "first":        {"start", "beginning", "page one"},
        "last":         {"end", "final", "latest page"},
        "page":         {"screen", "view"},
        "per_page":     {"items per page", "show", "display"},
        "goto":         {"jump to", "go to page", "navigate to"},

        // Selection & multi-select
        "select_all":   {"check all", "choose all", "mark all"},
        "deselect":     {"uncheck", "unmark", "clear selection"},
        "selected":     {"checked", "marked", "chosen"},
        "deselected":   {"unchecked", "unmarked"},

        // Help & support
        "help":         {"support", "assistance", "guide"},
        "faq":          {"frequently asked questions", "common questions"},
        "contact":      {"get in touch", "reach out", "support contact"},
        "feedback":     {"review", "rating", "comment"},
        "report":       {"flag", "notify", "submit issue"},

        // Security & privacy
        "privacy":      {"private", "confidentiality"},
        "terms":        {"tos", "terms of service", "terms and conditions"},
        "cookie":       {"cookies", "tracking"},
        "gdpr":         {"data protection", "privacy regulation"},
        "2fa":          {"two factor", "two-factor authentication", "mfa", "multi-factor"},
        "verify":       {"verification", "confirm identity", "authenticate"},

        // E-commerce extended
        "wishlist":     {"favorites", "saved items", "wish list"},
        "compare":      {"comparison", "vs", "versus"},
        "review":       {"rating", "customer review", "feedback"},
        "stock":        {"availability", "in stock", "inventory"},
        "out_of_stock": {"unavailable", "sold out", "no stock"},
        "discount":     {"sale", "offer", "deal", "promotion"},
        "coupon":       {"promo code", "voucher", "discount code"},

        // Date & time
        "today":        {"current day", "this day"},
        "yesterday":    {"previous day", "last day"},
        "tomorrow":     {"next day", "following day"},
        "now":          {"current time", "present"},
        "calendar":     {"date picker", "scheduler", "agenda"},
        "schedule":     {"plan", "book", "appointment"},

        // Analytics & metrics
        "analytics":    {"stats", "statistics", "metrics"},
        "dashboard":    {"overview", "home base", "control panel"},
        "chart":        {"graph", "plot", "visualization"},
        "trend":        {"pattern", "direction", "movement"},
        "export":       {"download data", "save report", "generate file"},
        "import":       {"upload data", "load file", "bring in"},
}

var synonymIndex map[string]map[string]bool

func init() {
	synonymIndex = buildSynonymIndex(uiSynonyms)
}

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

// expandWithSynonyms adds synonym tokens from descTokens. Conservative:
// only one expansion per query token to avoid combinatorial explosion.
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

type phrase struct {
	text     string
	startIdx int
	endIdx   int
}

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
