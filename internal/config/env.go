package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

// Getenv reads an environment variable and trim the space.
func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
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

// GetenvAsNonEmptyList reads an environment variable as a comma-separated list of strings.
// Spaces are trimed.
func GetenvAsNonEmptyList(key string, quiet quiet.Quiet) ([]string, error) {
	val := Getenv(key)
	if val == "" {
		return nil, fmt.Errorf("😡 The variable %s is empty or unset.", key)
	}

	list := strings.Split(val, ",")
	for i := range list {
		list[i] = strings.TrimSpace(list[i])
	}
	return list, nil
}

// GetenvAsPolicy reads an environment variable and parses it as a policy.
func GetenvAsPolicy(key string, quiet quiet.Quiet) (detector.Policy, error) {
	switch val := Getenv(key); val {
	case "":
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: cloudflare", key)
		}
		return &detector.Cloudflare{}, nil
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

// GetenvAsPolicy reads an environment variable and parses it as a time duration
func GetenvAsPositiveTimeDuration(key string, def time.Duration, quiet quiet.Quiet) (time.Duration, error) {
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
