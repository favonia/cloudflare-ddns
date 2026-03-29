package config

import (
	"regexp"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// The watched name-rule snapshot below was adopted on 2026-03-22. Update that
// date only when the Cloudflare OpenAPI lists schema case in
// scripts/github-actions/cloudflare-doc-watch/cases.go changes.
// Cloudflare documentation for that snapshot says:
//   - The name uses only lowercase letters, numbers, and the underscore (_) character in the name.
//     A valid name satisfies this regular expression: ^[a-z0-9_]+$.
var inverseWAFListNameRegex = regexp.MustCompile(`[^a-z0-9_]`)

// ReadWAFListNames reads an environment variable as a comma-separated list of IP list names.
//
// The quota snapshot below was adopted on 2026-03-22. Update that date only
// when the Cloudflare WAF list availability case in
// scripts/github-actions/cloudflare-doc-watch/cases.go changes.
// This intentionally stays a simple comma-separated input surface instead of a
// more structured format because the watched Cloudflare quota snapshot is still
// small for most accounts: Free allows 1 custom list, Pro/Business allow 10,
// and Enterprise allows 1,000. That quota snapshot is watched by the
// Cloudflare WAF list availability case in
// scripts/github-actions/cloudflare-doc-watch/cases.go.
func ReadWAFListNames(ppfmt pp.PP, key string, field *[]api.WAFList) bool {
	vals := GetenvAsList(key, ",")
	if len(vals) == 1 && vals[0] == "" {
		return true
	}
	hasNonCanonicalEmptyEntry := false

	ppfmt.InfoOncef(pp.MessageExperimentalWAF, pp.EmojiHint,
		"You are using the experimental WAF list manipulation feature available since version 1.14.0")

	lists := make([]api.WAFList, 0, len(vals))

	for i, val := range vals {
		if val == "" {
			if i != len(vals)-1 {
				hasNonCanonicalEmptyEntry = true
			}
			continue
		}

		nthEntry := pp.Ordinal(i + 1)
		var list api.WAFList

		parts := strings.SplitN(val, "/", 2)
		if len(parts) != 2 {
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry of %s (%q) should be in the format "account-id/list-name"`,
				nthEntry, key, val)
			return false
		}
		list = api.WAFList{
			AccountID: api.ID(parts[0]),
			Name:      parts[1],
		}

		if violated := inverseWAFListNameRegex.FindString(list.Name); violated != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"The %s entry of %s has the list name %q, which contains an invalid character %q",
				nthEntry, key, list.Name, violated)
		}

		lists = append(lists, list)
	}

	if hasNonCanonicalEmptyEntry {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) contains extra commas; "+
				"this is accepted for now but will be rejected in version 2.0.0",
			key, pp.QuotePreviewOrEmptyLabel(Getenv(key), pp.AdvisoryPreviewLimit, "empty"))
	}

	*field = sliceutil.SortAndCompact(lists, api.CompareWAFList)
	return true
}
