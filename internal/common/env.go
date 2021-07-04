package common

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func GetenvAsPolicy(key string, quiet Quiet) (Policy, error) {
	val := strings.TrimSpace(os.Getenv(key))
	switch val {
	case "":
		log.Printf("📭 The variable %s is empty or unset. Default value: cloudflare", key)
		return Cloudflare, nil
	case "cloudflare":
		return Cloudflare, nil
	case "unmanaged":
		return Unmanaged, nil
	case "local":
		return Local, nil
	default:
		return Unmanaged, fmt.Errorf("😡 Error parsing the variable %s with the value %s", key, val)
	}
}

func GetenvAsNonEmptyList(key string, quiet Quiet) ([]string, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		return nil, fmt.Errorf("😡 The variable %s is empty or unset.", key)
	} else {
		list := strings.Split(val, ",")
		for i := range list {
			list[i] = strings.TrimSpace(list[i])
		}
		return list, nil
	}
}

func GetenvAsBool(key string, def bool, quiet Quiet) (bool, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %t", key, def)
		}
		return def, nil
	} else {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return b, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return b, err
	}
}

func GetenvAsQuiet(key string, def Quiet, quiet Quiet) (Quiet, error) {
	b, err := GetenvAsBool(key, bool(def), quiet)
	return Quiet(b), err
}

func GetenvAsInt(key string, def int, quiet Quiet) (int, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %d", key, def)
		}
		return def, nil
	} else {
		i, err := strconv.Atoi(val)
		if err != nil {
			return i, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return i, err
	}
}

func GetenvAsPositiveTimeDuration(key string, def time.Duration, quiet Quiet) (time.Duration, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		if !quiet {
			log.Printf("📭 The variable %s is empty or unset. Default value: %s", key, def.String())
		}
		return def, nil
	} else {
		t, err := time.ParseDuration(val)
		if err != nil || t <= 0 {
			return t, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return t, err
	}
}
