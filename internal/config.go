package ddns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Policy int

const (
	Unmanaged Policy = iota
	Cloudflare
	Local
)

func (p Policy) String() string {
	switch p {
	case Unmanaged:
		return "unmanaged"
	case Cloudflare:
		return "cloudflare"
	case Local:
		return "local"
	default:
		return "<unrecognized>"
	}
}

type site = struct {
	Handle  *handle
	Domains []string
	TTL     int
	Proxied bool
}

type Config struct {
	Sites           []site
	IP4Policy       Policy // "cloudflare", "local", "unmanaged"
	IP6Policy       Policy // "cloudflare", "local", "unmanaged"
	RefreshInterval time.Duration
}

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

func GetenvAsTimeDuration(key string, def time.Duration) (time.Duration, error) {
	if val := strings.TrimSpace(os.Getenv(key)); val == "" {
		log.Printf("📭 The variable %s is missing. Default value: %s", key, def.String())
		return def, nil
	} else {
		t, err := time.ParseDuration(val)
		if err != nil {
			return t, fmt.Errorf("😡 Error parsing the variable %s: %v", key, err)
		}
		return t, err
	}
}

func readConfigFromEnv(ctx context.Context) (*Config, error) {
	token := os.Getenv("CF_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("😡 The Cloudflare API token (CF_API_TOKEN) is missing.")
	}
	handle, err := newHandleWithToken(token)
	if err != nil {
		return nil, err
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
	log.Printf("📜 Policy for IPv6: %v", ip6Policy)
	ip4Policy, err := GetenvAsPolicy("IP4_POLICY")
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IPv4: %v", ip4Policy)
	ttl, err := GetenvAsInt("TTL", 1)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", ttl)
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
		Sites: []site{{
			Handle:  handle,
			Domains: domains,
			TTL:     ttl,
			Proxied: proxied,
		}},
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		RefreshInterval: refreshInterval,
	}, nil
}

// the JSON structure used by cloudflare DDNS
type jsonConfig struct {
	Cloudflare []struct {
		Authentication struct {
			APIToken *string `json:"api_token,omitempty"`
			APIKey   *struct {
				APIKey       string `json:"api_key"`
				AccountEmail string `json:"account_email"`
			} `json:"api_key,omitempty"`
		} `json:"authentication"`
		ZoneID     string   `json:"zone_id"`
		Subdomains []string `json:"subdomains"`
		Proxied    bool     `json:"proxied"`
	} `json:"cloudflare"`
	A    *bool `json:"a,omitempty"`
	AAAA *bool `json:"aaaa,omitempty"`
}

// compatible mode for cloudflare-ddns
func readConfigFromJSON(ctx context.Context, path string) (*Config, error) {
	jsonFile, err := os.Open(path)
	defer jsonFile.Close()
	jsonBytes, err := io.ReadAll(jsonFile)

	var config *jsonConfig
	err = json.Unmarshal(jsonBytes, &config)
	if err != nil {
		return nil, err
	}

	sites := make([]site, len(config.Cloudflare))
	for i, options := range config.Cloudflare {
		if token := options.Authentication.APIToken; token != nil && *token != "" {
			sites[i].Handle, err = newHandleWithToken(*token)
			if err == nil {
				log.Printf("🔑 Using an API token for authentication.")
			} else {
				log.Print(err)
			}
		}
		if key := options.Authentication.APIKey; key != nil && key.APIKey != "" {
			sites[i].Handle, err = newHandleWithKey(key.APIKey, key.AccountEmail)
			if err != nil {
				return nil, err
			}
			log.Printf("🗝️ Using an API key for authentication.")
		} else {
			return nil, fmt.Errorf("😡 Needs at least the API token or the API key.")
		}

		zone, err := sites[i].Handle.zoneDetails(ctx, options.ZoneID)
		if err != nil {
			return nil, err
		}
		log.Printf("🧐 Found the zone at %s from the zone ID %s.", zone.Name, options.ZoneID)

		sites[i].Domains = options.Subdomains
		log.Printf("🔧 Appending the zone name to all subdomain names . . .")
		for j, sub := range sites[i].Domains {
			if sub == "" {
				continue
			}
			sites[i].Domains[j] = fmt.Sprintf("%s.%s", sub, zone.Name)
		}

		sites[i].TTL = 60 * 5
		log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", sites[i].TTL)

		sites[i].Proxied = options.Proxied
		log.Printf("📜 Whether new DNS entries are proxied: %t", sites[i].Proxied)
	}

	ip4Policy := Unmanaged
	ip6Policy := Unmanaged
	if config.A == nil || config.AAAA == nil {
		log.Printf("🆙 Please upgrade your configuration file and individually disable IPv4 or IPv6.")
		ip4Policy = Cloudflare
		ip6Policy = Cloudflare
	} else {
		if *config.A == true {
			ip4Policy = Cloudflare
		}
		if *config.AAAA == true {
			ip6Policy = Cloudflare
		}
	}

	refreshInterval := time.Minute * 5

	return &Config{
		Sites:           sites,
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		RefreshInterval: refreshInterval,
	}, nil
}

var jsonPath string = "/config.json"

func ReadConfig(ctx context.Context) (*Config, error) {
	useJSON, err := GetenvAsBool("USE_JSON", false)
	if err != nil {
		return nil, err
	}
	if useJSON {
		log.Printf("🤝 Using the JSON compatible mode; reading %s . . .", jsonPath)
		return readConfigFromJSON(ctx, jsonPath)
	} else {
		log.Printf("🥳 Not using the JSON compatible mode.")
		return readConfigFromEnv(ctx)
	}
}
