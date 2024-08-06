package config

import (
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// According to the Cloudflare docs:
//   - The name uses only lowercase letters, numbers, and the underscore (_) character in the name.
//     A valid name satisfies this regular expression: ^[a-z0-9_]+$.
//   - The maximum length of a list name is 50 characters.
const wafListNameMaxLength int = 50

var (
	wafListNameRegex        = regexp.MustCompile(`^[a-z0-9_]+$`)
	inverseWAFListNameRegex = regexp.MustCompile(`[^a-z0-9_]`)
)

// ReadAndAppendWAFListNames reads an environment variable as
// a comma-separated list of IP list names and append the list
// to the field.
func ReadAndAppendWAFListNames(ppfmt pp.PP, key string, field *[]string) bool {
	vals := GetenvAsList(key, ",")
	if len(vals) == 0 {
		return true
	}

	// The following messages assume the user only put a single name,
	// which should be the case. The Free plan can only have one list,
	// and even the Enterprise plan can only have up to 10 custom lists
	// at the time of writing (Jul 2024)! Hard to believe anyone would
	// want the tool to update more than one list.
	for _, val := range vals {
		if !wafListNameRegex.MatchString(val) {
			ppfmt.Errorf(pp.EmojiUserError, "%s=%s contains invalid character %q",
				key, val, inverseWAFListNameRegex.FindString(val))
			return false
		}
		if len(val) > wafListNameMaxLength {
			ppfmt.Errorf(pp.EmojiUserError, "%s is too long (more than 50 characters in a name)", key)
			return false
		}
	}

	// Append all the names after checking them
	*field = append(*field, vals...)
	return true
}
