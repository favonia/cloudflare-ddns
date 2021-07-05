package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/common"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
)

var jsonPath string = "/config.json"

// the JSON structure used by cloudflare-ddns
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

		if strings.TrimSpace(options.ZoneID) == "" {
			return nil, fmt.Errorf("😡 Zone ID is empty or missing.")
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

	var (
		ip4Policy detector.Policy = &detector.Unmanaged{}
		ip6Policy detector.Policy = &detector.Unmanaged{}
	)
	if config.A == nil || config.AAAA == nil {
		log.Printf("😰 Consider using the newer format to individually enable or disable IPv4 or IPv6.")
		ip4Policy = &detector.Cloudflare{}
		ip6Policy = &detector.Cloudflare{}
	} else {
		if *config.A == true {
			ip4Policy = &detector.Cloudflare{}
		}
		if *config.AAAA == true {
			ip6Policy = &detector.Cloudflare{}
		}
	}
	if !quiet {
		log.Printf("📜 Policy for IPv4: %v", ip4Policy)
		log.Printf("📜 Policy for IPv6: %v", ip6Policy)
	}

	refreshInterval := time.Minute * 5
	if !quiet {
		log.Printf("📜 Refresh interval: %v (fixed in the compatible mode)", refreshInterval)
	}

	deleteOnExit := false
	if !quiet {
		log.Printf("📜 Whether managed records are deleted on exit: %t (fixed in the compatible mode)", deleteOnExit)
	}

	return &Config{
		Sites:           sites,
		IP4Policy:       ip4Policy,
		IP6Policy:       ip6Policy,
		RefreshInterval: refreshInterval,
		Quiet:           quiet,
		DeleteOnExit:    deleteOnExit,
	}, nil
}
