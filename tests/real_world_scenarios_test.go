package tests

import (
"context"
"testing"
"github.com/pinchtab/semantic"
)

func TestRealWorldEcommerceCheckout(t *testing.T) {
checkoutElements := []semantic.ElementDescriptor{
{Ref: "e1", Role: "button", Name: "Add to Cart"},
{Ref: "e2", Role: "button", Name: "Proceed to Checkout"},
{Ref: "e3", Role: "textbox", Name: "Card Number"},
{Ref: "e4", Role: "textbox", Name: "Expiry Date"},
{Ref: "e5", Role: "textbox", Name: "CVV"},
{Ref: "e6", Role: "button", Name: "Place Order"},
{Ref: "e7", Role: "checkbox", Name: "Save card for future"},
}

matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
testCases := []struct {
query    string
expected string
}{
{"add item to shopping cart", "e1"},
{"go to checkout", "e2"},
{"enter credit card number", "e3"},
{"expiration date field", "e4"},
{"security code input", "e5"},
{"finalize order", "e6"},
{"remember my payment info", "e7"},
}

for _, tc := range testCases {
result, err := matcher.Find(context.Background(), tc.query, checkoutElements, semantic.FindOptions{Threshold: 0.1, TopK: 1})
if err != nil {
t.Errorf("Query '%s': ERROR - %v", tc.query, err)
} else if result.BestRef != tc.expected {
t.Errorf("Query '%s': expected %s, got %s (score: %.3f)", tc.query, tc.expected, result.BestRef, result.BestScore)
}
}
}

func TestRealWorldAdminDashboard(t *testing.T) {
dashboardElements := []semantic.ElementDescriptor{
{Ref: "d1", Role: "button", Name: "Export CSV"},
{Ref: "d2", Role: "button", Name: "Export PDF"},
{Ref: "d3", Role: "combobox", Name: "Date Range"},
{Ref: "d4", Role: "table", Name: "User Metrics"},
{Ref: "d5", Role: "button", Name: "Refresh Data"},
{Ref: "d6", Role: "link", Name: "View Details"},
{Ref: "d7", Role: "button", Name: "Filter Results"},
}

matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
testCases := []struct {
query    string
expected string
}{
{"download spreadsheet", "d1"},
{"save as pdf document", "d2"},
{"change time period", "d3"},
{"show me the data table", "d4"},
{"update the numbers", "d5"},
{"see complete information", "d6"},
{"apply filtering", "d7"},
}

for _, tc := range testCases {
result, err := matcher.Find(context.Background(), tc.query, dashboardElements, semantic.FindOptions{Threshold: 0.1, TopK: 1})
if err != nil {
t.Errorf("Query '%s': ERROR - %v", tc.query, err)
} else if result.BestRef != tc.expected {
t.Errorf("Query '%s': expected %s, got %s (score: %.3f)", tc.query, tc.expected, result.BestRef, result.BestScore)
}
}
}

func TestRealWorldAccessibilityNavigation(t *testing.T) {
a11yElements := []semantic.ElementDescriptor{
{Ref: "a1", Role: "navigation", Name: "Main Menu"},
{Ref: "a2", Role: "link", Name: "Skip to Content"},
{Ref: "a3", Role: "button", Name: "Open Search"},
{Ref: "a4", Role: "button", Name: "Toggle Dark Mode"},
{Ref: "a5", Role: "link", Name: "Accessibility Settings"},
{Ref: "a6", Role: "button", Name: "Increase Font Size"},
}

matcher := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))
testCases := []struct {
query    string
expected string
}{
{"main navigation menu", "a1"},
{"jump to main content", "a2"},
{"start searching", "a3"},
{"switch to dark theme", "a4"},
{"configure screen reader options", "a5"},
{"make text bigger", "a6"},
}

for _, tc := range testCases {
result, err := matcher.Find(context.Background(), tc.query, a11yElements, semantic.FindOptions{Threshold: 0.1, TopK: 1})
if err != nil {
t.Errorf("Query '%s': ERROR - %v", tc.query, err)
} else if result.BestRef != tc.expected {
t.Errorf("Query '%s': expected %s, got %s (score: %.3f)", tc.query, tc.expected, result.BestRef, result.BestScore)
}
}
}
