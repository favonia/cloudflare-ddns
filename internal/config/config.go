package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/file"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Config struct {
	Quiet            quiet.Quiet
	Auth             api.Auth
	Targets          []api.Target
	IP4Targets       []api.Target
	IP6Targets       []api.Target
	IP4Policy        detector.Policy
	IP6Policy        detector.Policy
	TTL              int
	Proxied          bool
	RefreshCron      cron.Schedule
	RefreshOnStart   bool
	DeleteOnStop     bool
	APITimeout       time.Duration
	DetectionTimeout time.Duration
	CacheExpiration  time.Duration
}

func ReadConfig(ctx context.Context) (*Config, error) {
	quiet, err := GetenvAsQuiet("QUIET")
	if err != nil {
		return nil, err
	}
	if quiet {
		log.Printf("🤫 Quiet mode enabled.")
	}

	var auth api.Auth
	{
		token := Getenv("CF_API_TOKEN")
		{
			tokenFile := Getenv("CF_API_TOKEN_FILE")
			switch {
			case token == "" && tokenFile == "":
				return nil, fmt.Errorf("😡 Needs either CF_API_TOKEN or CF_API_TOKEN_FILE.")
			case token != "" && tokenFile != "":
				return nil, fmt.Errorf("😡 Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.")
			case token != "":
				if !quiet {
					log.Printf("📜 CF_API_TOKEN is specified.")
				}
			case tokenFile != "":
				if !quiet {
					log.Printf("📜 CF_API_TOKEN_FILE is specified.")
				}
				token, err = file.ReadFileAsString(tokenFile)
				if err != nil {
					return nil, err
				}
				if token == "" {
					return nil, fmt.Errorf("😡 The token in the file specified by CF_API_TOKEN_FILE is empty.")
				}
			}
		}

		accountID := Getenv("CF_ACCOUNT_ID")
		if !quiet {
			switch accountID {
			case "":
				log.Printf("📜 CF_ACCOUNT_ID is not specified (which is fine).")
			default:
				log.Printf("📜 CF_ACCOUNT_ID is specified.")
			}
		}

		auth = &api.TokenAuth{Token: token, AccountID: accountID}
	}

	var (
		targets    []api.Target
		ip4Targets []api.Target
		ip6Targets []api.Target
	)

	{
		domains := GetenvAsNormalizedDomains("DOMAINS", quiet)
		ip4Domains := GetenvAsNormalizedDomains("IP4_DOMAINS", quiet)
		ip6Domains := GetenvAsNormalizedDomains("IP6_DOMAINS", quiet)

		var (
			addedDomains    map[string]bool = map[string]bool{}
			addedIP4Domains map[string]bool = map[string]bool{}
			addedIP6Domains map[string]bool = map[string]bool{}
		)

		for _, domain := range domains {
			if addedDomains[domain] {
				log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
				continue
			}
			addedDomains[domain] = true
			targets = append(targets, &api.FQDNTarget{Domain: domain})
		}
		for _, domain := range ip4Domains {
			if addedIP4Domains[domain] {
				log.Printf("😡 Domain %s was already listed in IP4_DOMAINS and thus ignored.", domain)
				continue
			}
			if addedDomains[domain] {
				log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
				continue
			}
			addedIP4Domains[domain] = true
			ip4Targets = append(ip4Targets, &api.FQDNTarget{Domain: domain})
		}
		for _, domain := range ip6Domains {
			if addedIP6Domains[domain] {
				log.Printf("😡 Domain %s was already listed in IP6_DOMAINS and thus ignored.", domain)
				continue
			}
			if addedDomains[domain] {
				log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
				continue
			}
			addedIP6Domains[domain] = true
			ip6Targets = append(ip6Targets, &api.FQDNTarget{Domain: domain})
		}

		if len(targets) == 0 && len(ip4Targets) == 0 && len(ip6Targets) == 0 {
			return nil, fmt.Errorf("😡 DOMAINS, IP4_DOMAINS, and IP6_DOMAINS are all empty or unset.")
		}

		if !quiet {
			if len(targets) > 0 {
				log.Printf("📜 Managed domains for IPv4 and IPv6: %v", targets)
			}
			if len(ip4Targets) > 0 {
				log.Printf("📜 Managed domains for IPv4: %v", ip4Targets)
			}
			if len(ip6Targets) > 0 {
				log.Printf("📜 Managed domains for IPv6: %v", ip6Targets)
			}
		}
	}

	var defaultIP4Policy detector.Policy
	if len(targets) > 0 || len(ip4Targets) > 0 {
		defaultIP4Policy = &detector.Cloudflare{}
	} else {
		defaultIP4Policy = &detector.Unmanaged{}
	}
	ip4Policy, err := GetenvAsPolicy("IP4_POLICY", defaultIP4Policy, quiet)
	switch {
	case err != nil:
		return nil, err
	case len(targets) == 0 && len(ip4Targets) == 0 && ip4Policy.IsManaged():
		if !quiet {
			log.Printf("🤔 DOMAINS and IP4_DOMAINS are all empty, and thus IP4_POLICY=%s would be ignored.", ip4Policy)
		}
		ip4Policy = &detector.Unmanaged{}
	case len(ip4Targets) > 0 && !ip4Policy.IsManaged():
		return nil, fmt.Errorf("😡 IPv4 is unmanaged and yet IP4_DOMAINS is not empty.")
	}
	if !quiet {
		log.Printf("📜 Policy for IPv4: %v", ip4Policy)
	}

	var defaultIP6Policy detector.Policy
	switch {
	case len(targets) > 0 || len(ip6Targets) > 0:
		defaultIP6Policy = &detector.Cloudflare{}
	default:
		defaultIP6Policy = &detector.Unmanaged{}
	}
	ip6Policy, err := GetenvAsPolicy("IP6_POLICY", defaultIP6Policy, quiet)
	switch {
	case err != nil:
		return nil, err
	case len(targets) == 0 && len(ip6Targets) == 0 && ip6Policy.IsManaged():
		if !quiet {
			log.Printf("🤔 DOMAINS and IP6_DOMAINS are all empty, and thus IP6_POLICY=%s would be ignored.", ip6Policy)
		}
		ip6Policy = &detector.Unmanaged{}
	case len(ip6Targets) > 0 && !ip6Policy.IsManaged():
		return nil, fmt.Errorf("😡 IPv6 is unmanaged and yet IP6_DOMAINS is not empty.")
	}
	if !quiet {
		log.Printf("📜 Policy for IPv6: %v", ip6Policy)
	}

	if !ip4Policy.IsManaged() && !ip6Policy.IsManaged() {
		return nil, fmt.Errorf("😡 Both IPv4 and IPv6 are unmanaged.")
	}

	ttl, err := GetenvAsInt("TTL", 1, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", ttl)
	}

	proxied, err := GetenvAsBool("PROXIED", false, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Whether new DNS entries are proxied: %t", proxied)
	}

	refreshCron, err := GetenvAsCron("REFRESH_CRON", cron.MustNew("@every 5m"), quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Refresh schedule: %v", refreshCron)
	}

	refreshOnStart, err := GetenvAsBool("REFRESH_ON_START", true, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Whether to refresh IP addresses on start: %t", refreshOnStart)
	}

	deleteOnStop, err := GetenvAsBool("DELETE_ON_STOP", false, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Whether managed records are deleted on exit: %t", deleteOnStop)
	}

	apiTimeout, err := GetenvAsPosDuration("CF_API_TIMEOUT", time.Second*10, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Timeout of each access to the CloudFlare API: %v", apiTimeout)
	}

	detectionTimeout, err := GetenvAsPosDuration("DETECTION_TIMEOUT", time.Second*5, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Timeout of each attempt to detect IP addresses: %v", detectionTimeout)
	}

	cacheExpiration, err := GetenvAsPosDuration("CACHE_EXPIRATION", api.DefaultCacheExpiration, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Expiration of cached CloudFlare API responses: %v", cacheExpiration)
	}

	return &Config{
		Quiet:            quiet,
		Auth:             auth,
		Targets:          targets,
		IP4Targets:       ip4Targets,
		IP6Targets:       ip6Targets,
		IP4Policy:        ip4Policy,
		IP6Policy:        ip6Policy,
		TTL:              ttl,
		Proxied:          proxied,
		RefreshCron:      refreshCron,
		RefreshOnStart:   refreshOnStart,
		DeleteOnStop:     deleteOnStop,
		APITimeout:       apiTimeout,
		DetectionTimeout: detectionTimeout,
		CacheExpiration:  cacheExpiration,
	}, nil
}
