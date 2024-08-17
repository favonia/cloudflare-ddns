package config

import (
	"regexp"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// According to the Cloudflare documentation:
//   - The maximum length of a list name is 50 characters.
//
// Note that all valid characters in a name are ASCII printable, so we can count the characters
// by the number of bytes without worrying about multi-byte characters.
const wafListNameMaxLength int = 50

// According to the Cloudflare documentation:
//   - The name uses only lowercase letters, numbers, and the underscore (_) character in the name.
//     A valid name satisfies this regular expression: ^[a-z0-9_]+$.
var (
	wafListNameRegex        = regexp.MustCompile(`^[a-z0-9_]+$`)
	inverseWAFListNameRegex = regexp.MustCompile(`[^a-z0-9_]`)
)

// ReadAndAppendWAFListNames reads an environment variable as
// a comma-separated list of IP list names and append the list
// to the field.
//
// Note: the Free plan can only have one list, and even the Enterprise
// plan can only have up to 10 custom lists at the time of writing
// (July 2024)! Hard to believe anyone would want to update more than
// one list in the same account.
func ReadAndAppendWAFListNames(ppfmt pp.PP, key string, field *[]api.WAFList) bool {
	vals := GetenvAsList(key, ",")
	if len(vals) == 0 {
		return true
	}

	lists := make([]api.WAFList, 0, len(vals))

	for _, val := range vals {
		var list api.WAFList

		parts := strings.SplitN(val, "/", 2) //nolint:mnd
		if len(parts) != 2 {                 //nolint:mnd
			ppfmt.Errorf(pp.EmojiUserError, `List %q should be in format "account-id/list-name"`, val)
			return false
		}
		list.AccountID, list.ListName = api.ID(parts[0]), parts[1]

		if !wafListNameRegex.MatchString(list.ListName) {
			ppfmt.Errorf(pp.EmojiUserError, "List name %q contains invalid character %q",
				list.ListName, inverseWAFListNameRegex.FindString(list.ListName))
			return false
		}
		if len(val) > wafListNameMaxLength {
			ppfmt.Errorf(pp.EmojiUserError, `List name "%.10s..." is too long (more than 50 characters)`, list.ListName)
			return false
		}

		lists = append(lists, list)
	}

	// Append all the names after checking them
	*field = append(*field, lists...)
	return true
}
