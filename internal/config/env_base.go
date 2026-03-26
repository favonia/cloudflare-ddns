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

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// GetenvAsList reads an environment variable, split it by sep, and trim the space.
func GetenvAsList(key string, sep string) []string {
	vals := []string{}
	for v := range strings.SplitSeq(os.Getenv(key), sep) {
		v = strings.TrimSpace(v)
		if v != "" {
			vals = append(vals, v)
		}
	}
	return vals
}

// ReadString reads an environment variable as a plain string.
func ReadString(ppfmt pp.PP, key string, field *string) bool {
	val := Getenv(key)
	if val == "" {
		if *field != "" {
			ppfmt.Infof(pp.EmojiBullet, "Using default %s=%s", key, *field)
		}
		return true
	}

	*field = val
	return true
}

// ReadBool reads an environment variable as a boolean value.
func ReadBool(ppfmt pp.PP, key string, field *bool) bool {
	val := Getenv(key)
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

// ReadNonnegInt reads an environment variable as a non-negative integer.
func ReadNonnegInt(ppfmt pp.PP, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Using default %s=%d", key, *field)
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case i < 0:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%d) is negative", key, i)
		return false

	default:
		*field = i
		return true
	}
}

// prefixLenRange returns the valid prefix-length bounds for an IP family.
//
// The WAF IP-list prefix range snapshot below was adopted on 2026-03-24. Update
// that date only when
// scripts/github-actions/cloudflare-doc-watch/config/waf-list-ip-ranges.json
// changes. According to WAF documentation, the valid CIDR ranges are:
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

// ReadPrefixLen reads an environment variable as a prefix length for the given
// IP family. The valid range is derived from the family.
func ReadPrefixLen(ppfmt pp.PP, key string, field *int, ipFamily ipnet.Family) bool {
	val := Getenv(key)
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

// ReadTTL reads a valid TTL value.
//
// The TTL snapshot below was adopted on 2026-03-22. Update that date only when
// scripts/github-actions/cloudflare-doc-watch/config/dns-ttl.json changes.
// According to [API documentation], the valid range is 1 (auto) and [60, 86400].
// According to [DNS documentation], the valid range is "Auto" and [30, 86400].
// We thus accept the union of both ranges---1 (auto) and [30, 86400].
//
// [API documentation]: https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/
// [DNS documentation]: https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl
func ReadTTL(ppfmt pp.PP, key string, field *api.TTL) bool {
	val := Getenv(key)
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

// ReadNonnegDuration reads an environment variable and parses it as a time duration.
func ReadNonnegDuration(ppfmt pp.PP, key string, field *time.Duration) bool {
	val := Getenv(key)
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

// ReadCron reads an environment variable and parses it as a Cron expression.
func ReadCron(ppfmt pp.PP, key string, field *cron.Schedule) bool {
	switch val := Getenv(key); val {
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
