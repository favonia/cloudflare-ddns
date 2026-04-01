package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// getenv reads an environment variable and trim the space.
func getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// getenvAsList reads an environment variable, splits it by sep, and trims each item.
// Empty trimmed entries are preserved. Callers remain responsible for assigning any
// higher-level meaning to empty items, including comma-placement policy.
func getenvAsList(key string, sep string) []string {
	vals := []string{}
	for v := range strings.SplitSeq(os.Getenv(key), sep) {
		v = strings.TrimSpace(v)
		vals = append(vals, v)
	}
	return vals
}

// readString reads an environment variable as a plain string.
//
//nolint:unparam // Keep the read* helper signature uniform for ReadEnv.
func readString(ppfmt pp.PP, key string, field *string) bool {
	val := getenv(key)
	if val == "" {
		if *field != "" {
			ppfmt.Infof(pp.EmojiBullet, "Using default %s=%s", key, *field)
		}
		return true
	}

	*field = val
	return true
}

// readBool reads an environment variable as a boolean value.
func readBool(ppfmt pp.PP, key string, field *bool) bool {
	val := getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%t", key, *field)
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, val, err)
		return false
	}

	*field = b
	return true
}

// prefixLenRange returns the valid prefix-length bounds for an IP family.
//
// The WAF IP-list prefix range snapshot below was adopted on 2026-03-24. Update
// that date only when the Cloudflare WAF IP list item format case in
// scripts/github-actions/cloudflare-doc-watch/cases.go changes. According to
// WAF documentation, the valid CIDR ranges are:
//   - IPv4: /8 through /32
//   - IPv6: /12 through /128
//
// [WAF documentation]: https://developers.cloudflare.com/waf/tools/lists/lists-api/json-object
func prefixLenRange(ipFamily ipnet.Family) (int, int) {
	switch ipFamily {
	case ipnet.IP4:
		return 8, 32
	case ipnet.IP6:
		return 12, 128
	default:
		return 0, 0
	}
}

// readPrefixLen reads an environment variable as a prefix length for the given
// IP family. The valid range is derived from the family.
func readPrefixLen(ppfmt pp.PP, key string, field *int, ipFamily ipnet.Family) bool {
	val := getenv(key)
	lo, hi := prefixLenRange(ipFamily)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%d", key, *field)
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case i < lo || i > hi:
		ppfmt.Noticef(pp.EmojiUserError,
			"%s (%d) is not within the range %d-%d for %s",
			key, i, lo, hi, ipFamily.Describe())
		return false

	default:
		*field = i
		return true
	}
}

// readTTL reads a valid TTL value.
//
// The TTL snapshot below was adopted on 2026-03-22. Update that date only when
// the Cloudflare DNS TTL semantics case in
// scripts/github-actions/cloudflare-doc-watch/cases.go changes.
// According to the current [API documentation] and [DNS documentation], TTL
// uses 1 for "automatic" and otherwise accepts 30 through 86400 seconds, with
// the 30-second minimum limited to Enterprise zones and 60 seconds applying to
// non-Enterprise zones. We accept the documented union here: 1 (automatic) or
// 30 through 86400.
//
// [API documentation]: https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/
// [DNS documentation]: https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl
func readTTL(ppfmt pp.PP, key string, field *api.TTL) bool {
	val := getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%d", key, *field)
		return true
	}

	res, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case res != 1 && (res < 30 || res > 86400):
		ppfmt.Noticef(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, res)
		return false

	default:
		*field = api.TTL(res)
		return true
	}
}

// readNonnegDuration reads an environment variable and parses it as a time duration.
func readNonnegDuration(ppfmt pp.PP, key string, field *time.Duration) bool {
	val := getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%v", key, *field)
		return true
	}

	t, err := time.ParseDuration(val)

	switch {
	case err != nil:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a time duration: %v", key, val, err)
		return false
	case t < 0:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%v) is negative", key, t)
		return false
	}

	*field = t
	return true
}

// readCron reads an environment variable and parses it as a Cron expression.
func readCron(ppfmt pp.PP, key string, field *cron.Schedule) bool {
	switch val := getenv(key); val {
	case "":
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%s", key, cron.DescribeSchedule(*field))
		return true

	case "@once":
		*field = nil
		return true

	case "@disabled", "@nevermore":
		ppfmt.Noticef(pp.EmojiUserWarning, "%s=%s is deprecated; use %s=@once", key, val, key)
		*field = nil
		return true

	default:
		c, err := cron.New(val)
		if err != nil {
			ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a cron expression: %v", key, val, err)
			return false
		}
		*field = c
		return true
	}
}
