package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// GetenvAsString reads an environment variable as a string.
func GetenvAsString(key string, def string, quiet quiet.Quiet) (string, bool) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("🈳 Use default %s=%q", key, def)
		}
		return def, true
	}

	return val, true
}

// GetenvAsBool reads an environment variable as a boolean value.
func GetenvAsBool(key string, def bool, quiet quiet.Quiet) (bool, bool) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("🈳 Use default %s=%t", key, def)
		}
		return def, true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		log.Printf("😡 Failed to parse %s: %v", key, err)
		return b, false
	}

	return b, true
}

// GetenvAsQuiet reads an environment variable as quiet/verbose.
func GetenvAsQuiet(key string) (quiet.Quiet, bool) {
	def := quiet.Default

	val := Getenv(key)
	if val == "" {
		log.Printf("🈳 Use default %s=%t", key, def)
		return def, true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		log.Printf("😡 Failed to parse %s: %v", key, err)
		return def, false
	}

	return quiet.Quiet(b), true
}

// GetenvAsInt reads an environment variable as an integer.
func GetenvAsInt(key string, def int, quiet quiet.Quiet) (int, bool) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("🈳 Use default %s=%d", key, def)
		}
		return def, true
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("😡 Failed to parse %s: %v", key, err)
		return 0, false
	}

	return i, true
}

// GetenvAsNormalizedDomains reads an environment variable as a comma-separated list of domains.
// Spaces are trimed.
func GetenvAsNormalizedDomains(key string, quiet quiet.Quiet) []string {
	rawList := strings.Split(Getenv(key), ",")

	list := make([]string, 0, len(rawList))
	for _, item := range rawList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		list = append(list, normalizeDomain(item))
	}

	return list
}

// GetenvAsPolicy reads an environment variable and parses it as a policy.
func GetenvAsPolicy(ipNet ipnet.Type, key string, def detector.Policy, quiet quiet.Quiet) (detector.Policy, bool) {
	switch val := Getenv(key); val {
	case "":
		if !quiet {
			log.Printf("🈳 Use default %s=%v", key, def)
		}
		return def, true
	case "cloudflare":
		return &detector.Cloudflare{Net: ipNet}, true
	case "ipify":
		return &detector.Ipify{Net: ipNet}, true
	case "local":
		return &detector.Local{Net: ipNet}, true
	case "unmanaged":
		return &detector.Unmanaged{}, true
	default:
		log.Printf("😡 Failed to parse %s: %q is not a valid policy.", key, val)
		return nil, false
	}
}

// GetenvAsPosDuration reads an environment variable and parses it as a time duration.
func GetenvAsPosDuration(key string, def time.Duration, quiet quiet.Quiet) (time.Duration, bool) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("🈳 Use default %s=%s", key, def.String())
		}
		return def, true
	}

	t, err := time.ParseDuration(val)

	switch {
	case err != nil:
		log.Printf("😡 Failed to parse %s: %v", key, err)
		return 0, false
	case t < 0:
		log.Printf("😡 Failed to parse %s: %v is negative.", key, t)
	}

	return t, true
}

// GetenvAsCron reads an environment variable and parses it as a Cron expression.
func GetenvAsCron(key string, def cron.Schedule, quiet quiet.Quiet) (cron.Schedule, bool) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("🈳 Use default %s=%v", key, def)
		}
		return def, true
	}

	c, err := cron.New(val)
	if err != nil {
		log.Printf("😡 Failed to parse %s: %v", key, err)
		return c, false
	}

	return c, true
}
