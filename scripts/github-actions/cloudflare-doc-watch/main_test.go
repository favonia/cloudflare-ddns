package main

import "testing"

func TestSelectedWatchesRunPattern(t *testing.T) {
	t.Parallel()

	selected, err := selectedWatches("^Cloudflare IP ranges \\(IPv4\\)$")
	if err != nil {
		t.Fatalf("selectedWatches returned error: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("selectedWatches returned %d watches, want 1", len(selected))
	}
	if selected[0].Name != "Cloudflare IP ranges (IPv4)" {
		t.Fatalf("selected watch = %q, want %q", selected[0].Name, "Cloudflare IP ranges (IPv4)")
	}
}

func TestSelectedWatchesRejectsUnknownRunPattern(t *testing.T) {
	t.Parallel()

	_, err := selectedWatches("^does-not-exist$")
	if err == nil {
		t.Fatal("selectedWatches succeeded for an unknown run pattern")
	}
}

func TestParseOptionsRejectsPositionalArguments(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"config/example.json"})
	if err == nil {
		t.Fatal("parseOptions succeeded with positional arguments")
	}
}

func TestExtractKeySetItems(t *testing.T) {
	t.Parallel()

	document := `{"paths":{"/user/tokens/verify":{},"/accounts/{account_id}/tokens/verify":{},"/zones":{}}}`
	selectors := []keySetSelector{{
		Label:    "token-verify paths",
		Pointer:  "/paths",
		Pattern:  `/tokens/verify$`,
		Expected: []string{"/accounts/{account_id}/tokens/verify", "/user/tokens/verify"},
	}}

	actual, err := extractKeySetItems(document, selectors)
	if err != nil {
		t.Fatalf("extractKeySetItems returned error: %v", err)
	}
	want := "token-verify paths: [/accounts/{account_id}/tokens/verify /user/tokens/verify]"
	if len(actual) != 1 || actual[0] != want {
		t.Fatalf("extractKeySetItems = %v, want [%q]", actual, want)
	}
}

func TestExtractKeySetItemsRejectsNonObject(t *testing.T) {
	t.Parallel()

	document := `{"paths":[]}`
	selectors := []keySetSelector{{Label: "x", Pointer: "/paths", Pattern: `.`, Expected: nil}}
	if _, err := extractKeySetItems(document, selectors); err == nil {
		t.Fatal("extractKeySetItems succeeded on a non-object base pointer")
	}
}
