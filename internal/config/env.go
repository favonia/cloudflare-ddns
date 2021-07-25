package config

import (
	"fmt"
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

// ReadQuiet reads an environment variable as quiet/verbose.
func ReadQuiet(key string, field *quiet.Quiet) bool {
	val := Getenv(key)
	if val == "" {
		fmt.Printf("🈳 Use default %s=%t\n", key, *field)
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		fmt.Printf("😡 Failed to parse %s: %v\n", key, err)
		return false
	}

	*field = quiet.Quiet(b)
	return true
}

// ReadString reads an environment variable as a string.
func ReadString(quiet quiet.Quiet, key string, field *string) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			fmt.Printf("🈳 Use default %s=%q\n", key, *field)
		}
		return true
	}

	*field = val
	return true
}

// ReadBool reads an environment variable as a boolean value.
func ReadBool(quiet quiet.Quiet, key string, field *bool) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			fmt.Printf("🈳 Use default %s=%t\n", key, *field)
		}
		return true
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		fmt.Printf("😡 Failed to parse %s: %v\n", key, err)
		return false
	}

	*field = b
	return true
}

// ReadNonnegInt reads an environment variable as an integer.
func ReadNonnegInt(quiet quiet.Quiet, key string, field *int) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			fmt.Printf("🈳 Use default %s=%d\n", key, *field)
		}
		return true
	}

	i, err := strconv.Atoi(val)
	switch {
	case err != nil:
		fmt.Printf("😡 Failed to parse %s: %v\n", key, err)
		return false
	case i < 0:
		fmt.Printf("😡 Failed to parse %s: %v is negative.\n", key, i)
	}

	*field = i
	return true
}

// ReadDomains reads an environment variable as a comma-separated list of domains.
// Spaces are trimed.
func ReadDomains(_ quiet.Quiet, key string, field *[]string) bool {
	rawList := strings.Split(Getenv(key), ",")

	*field = make([]string, 0, len(rawList))
	for _, item := range rawList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		*field = append(*field, normalizeDomain(item))
	}

	return true
}

// ReadPolicy reads an environment variable and parses it as a policy.
func ReadPolicy(quiet quiet.Quiet, ipNet ipnet.Type, key string, field *detector.Policy) bool {
	switch val := Getenv(key); val {
	case "":
		if !quiet {
			fmt.Printf("🈳 Use default %s=%v\n", key, *field)
		}
		return true
	case "cloudflare":
		*field = &detector.Cloudflare{Net: ipNet}
		return true
	case "ipify":
		*field = &detector.Ipify{Net: ipNet}
		return true
	case "local":
		*field = &detector.Local{Net: ipNet}
		return true
	case "unmanaged":
		*field = &detector.Unmanaged{}
		return true
	default:
		fmt.Printf("😡 Failed to parse %s: %q is not a valid policy.\n", key, val)
		return false
	}
}

// ReadNonnegDuration reads an environment variable and parses it as a time duration.
func ReadNonnegDuration(quiet quiet.Quiet, key string, field *time.Duration) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			fmt.Printf("🈳 Use default %s=%v\n", key, *field)
		}
		return true
	}

	t, err := time.ParseDuration(val)

	switch {
	case err != nil:
		fmt.Printf("😡 Failed to parse %s: %v\n", key, err)
		return false
	case t < 0:
		fmt.Printf("😡 Failed to parse %s: %v is negative.\n", key, t)
	}

	*field = t
	return true
}

// ReadCron reads an environment variable and parses it as a Cron expression.
func ReadCron(quiet quiet.Quiet, key string, field *cron.Schedule) bool {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			fmt.Printf("🈳 Use default %s=%v\n", key, *field)
		}
		return true
	}

	c, err := cron.New(val)
	if err != nil {
		fmt.Printf("😡 Failed to parse %s: %v\n", key, err)
		return false
	}

	*field = c
	return true
}
