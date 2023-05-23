package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// ReadEmoji reads an environment variable as a plain string.
func ReadString(ppfmt pp.PP, key string, field *string) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, *field)
		return true
	}

	*field = val
	return true
}

// ReadEmoji reads an environment variable as emoji/no-emoji.
func ReadEmoji(key string, ppfmt *pp.PP) bool {
	valEmoji := Getenv(key)
	if valEmoji == "" {
		return true
	}

	emoji, err := strconv.ParseBool(valEmoji)
	if err != nil {
		(*ppfmt).Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valEmoji, err)
		return false
	}

	*ppfmt = (*ppfmt).SetEmoji(emoji)

	return true
}

// ReadQuiet reads an environment variable as quiet/verbose.
func ReadQuiet(key string, ppfmt *pp.PP) bool {
	valQuiet := Getenv(key)
	if valQuiet == "" {
		return true
	}

	quiet, err := strconv.ParseBool(valQuiet)
	if err != nil {
		(*ppfmt).Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valQuiet, err)
		return false
	}

	if quiet {
		*ppfmt = (*ppfmt).SetLevel(pp.Quiet)
	} else {
		*ppfmt = (*ppfmt).SetLevel(pp.Verbose)
	}

	return true
}

// ReadBool reads an environment variable as a boolean value.
func ReadBool(ppfmt pp.PP, key string, field *bool) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%t", key, *field)
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, val, err)
		return false
	}

	*field = b
	return true
}

// ReadLinuxID reads an environment variable as a user or group ID.
func ReadLinuxID(ppfmt pp.PP, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%d", key, *field)
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case i < 0:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%d) is negative", key, i)
		return false

	case i == 0:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%d) cannot be zero (the superuser)", key, i)
		return false

	default:
		*field = i
		return true
	}
}

// ReadNonnegInt reads an environment variable as a non-negative integer.
func ReadNonnegInt(ppfmt pp.PP, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%d", key, *field)
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case i < 0:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%d) is negative", key, i)
		return false

	default:
		*field = i
		return true
	}
}

// ReadTTL reads a valid TTL value.
//
// According to [API documentation], the valid range is 1 (auto) and [60, 86400].
// According to [DNS documentation], the valid range is "Auto" and [30, 86400].
// We thus accept the union of both ranges---1 (auto) and [30, 86400].
//
// [API documentation]: https://api.cloudflare.com/#dns-records-for-a-zone-create-dns-record
// [DNS documentation]: https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl
func ReadTTL(ppfmt pp.PP, key string, field *api.TTL) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%d", key, *field)
		return true
	}

	res, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a number: %v", key, val, err)
		return false

	case res != 1 && (res < 30 || res > 86400):
		ppfmt.Errorf(pp.EmojiUserError, "%s (%d) should be 1 (auto) or between 30 and 86400", key, res)
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
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%v", key, *field)
		return true
	}

	t, err := time.ParseDuration(val)

	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a time duration: %v", key, val, err)
		return false
	case t < 0:
		ppfmt.Errorf(pp.EmojiUserError, "%s (%v) is negative", key, t)
		return false
	}

	*field = t
	return true
}

// ReadCron reads an environment variable and parses it as a Cron expression.
func ReadCron(ppfmt pp.PP, key string, field *cron.Schedule) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, cron.DescribeSchedule(*field))
		return true
	}

	c, err := cron.New(val)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a cron expression: %v", key, val, err)
		return false
	}

	*field = c
	return true
}
