package external

import (
	"reflect"
	"testing"

	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/extract"
	"github.com/favonia/cloudflare-ddns/scripts/github-actions/link-check/internal/testutil"
)

func TestCollectURLsUsesDefaultShellTemplateExclusions(t *testing.T) {
	for _, target := range []string{
		"https://github.com/${GITHUB_REPOSITORY}/.github/workflows/build.yaml@${GITHUB_REF}",
		"https://github.com/leanprover/elan/releases/download/${ELAN_VERSION}/elan-x86_64-unknown-linux-gnu.tar.gz",
		"https://github.com/${_release2}/archive.tar.gz",
	} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			const file = ".github/workflows/example.yaml"
			testutil.WriteFile(t, root, file, "# Links\n# "+target+" https://github.com/leanprover/elan\n")
			cfg := defaultConfig()
			got := collectURLs(root, []string{file}, cfg.Links.TargetURLs.IgnoreExact, cfg.Links.TargetURLs.IgnorePatterns)
			want := []extract.ExternalLink{{
				URL:     "https://github.com/leanprover/elan",
				Sources: []extract.LinkSource{{Path: file, Line: 2}},
			}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("collected URLs = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCollectURLsKeepsURLsWithoutNamedShellPlaceholders(t *testing.T) {
	for _, target := range []string{
		"https://github.com/leanprover/elan/releases/download/v4.2.4/elan-x86_64-unknown-linux-gnu.tar.gz",
		"https://github.com/prices/$5",
		"https://github.com/%24%7BELAN_VERSION%7D",
		"https://github.com/${}",
		"https://github.com/${2NAME}",
		"https://github.com/${NAME",
		"https://github.com/${NAME:-default}",
	} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			const file = ".github/workflows/example.yaml"
			testutil.WriteFile(t, root, file, "# Links\n# "+target+"\n")
			cfg := defaultConfig()
			got := collectURLs(root, []string{file}, cfg.Links.TargetURLs.IgnoreExact, cfg.Links.TargetURLs.IgnorePatterns)
			want := []extract.ExternalLink{{
				URL:     target,
				Sources: []extract.LinkSource{{Path: file, Line: 2}},
			}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("collected URLs = %#v, want %#v", got, want)
			}
		})
	}
}
