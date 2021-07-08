package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/common"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
)

func Getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func GetenvAsBool(key string, def bool, quiet common.Quiet) (bool, error) {
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

func GetenvAsQuiet(key string, def common.Quiet, quiet common.Quiet) (common.Quiet, error) {
	b, err := GetenvAsBool(key, bool(def), quiet)
	return common.Quiet(b), err
}

func GetenvAsInt(key string, def int, quiet common.Quiet) (int, error) {
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

func GetenvAsNonEmptyList(key string, quiet common.Quiet) ([]string, error) {
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

func GetenvAsPolicy(key string, quiet common.Quiet) (detector.Policy, error) {
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

func GetenvAsPositiveTimeDuration(key string, def time.Duration, quiet common.Quiet) (time.Duration, error) {
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
