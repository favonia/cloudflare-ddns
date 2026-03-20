package config

import (
	"regexp"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// According to the Cloudflare documentation:
//   - The name uses only lowercase letters, numbers, and the underscore (_) character in the name.
//     A valid name satisfies this regular expression: ^[a-z0-9_]+$.
var inverseWAFListNameRegex = regexp.MustCompile(`[^a-z0-9_]`)

// ReadWAFListNames reads an environment variable as a comma-separated list of IP list names.
//
// Note: the Free plan can only have one list, and even the Enterprise
// plan can only have up to 10 custom lists at the time of writing
// (July 2024)! Hard to believe anyone would want to update more than
// one list in the same account.
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
