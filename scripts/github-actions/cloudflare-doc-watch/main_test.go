package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

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

func TestExtractMarkdownSectionLinesStopsAtConfiguredHeading(t *testing.T) {
	t.Parallel()

	document := `# Watched section

## First item

First details.

## Second item

Second details.

## Stop here

Outside the watched section.
`
	actual, err := extractMarkdownSectionLines(
		document,
		"# Watched section",
		"## Stop here",
		[]string{"^## "},
	)
	if err != nil {
		t.Fatalf("extractMarkdownSectionLines returned error: %v", err)
	}
	want := []string{"## First item", "## Second item"}
	if len(actual) != len(want) {
		t.Fatalf("extractMarkdownSectionLines = %v, want %v", actual, want)
	}
	for index := range want {
		if actual[index] != want[index] {
			t.Fatalf("extractMarkdownSectionLines = %v, want %v", actual, want)
		}
	}
}

// The fixtures capture the published index.md pages on 2026-09-18, before
// normalization. They reproduce renderer changes independently of our baselines.
func TestPublishedMarkdownWatches(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name      string
		fixture   string
		mutations [][2]string
	}{
		{
			name:    "Cloudflare DNS record attributes",
			fixture: "record-attributes",
			mutations: [][2]string{
				{"| Character limit | 100 |", "| Character limit | 200 |"},
				{"case-sensitive", "case-insensitive"},
				{"Graphic_character", "Different_character"},
			},
		},
		{
			name:    "Cloudflare WAF list availability",
			fixture: "lists",
			mutations: [][2]string{
				{"| Number of custom lists (any type) | 1 |", "| Number of custom lists (any type) | 2 |"},
				{"highest quota.", "lowest quota."},
				{"Notes:", "Notes:\n\n- A newly documented quota restriction."},
			},
		},
		{
			name:    "Cloudflare WAF list name rules",
			fixture: "lists",
			mutations: [][2]string{
				{"50 characters", "100 characters"},
				{"lowercase letters", "uppercase letters"},
				{"## List names", "## List names\n\n- A newly documented naming restriction."},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("testdata/" + testCase.fixture + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			document := string(data)
			watches, err := selectedWatches("^" + testCase.name + "$")
			if err != nil {
				t.Fatal(err)
			}
			if len(watches) != 1 {
				t.Fatalf("selected %d watches, want 1", len(watches))
			}
			cfg := watches[0]
			checkPublishedMarkdown(t, cfg, document)
			for _, mutation := range testCase.mutations {
				t.Run(mutation[0], func(t *testing.T) {
					t.Parallel()
					if !strings.Contains(document, mutation[0]) {
						t.Fatalf("fixture lacks %q", mutation[0])
					}
					changed := strings.Replace(document, mutation[0], mutation[1], 1)
					actual, err := extractMarkdownSectionLines(changed, cfg.WatchedHeading, cfg.StopHeading, cfg.LineFilters)
					if err == nil && slices.Equal(actual, cfg.ExpectedLines) {
						t.Fatalf("watch missed change from %q to %q", mutation[0], mutation[1])
					}
				})
			}
		})
	}
}

func TestNormalizeMarkdownLinePreservesContent(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"-1", "---", "**bold**", "`- literal`", "1. Ordered rule",
		"* Keep %5F and ( `_`) unchanged.",
	} {
		if actual := normalizeMarkdownLine(line); actual != line {
			t.Errorf("normalizeMarkdownLine(%q) = %q", line, actual)
		}
	}
}

func checkPublishedMarkdown(t *testing.T, cfg config, document string) {
	t.Helper()
	for _, marker := range []string{"-", "*", "+"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			rendered := strings.ReplaceAll(document, "\n- ", "\n"+marker+" ")
			actual, err := extractMarkdownSectionLines(rendered, cfg.WatchedHeading, cfg.StopHeading, cfg.LineFilters)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(actual, cfg.ExpectedLines) {
				t.Fatalf("published content differs: got %q, want %q", actual, cfg.ExpectedLines)
			}
		})
	}
}
