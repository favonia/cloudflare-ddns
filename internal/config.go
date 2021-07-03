package ddns

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Policy int

const (
	Disabled Policy = iota
	CloudFlare
	Local
)

func (p Policy) String() string {
	switch p {
	case Disabled:
		return "disabled"
	case CloudFlare:
		return "cloudflare"
	case Local:
		return "local"
	default:
		return "<unrecognized>"
	}
}

type Config struct {
	Token           string
	Domains         []string
	IP4Policy       Policy // "cloudflare", "local", "disabled"
	IP6Policy       Policy // "cloudflare", "local", "disabled"
	TTL             int
	Proxied         bool
	RefreshInterval time.Duration
}

func GetenvAsPolicy(key string) (Policy, error) {
	val := strings.TrimSpace(os.Getenv(key))
	switch val {
	case "cloudflare", "":
		return CloudFlare, nil
	case "disabled":
		return Disabled, nil
	case "local":
		return Local, nil
	default:
		return Disabled, fmt.Errorf("😡 Error parsing the variable %s with the value %s", key, val)
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
		log.Printf("🈳 The variable %s is missing. Default value: %t", key, def)
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
		log.Printf("🈳 The variable %s is missing. Default value: %d", key, def)
		return def, nil
	} else {
		i, err := strconv.Atoi(val)
		if err != nil {
			return i, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return i, err
	}
}

func GetenvAsTimeDuration(key string, def time.Duration) (time.Duration, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		log.Printf("🈳 The variable %s is missing. Default value: %s", key, def.String())
		return def, nil
	} else {
		t, err := time.ParseDuration(val)
		if err != nil {
			return t, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return t, err
	}
}

func ReadEnv() (*Config, error) {
	token := os.Getenv("CF_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("😡 The Cloudflare API token (CF_API_TOKEN) is missing.")
	}
	domains, err := GetenvAsNonEmptyList("DOMAINS")
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Domains to check: %v", domains)
	ip6Policy, err := GetenvAsPolicy("IP6_POLICY")
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IP6: %v", ip6Policy)
	ip4Policy, err := GetenvAsPolicy("IP4_POLICY")
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IP4: %v", ip4Policy)
	ttl, err := GetenvAsInt("TTL", 1)
	if err != nil {
		return nil, err
	}
	proxied, err := GetenvAsBool("PROXIED", false)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Whether new DNS entries are proxied: %t", proxied)
	refreshInterval, err := GetenvAsTimeDuration("REFRESH_INTERVAL", time.Minute*5)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Refresh interval: %s", refreshInterval.String())

	return &Config{
		Token:           token,
		Domains:         domains,
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		TTL:             ttl,
		Proxied:         proxied,
		RefreshInterval: refreshInterval,
	}, nil
}
