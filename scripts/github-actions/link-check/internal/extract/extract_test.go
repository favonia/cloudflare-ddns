package extract

import "testing"

func TestLocalTargetsCollectsExplicitHTMLIDs(t *testing.T) {
	targets := LocalTargets("docs/example.markdown", `<a id="alpha"></a><p id="beta">text</p>`)

	if !targets.SupportsFragments {
		t.Fatal("expected Markdown local targets to support fragment validation")
	}
	if !targets.Fragments["alpha"] {
		t.Fatal("expected explicit anchor id to be collected")
	}
	if !targets.Fragments["beta"] {
		t.Fatal("expected non-anchor HTML element id to be collected")
	}
}

func TestLocalTargetsIgnoresMarkdownHeadingSlugs(t *testing.T) {
	targets := LocalTargets("docs/example.markdown", "## Docker Compose Special Setups\n")

	if targets.Fragments["docker-compose-special-setups"] {
		t.Fatal("expected Markdown heading slug to be ignored")
	}
}

func TestLocalTargetsReportsUnsupportedFileTypes(t *testing.T) {
	targets := LocalTargets("docs/example.txt", "plain text")

	if targets.SupportsFragments {
		t.Fatal("expected plain text to report fragment validation as unsupported")
	}
	if len(targets.Fragments) != 0 {
		t.Fatalf("expected no fragments for unsupported file type, got %#v", targets.Fragments)
	}
}
