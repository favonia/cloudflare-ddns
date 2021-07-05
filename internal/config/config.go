package config

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/common"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
)

type Config struct {
	Quiet            common.Quiet
	Handler          api.Handler
	Targets          []api.Target
	IP4Policy        detector.Policy
	IP6Policy        detector.Policy
	TTL              int
	Proxied          bool
	RefreshInterval  time.Duration
	DeleteOnExit     bool
	DetectionTimeout time.Duration
	CacheExpiration  time.Duration
}

func ReadConfig(ctx context.Context) (*Config, error) {
	quiet, err := GetenvAsQuiet("QUIET", common.VERBOSE, common.VERBOSE)
	if err != nil {
		return nil, err
	}
	if quiet {
		log.Printf("🤫 Quiet mode enabled.")
	}

	var (
		token     = os.Getenv("CF_API_TOKEN")
		tokenFile = os.Getenv("CF_API_TOKEN_FILE")
		handler   = api.Handler(nil)
	)
	switch {
	case token == "" && tokenFile == "":
		return nil, fmt.Errorf("😡 Needs CF_API_TOKEN or CF_API_TOKEN_FILE.")
	case token != "" && tokenFile != "":
		return nil, fmt.Errorf("😡 Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.")
	case token != "":
		handler = &api.TokenHandler{Token: token}
	case tokenFile != "":
		tokenBytes, err := common.ReadFile(tokenFile)
		if err != nil {
			return nil, err
		}
		handler = &api.TokenHandler{Token: string(bytes.TrimSpace(tokenBytes))}
	}

	domains, err := GetenvAsNonEmptyList("DOMAINS", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Managed domains: %v", domains)

	// converting domains to generic targets
	targets := make([]api.Target, len(domains))
	for i, domain := range domains {
		targets[i] = &api.FQDNTarget{Domain: domain}
	}

	ip4Policy, err := GetenvAsPolicy("IP4_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IPv4: %v", ip4Policy)

	ip6Policy, err := GetenvAsPolicy("IP6_POLICY", quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Policy for IPv6: %v", ip6Policy)

	ttl, err := GetenvAsInt("TTL", 1, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", ttl)

	proxied, err := GetenvAsBool("PROXIED", false, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Whether new DNS entries are proxied: %t", proxied)

	refreshInterval, err := GetenvAsPositiveTimeDuration("REFRESH_INTERVAL", time.Minute*5, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Refresh interval: %v", refreshInterval)

	deleteOnExit, err := GetenvAsBool("DELETE_ON_EXIT", false, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Whether managed records are deleted on exit: %t", deleteOnExit)

	detectionTimeout, err := GetenvAsPositiveTimeDuration("DETECTION_TIMEOUT", time.Second*5, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Timeout of each attempt to detect IP addresses: %v", detectionTimeout)

	cacheExpiration, err := GetenvAsPositiveTimeDuration("CACHE_EXPIRATION", api.DefaultCacheExpiration, quiet)
	if err != nil {
		return nil, err
	}
	log.Printf("📜 Expiration of cached CloudFlare API responses: %v", cacheExpiration)

	return &Config{
		Quiet:            quiet,
		Handler:          handler,
		Targets:          targets,
		IP4Policy:        ip4Policy,
		IP6Policy:        ip6Policy,
		TTL:              ttl,
		Proxied:          proxied,
		RefreshInterval:  refreshInterval,
		DeleteOnExit:     deleteOnExit,
		DetectionTimeout: detectionTimeout,
		CacheExpiration:  cacheExpiration,
	}, nil
}
