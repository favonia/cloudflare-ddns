package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/detector"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/quiet"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// ReadQuiet reads an environment variable as quiet/verbose.
func ReadQuiet(indent pp.Indent, key string, field *quiet.Quiet) bool {
	val := Getenv(key)
	if val == "" {
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	*field = quiet.Quiet(b)
	return true
}

// ReadString reads an environment variable as a string.
func ReadString(quiet quiet.Quiet, indent pp.Indent, key string, field *string) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%s", key, *field)
		}
		return true
	}

	*field = val
	return true
}

// ReadBool reads an environment variable as a boolean value.
func ReadBool(quiet quiet.Quiet, indent pp.Indent, key string, field *bool) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%t", key, *field)
		}
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	*field = b
	return true
}

// ReadNonnegInt reads an environment variable as an integer.
func ReadNonnegInt(quiet quiet.Quiet, indent pp.Indent, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%d", key, *field)
		}
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	case i < 0:
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %d is negative.", val, i)
		return false
	}

	*field = i
	return true
}

// ReadDomains reads an environment variable as a comma-separated list of domains.
// Spaces are trimed.
func ReadDomains(_ quiet.Quiet, indent pp.Indent, key string, field *[]api.FQDN) bool {
	rawList := strings.Split(Getenv(key), ",")

	*field = make([]api.FQDN, 0, len(rawList))
	for _, item := range rawList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		item, err := api.NewFQDN(item)
		if err != nil {
			pp.Printf(indent, pp.EmojiUserError, "Domain %q was added but it is ill-formed: %v", item, err)
		}

		*field = append(*field, item)
	}

	return true
}

// ReadPolicy reads an environment variable and parses it as a policy.
func ReadPolicy(quiet quiet.Quiet, indent pp.Indent, key string, field *detector.Policy) bool {
	switch val := Getenv(key); val {
	case "":
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%s", key, *field)
		}
		return true
	case "cloudflare":
		*field = &detector.Cloudflare{}
		return true
	case "ipify":
		*field = &detector.Ipify{}
		return true
	case "local":
		*field = &detector.Local{}
		return true
	case "unmanaged":
		*field = &detector.Unmanaged{}
		return true
	default:
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: not a valid policy.", val)
		return false
	}
}

// ReadNonnegDuration reads an environment variable and parses it as a time duration.
func ReadNonnegDuration(quiet quiet.Quiet, indent pp.Indent, key string, field *time.Duration) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%s", key, field)
		}
		return true
	}

	t, err := time.ParseDuration(val)

	switch {
	case err != nil:
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	case t < 0:
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v is negative.", val, t)
	}

	*field = t
	return true
}

// ReadCron reads an environment variable and parses it as a Cron expression.
func ReadCron(quiet quiet.Quiet, indent pp.Indent, key string, field *cron.Schedule) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			pp.Printf(indent, pp.EmojiBullet, "Use default %s=%s", key, *field)
		}
		return true
	}

	c, err := cron.New(val)
	if err != nil {
		pp.Printf(indent, pp.EmojiUserError, "Failed to parse %q: %v", val, err)
		return false
	}

	*field = c
	return true
}
