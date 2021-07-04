package common

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func GetenvAsPolicy(key string) (Policy, error) {
	val := strings.TrimSpace(os.Getenv(key))
	switch val {
	case "cloudflare", "":
		return Cloudflare, nil
	case "unmanaged":
		return Unmanaged, nil
	case "local":
		return Local, nil
	default:
		return Unmanaged, fmt.Errorf("😡 Error parsing the variable %s with the value %s", key, val)
	}
}

func GetenvAsNonEmptyList(key string) ([]string, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		return nil, fmt.Errorf("😡 The variable %s is missing.", key)
	} else {
		list := strings.Split(val, ",")
		for i := range list {
			list[i] = strings.TrimSpace(list[i])
		}
		return list, nil
	}
}

func GetenvAsBool(key string, def bool) (bool, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		log.Printf("📭 The variable %s is missing. Default value: %t", key, def)
		return def, nil
	} else {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return b, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return b, err
	}
}

func GetenvAsInt(key string, def int) (int, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		log.Printf("📭 The variable %s is missing. Default value: %d", key, def)
		return def, nil
	} else {
		i, err := strconv.Atoi(val)
		if err != nil {
			return i, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return i, err
	}
}

func GetenvAsPositiveTimeDuration(key string, def time.Duration) (time.Duration, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		log.Printf("📭 The variable %s is missing. Default value: %s", key, def.String())
		return def, nil
	} else {
		t, err := time.ParseDuration(val)
		if err != nil || t <= 0 {
			return t, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return t, err
	}
}
