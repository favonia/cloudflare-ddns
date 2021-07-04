package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/common"
)

type site = struct {
	Handler api.Handler
	Targets []api.Target
	TTL     int
	Proxied bool
}

type Config struct {
	Sites           []site
	IP4Policy       common.Policy // "cloudflare", "local", "unmanaged"
	IP6Policy       common.Policy // "cloudflare", "local", "unmanaged"
	RefreshInterval time.Duration
	Quiet           common.Quiet
}

func readConfigFromEnv(ctx context.Context, quiet common.Quiet) (*Config, error) {
	var (
		token     = os.Getenv("CF_API_TOKEN")
		tokenFile = os.Getenv("CF_API_TOKEN_FILE")
		handler   = api.Handler(nil)
	)
	switch {
	case token == "" && tokenFile == "":
		return nil, fmt.Errorf("😡 Needs CF_API_TOKEN or CF_API_TOKEN_FILE.")
	case token != "" && tokenFile != "":
		return nil, fmt.Errorf("😡 Cannot set both CF_API_TOKEN and CF_API_TOKEN_FILE.")
	case token != "":
		handler = &api.TokenHandler{Token: token}
	case tokenFile != "":
		tokenBytes, err := common.ReadFile(tokenFile)
		if err != nil {
			return nil, err
		}
		handler = &api.TokenHandler{Token: string(bytes.TrimSpace(tokenBytes))}
	}

	domains, err := common.GetenvAsNonEmptyList("DOMAINS", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Managed domains: %v", domains)

	// converting domains to generic targets
	targets := make([]api.Target, len(domains))
	for i, domain := range domains {
		targets[i] = &api.FQDNTarget{Domain: domain}
	}

	ip4Policy, err := common.GetenvAsPolicy("IP4_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IPv4: %v", ip4Policy)

	ip6Policy, err := common.GetenvAsPolicy("IP6_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IPv6: %v", ip6Policy)

	ttl, err := common.GetenvAsInt("TTL", 1, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", ttl)

	proxied, err := common.GetenvAsBool("PROXIED", false, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Whether new DNS entries are proxied: %t", proxied)

	refreshInterval, err := common.GetenvAsPositiveTimeDuration("REFRESH_INTERVAL", time.Minute*5, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Refresh interval: %s", refreshInterval.String())

	return &Config{
		Sites: []site{{
			Handler: handler,
			Targets: targets,
			TTL:     ttl,
			Proxied: proxied,
		}},
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		RefreshInterval: refreshInterval,
		Quiet:           quiet,
	}, nil
}

// the JSON structure used by CloudFlare-DDNS
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
func readConfigFromJSON(ctx context.Context, path string, quiet common.Quiet) (*Config, error) {
	jsonBytes, err := common.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config *jsonConfig
	err = json.Unmarshal(jsonBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("😡 Could not parse %s: %v", path, err)
	}

	sites := make([]site, len(config.Cloudflare))
	for i, options := range config.Cloudflare {
		if token := options.Authentication.APIToken; token != nil &&
			*token != "" && *token != "api_token_here" {
			if !quiet {
				log.Printf("🔑 Using an API token for authentication . . .")
			}
			sites[i].Handler = &api.TokenHandler{Token: *token}
		}
		if sites[i].Handler == nil {
			if key := options.Authentication.APIKey; key != nil &&
				key.APIKey != "" && key.APIKey != "api_key_here" &&
				key.AccountEmail != "" && key.AccountEmail != "your_email_here" {
				if !quiet {
					log.Printf("🗝️ Using an API key for authentication . . .")
					log.Printf("😰 Please consider using the more secure API tokens.")
				}
				sites[i].Handler = &api.KeyHandler{Key: key.APIKey, Email: key.AccountEmail}
			} else {
				return nil, fmt.Errorf("😡 Needs at least an API token or an API key.")
			}
		}

		if !quiet {
			log.Printf("📜 Managed subdomains: %v", options.Subdomains)
		}
		// converting domains to generic targets
		sites[i].Targets = make([]api.Target, len(options.Subdomains))
		for j, sub := range options.Subdomains {
			sites[i].Targets[j] = &api.SubdomainTarget{ZoneID: options.ZoneID, Subdomain: sub}
		}

		sites[i].TTL = 60 * 5
		if !quiet {
			log.Printf("📜 TTL for new DNS entries: %d (fixed in the compatible mode)", sites[i].TTL)
		}

		sites[i].Proxied = options.Proxied
		if !quiet {
			log.Printf("📜 Whether new DNS entries are proxied: %t", sites[i].Proxied)
		}
	}

	ip4Policy := common.Unmanaged
	ip6Policy := common.Unmanaged
	if config.A == nil || config.AAAA == nil {
		log.Printf("😰 Consider using the newer format to individually enable or disable IPv4 or IPv6.")
		ip4Policy = common.Cloudflare
		ip6Policy = common.Cloudflare
	} else {
		if *config.A == true {
			ip4Policy = common.Cloudflare
		}
		if *config.AAAA == true {
			ip6Policy = common.Cloudflare
		}
	}
	if !quiet {
		log.Printf("📜 Policy for IPv4: %v", ip4Policy)
		log.Printf("📜 Policy for IPv6: %v", ip6Policy)
	}

	refreshInterval := time.Minute * 5
	if !quiet {
		log.Printf("📜 Refresh interval: %s (fixed in the compatible mode)", refreshInterval.String())
	}

	return &Config{
		Sites:           sites,
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		RefreshInterval: refreshInterval,
		Quiet:           quiet,
	}, nil
}

var jsonPath string = "/config.json"

func ReadConfig(ctx context.Context) (*Config, error) {
	quiet, err := common.GetenvAsQuiet("QUIET", common.VERBOSE, common.VERBOSE)
	if err != nil {
		return nil, err
	}
	if quiet {
		log.Printf("🤫 Quiet mode enabled.")
	}

	useJSON, err := common.GetenvAsBool("COMPATIBLE", false, quiet)
	if err != nil {
		return nil, err
	}
	if useJSON {
		if !quiet {
			log.Printf("🤝 Using the CloudFlare-DDNS compatible mode; reading %s . . .", jsonPath)
		}
		return readConfigFromJSON(ctx, jsonPath, quiet)
	} else {
		if !quiet {
			log.Printf("😎 Not using the CloudFlare-DDNS compatible mode; checking environment variables . . .")
		}
		return readConfigFromEnv(ctx, quiet)
	}
}
