package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
	"github.com/favonia/cloudflare-ddns-go/internal/file"
)

type Config struct {
	Quiet            quiet.Quiet
	NewHandler       api.NewHandler
	Targets          []api.Target
	IP4Policy        detector.Policy
	IP6Policy        detector.Policy
	TTL              int
	Proxied          bool
	RefreshInterval  time.Duration
	DeleteOnStop     bool
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

	var newHandler api.NewHandler
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

		newHandler = &api.TokenNewHandler{Token: token, AccountID: accountID}
	}

	domains, err := GetenvAsNonEmptyList("DOMAINS", quiet)
	for i, domain := range domains {
		domains[i] = normalizeDomain(domain)
	}
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Managed domains: %v", domains)
	}

	// converting domains to generic targets
	targets := make([]api.Target, len(domains))
	for i, domain := range domains {
		targets[i] = &api.FQDNTarget{Domain: domain}
	}

	ip4Policy, err := GetenvAsPolicy("IP4_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Policy for IPv4: %v", ip4Policy)
	}

	ip6Policy, err := GetenvAsPolicy("IP6_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Policy for IPv6: %v", ip6Policy)
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

	refreshInterval, err := GetenvAsPositiveTimeDuration("REFRESH_INTERVAL", time.Minute*5, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Refresh interval: %v", refreshInterval)
	}

	deleteOnStop, err := GetenvAsBool("DELETE_ON_STOP", false, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Whether managed records are deleted on exit: %t", deleteOnStop)
	}

	detectionTimeout, err := GetenvAsPositiveTimeDuration("DETECTION_TIMEOUT", time.Second*5, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Timeout of each attempt to detect IP addresses: %v", detectionTimeout)
	}

	cacheExpiration, err := GetenvAsPositiveTimeDuration("CACHE_EXPIRATION", api.DefaultCacheExpiration, quiet)
	if err != nil {
		return nil, err
	}
	if !quiet {
		log.Printf("📜 Expiration of cached CloudFlare API responses: %v", cacheExpiration)
	}

	return &Config{
		Quiet:            quiet,
		NewHandler:       newHandler,
		Targets:          targets,
		IP4Policy:        ip4Policy,
		IP6Policy:        ip6Policy,
		TTL:              ttl,
		Proxied:          proxied,
		RefreshInterval:  refreshInterval,
		DeleteOnStop:     deleteOnStop,
		DetectionTimeout: detectionTimeout,
		CacheExpiration:  cacheExpiration,
	}, nil
}
