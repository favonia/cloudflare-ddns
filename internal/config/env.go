package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// GetenvAsString reads an environment variable as a string.
func GetenvAsString(key string, def string, quiet quiet.Quiet) (string, error) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %q", key, def)
		}
		return def, nil
	}

	return val, nil
}

// GetenvAsBool reads an environment variable as a boolean value.
func GetenvAsBool(key string, def bool, quiet quiet.Quiet) (bool, error) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %t", key, def)
		}
		return def, nil
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		return b, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
	}

	return b, nil
}

// GetenvAsQuiet reads an environment variable as quiet/verbose.
func GetenvAsQuiet(key string) (quiet.Quiet, error) {
	def := quiet.VERBOSE

	val := Getenv(key)
	if val == "" {
		log.Printf("📭 The variable %s is empty or unset. Default value: %t", key, def)
		return def, nil
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		return def, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
	}

	return quiet.Quiet(b), nil
}

// GetenvAsInt reads an environment variable as an integer.
func GetenvAsInt(key string, def int, quiet quiet.Quiet) (int, error) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %d", key, def)
		}
		return def, nil
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return i, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
	}

	return i, nil
}

// GetenvAsNormalizedDomains reads an environment variable as a comma-separated list of domains.
// Spaces are trimed.
func GetenvAsNormalizedDomains(key string, quiet quiet.Quiet) []string {
	val := Getenv(key)
	rawList := strings.Split(val, ",")
	var list []string
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
func GetenvAsPolicy(key string, def detector.Policy, quiet quiet.Quiet) (detector.Policy, error) {
	switch val := Getenv(key); val {
	case "":
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %v", key, def)
		}
		return def, nil
	case "cloudflare":
		return &detector.Cloudflare{}, nil
	case "ipify":
		return &detector.Ipify{}, nil
	case "local":
		return &detector.Local{}, nil
	case "unmanaged":
		return &detector.Unmanaged{}, nil
	default:
		return &detector.Unmanaged{}, fmt.Errorf("😡 Error parsing the variable %s with the value %s", key, val)
	}
}

// GetenvAsPosDuration reads an environment variable and parses it as a time duration
func GetenvAsPosDuration(key string, def time.Duration, quiet quiet.Quiet) (time.Duration, error) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %s", key, def.String())
		}
		return def, nil
	}

	t, err := time.ParseDuration(val)
	if err != nil || t <= 0 {
		return t, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
	}

	return t, err
}

// GetenvAsCron reads an environment variable and parses it as a Cron expression
func GetenvAsCron(key string, def cron.Schedule, quiet quiet.Quiet) (cron.Schedule, error) {
	val := Getenv(key)
	if val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %s", key, def.String())
		}
		return def, nil
	}

	c, err := cron.New(val)
	if err != nil {
		return c, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
	}

	return c, err
}
