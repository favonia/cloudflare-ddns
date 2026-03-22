package config

import (
	"regexp"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// The watched name-rule snapshot below was adopted on 2026-03-22. Update that
// date only when scripts/github-actions/cloudflare-doc-watch/config/waf-list-name-rules.json changes.
// Cloudflare documentation for that snapshot says:
//   - The name uses only lowercase letters, numbers, and the underscore (_) character in the name.
//     A valid name satisfies this regular expression: ^[a-z0-9_]+$.
var inverseWAFListNameRegex = regexp.MustCompile(`[^a-z0-9_]`)

// ReadWAFListNames reads an environment variable as a comma-separated list of IP list names.
//
// The quota snapshot below was adopted on 2026-03-22. Update that date only
// when scripts/github-actions/cloudflare-doc-watch/config/waf-list-availability.json changes.
// This intentionally stays a simple comma-separated input surface instead of a
// more structured format because the watched Cloudflare quota snapshot is still
// small for most accounts: Free allows 1 custom list, Pro/Business allow 10,
// and Enterprise allows 1,000. That quota snapshot is watched by
// scripts/github-actions/cloudflare-doc-watch/config/waf-list-availability.json.
func ReadWAFListNames(ppfmt pp.PP, key string, field *[]api.WAFList) bool {
	vals := GetenvAsList(key, ",")
	if len(vals) == 0 {
		return true
	}

	ppfmt.InfoOncef(pp.MessageExperimentalWAF, pp.EmojiHint,
		"You're using the experimental WAF list manipulation feature available since version 1.14.0")

	lists := make([]api.WAFList, 0, len(vals))

	for i, val := range vals {
		nthEntry := pp.Ordinal(i + 1)
		var list api.WAFList

		parts := strings.SplitN(val, "/", 2)
		if len(parts) != 2 {
			ppfmt.Noticef(pp.EmojiUserError,
				`The %s entry of %s (%q) should be in format "account-id/list-name"`,
				nthEntry, key, val)
			return false
		}
		list = api.WAFList{
			AccountID: api.ID(parts[0]),
			Name:      parts[1],
		}

		if violated := inverseWAFListNameRegex.FindString(list.Name); violated != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"The %s entry of %s has list name %q, which contains invalid character %q",
				nthEntry, key, list.Name, violated)
		}

		lists = append(lists, list)
	}

	*field = sliceutil.SortAndCompact(lists, api.CompareWAFList)
	return true
}
