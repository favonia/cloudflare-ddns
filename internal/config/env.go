package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func ReadString(ppfmt pp.PP, key string, field *string) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, *field)
		return true
	}

	*field = val
	return true
}

// ReadQuiet reads an environment variable as quiet/verbose.
func ReadQuiet(key string, ppfmt *pp.PP) bool {
	val := Getenv(key)
	if val == "" {
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		(*ppfmt).Errorf(pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	if b {
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
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	*field = b
	return true
}

// ReadNonnegInt reads an environment variable as an integer.
func ReadNonnegInt(ppfmt pp.PP, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%d", key, *field)
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	case i < 0:
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %d is negative", val, i)
		return false
	}

	*field = i
	return true
}

// ReadTTL reads a valid TTL value.
//
// According to [API documentation], the valid range is 1 (auto) and [60, 86400].
// According to [DNS documentation], the valid range is "Auto" and [30, 86400].
// We thus accept the union of both ranges---1 (auto) and [30, 86400].
//
// [API documentation] https://api.cloudflare.com/#dns-records-for-a-zone-create-dns-record
// [DNS documentation] https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl
func ReadTTL(ppfmt pp.PP, key string, field *api.TTL) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%d", key, *field)
		return true
	}

	res, err := strconv.Atoi(val)
	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "TTL (%q) is not a number: %v", val, err)
		return false

	case res != 1 && (res < 30 || res > 86400):
		ppfmt.Errorf(pp.EmojiUserError, "TTL (%d) should be 1 (auto) or between 30 and 86400", res)
		return false
	}

	*field = api.TTL(res)
	return true
}

// ReadDomains reads an environment variable as a comma-separated list of domains.
func ReadDomains(ppfmt pp.PP, key string, field *[]domain.Domain) bool {
	if list, ok := domainexp.ParseList(ppfmt, Getenv(key)); ok {
		*field = list
		return true
	}
	return false
}

// ReadProvider reads an environment variable and parses it as a provider.
//
// policyKey was the name of the deprecated parameters IP4/6_POLICY.
//
//nolint:funlen
func ReadProvider(ppfmt pp.PP, key, keyDeprecated string, field *provider.Provider) bool {
	if val := Getenv(key); val == "" {
		// parsing of the deprecated parameter
		switch valPolicy := Getenv(keyDeprecated); valPolicy {
		case "":
			ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, provider.Name(*field))
			return true
		case "cloudflare":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s and provider "cloudflare" were deprecated; use %s=cloudflare.doh or %s=cloudflare.trace`,
				keyDeprecated, key, key,
			)
			*field = provider.NewCloudflareTrace()
			return true
		case "cloudflare.trace":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s was deprecated; use %s=%s`,
				keyDeprecated, key, valPolicy,
			)
			*field = provider.NewCloudflareTrace()
			return true
		case "cloudflare.doh":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s was deprecated; use %s=%s`,
				keyDeprecated, key, valPolicy,
			)
			*field = provider.NewCloudflareDOH()
			return true
		case "ipify":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s was deprecated; use %s=%s`,
				keyDeprecated, key, valPolicy,
			)
			*field = provider.NewIpify()
			return true
		case "local":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s was deprecated; use %s=%s`,
				keyDeprecated, key, valPolicy,
			)
			*field = provider.NewLocal()
			return true
		case "unmanaged":
			ppfmt.Warningf(
				pp.EmojiUserWarning,
				`Parameter %s was deprecated; use %s=none`,
				keyDeprecated, key,
			)
			*field = nil
			return true
		default:
			ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid provider", valPolicy)
			return false
		}
	} else {
		if Getenv(keyDeprecated) != "" {
			ppfmt.Errorf(
				pp.EmojiUserError,
				`Cannot have both %s and %s set`,
				key, keyDeprecated,
			)
			return false
		}

		switch val {
		case "cloudflare":
			ppfmt.Errorf(
				pp.EmojiUserError,
				`Parameter %s does not accept "cloudflare"; use "cloudflare.doh" or "cloudflare.trace"`,
				key, key,
			)
			return false
		case "cloudflare.trace":
			*field = provider.NewCloudflareTrace()
			return true
		case "cloudflare.doh":
			*field = provider.NewCloudflareDOH()
			return true
		case "ipify":
			*field = provider.NewIpify()
			return true
		case "local":
			*field = provider.NewLocal()
			return true
		case "none":
			*field = nil
			return true
		default:
			ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: not a valid provider", val)
			return false
		}
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
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	case t < 0:
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %v is negative", val, t)
		return false
	}

	*field = t
	return true
}

// ReadCron reads an environment variable and parses it as a Cron expression.
func ReadCron(ppfmt pp.PP, key string, field *cron.Schedule) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%v", key, *field)
		return true
	}

	c, err := cron.New(val)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	*field = c
	return true
}

// ReadHealthChecksURL reads the base URL of the healthcheck.io endpoint.
func ReadHealthChecksURL(ppfmt pp.PP, key string, field *[]monitor.Monitor) bool {
	val := Getenv(key)

	if val == "" {
		return true
	}

	h, ok := monitor.NewHealthChecks(ppfmt, val)
	if !ok {
		return false
	}

	*field = append(*field, h)
	return true
}
