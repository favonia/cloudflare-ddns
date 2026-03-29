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
