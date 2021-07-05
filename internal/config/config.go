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

type site = struct {
	Handler api.Handler
	Targets []api.Target
	TTL     int
	Proxied bool
}

type Config struct {
	Sites           []site
	IP4Policy       detector.Policy // "cloudflare", "local", "unmanaged"
	IP6Policy       detector.Policy // "cloudflare", "local", "unmanaged"
	RefreshInterval time.Duration
	Quiet           common.Quiet
	DeleteOnExit    bool
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
		DeleteOnExit:    deleteOnExit,
	}, nil
}

func ReadConfig(ctx context.Context) (*Config, error) {
	quiet, err := GetenvAsQuiet("QUIET", common.VERBOSE, common.VERBOSE)
	if err != nil {
		return nil, err
	}
	if quiet {
		log.Printf("🤫 Quiet mode enabled.")
	}

	useJSON, err := GetenvAsBool("COMPATIBLE", false, quiet)
	if err != nil {
		return nil, err
	}
	if useJSON {
		if !quiet {
			log.Print("🤝 Using the cloudflare-ddns compatible mode.")
		}
		return readConfigFromJSON(ctx, jsonPath, quiet)
	} else {
		if !quiet {
			log.Printf("🆕 Not using the cloudflare-ddns compatible mode.")
		}
		return readConfigFromEnv(ctx, quiet)
	}
}
