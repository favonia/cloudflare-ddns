package config

import (
	"context"
	"log"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/file"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Config struct { //nolint:maligned
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
	UpdateTimeout    time.Duration
	DetectionTimeout time.Duration
	CacheExpiration  time.Duration
}

const (
	DefaultTTL              = 1
	DefaultProxied          = false
	DefaultRefreshCron      = "@every 5m"
	DefaultRefreshOnStart   = true
	DefaultDeleteOnStop     = false
	DefaultUpdateTimeout    = time.Second * 15
	DefaultDetectionTimeout = time.Second * 5
	DefaultCacheExpiration  = api.DefaultCacheExpiration
)

func readAuthToken(_ context.Context, quiet quiet.Quiet) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	switch {
	case token != "" && tokenFile != "":
		log.Printf("😡 Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.")
		return "", false //nolint:nlreturn
	case token != "":
		if !quiet {
			log.Printf("📜 CF_API_TOKEN is specified.")
		}
		return token, true //nolint:nlreturn
	case tokenFile != "":
		if !quiet {
			log.Printf("📜 CF_API_TOKEN_FILE is specified.")
		}

		token, ok := file.ReadFileAsString(tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			log.Printf("😡 The token in the file specified by CF_API_TOKEN_FILE is empty.")
			return "", false //nolint:nlreturn
		}

		return token, true
	default:
		log.Printf("😡 Needs either CF_API_TOKEN or CF_API_TOKEN_FILE.")
		return "", false //nolint:nlreturn
	}
}

func readAuth(ctx context.Context, quiet quiet.Quiet) (api.Auth, bool) {
	token, ok := readAuthToken(ctx, quiet)
	if !ok {
		return nil, false
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

	return &api.TokenAuth{Token: token, AccountID: accountID}, true
}

func readDomains(_ context.Context, quiet quiet.Quiet) (targets, ip4Targets, ip6Targets []api.Target, allOk bool) {
	var (
		rawDomains    = GetenvAsNormalizedDomains("DOMAINS", quiet)
		rawIP4Domains = GetenvAsNormalizedDomains("IP4_DOMAINS", quiet)
		rawIP6Domains = GetenvAsNormalizedDomains("IP6_DOMAINS", quiet)

		domainSet    = map[string]bool{}
		ip4DomainSet = map[string]bool{}
		ip6DomainSet = map[string]bool{}
	)

	for _, domain := range rawDomains {
		if domainSet[domain] {
			log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
			continue //nolint:nlreturn
		}

		domainSet[domain] = true

		targets = append(targets, &api.FQDNTarget{Domain: domain})
	}

	for _, domain := range rawIP4Domains {
		switch {
		case ip4DomainSet[domain]:
			log.Printf("😡 Domain %s was already listed in IP4_DOMAINS and thus ignored.", domain)
			continue //nolint:nlreturn
		case domainSet[domain]:
			log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
			continue //nolint:nlreturn
		}

		ip4DomainSet[domain] = true

		ip4Targets = append(ip4Targets, &api.FQDNTarget{Domain: domain})
	}

	for _, domain := range rawIP6Domains {
		switch {
		case ip6DomainSet[domain]:
			log.Printf("😡 Domain %s was already listed in IP6_DOMAINS and thus ignored.", domain)
			continue //nolint:nlreturn
		case domainSet[domain]:
			log.Printf("😡 Domain %s was already listed in DOMAINS and thus ignored.", domain)
			continue //nolint:nlreturn
		}

		ip6DomainSet[domain] = true

		ip6Targets = append(ip6Targets, &api.FQDNTarget{Domain: domain})
	}

	if len(targets) == 0 && len(ip4Targets) == 0 && len(ip6Targets) == 0 {
		log.Printf("😡 DOMAINS, IP4_DOMAINS, and IP6_DOMAINS are all empty or unset.")
		return nil, nil, nil, false //nolint:nlreturn
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

	return targets, ip4Targets, ip6Targets, true
}

func readPolicies(_ context.Context, quiet quiet.Quiet, targets, ip4Targets, ip6Targets []api.Target) (ip4Policy, ip6Policy detector.Policy, allOk bool) {
	var defaultIP4Policy detector.Policy
	if len(targets) > 0 || len(ip4Targets) > 0 {
		defaultIP4Policy = &detector.Cloudflare{}
	} else {
		defaultIP4Policy = &detector.Unmanaged{}
	}

	ip4Policy, ok := GetenvAsPolicy("IP4_POLICY", defaultIP4Policy, quiet)
	switch { //nolint:wsl
	case !ok:
		return nil, nil, false
	case len(targets) == 0 && len(ip4Targets) == 0 && ip4Policy.IsManaged():
		if !quiet {
			log.Printf("🤔 DOMAINS and IP4_DOMAINS are all empty, and thus IP4_POLICY=%s would be ignored.", ip4Policy)
		}
		ip4Policy = &detector.Unmanaged{} //nolint:wsl
	case len(ip4Targets) > 0 && !ip4Policy.IsManaged():
		log.Printf("😡 IPv4 is unmanaged and yet IP4_DOMAINS is not empty.")
		return nil, nil, false //nolint:nlreturn
	}

	if !quiet {
		log.Printf("📜 Policy for IPv4: %v", ip4Policy)
	}

	var defaultIP6Policy detector.Policy
	switch { //nolint:wsl
	case len(targets) > 0 || len(ip6Targets) > 0:
		defaultIP6Policy = &detector.Cloudflare{}
	default:
		defaultIP6Policy = &detector.Unmanaged{}
	}

	ip6Policy, ok = GetenvAsPolicy("IP6_POLICY", defaultIP6Policy, quiet)
	switch { //nolint:wsl
	case !ok:
		return nil, nil, false
	case len(targets) == 0 && len(ip6Targets) == 0 && ip6Policy.IsManaged():
		if !quiet {
			log.Printf("🤔 DOMAINS and IP6_DOMAINS are all empty, and thus IP6_POLICY=%s would be ignored.", ip6Policy)
		}
		ip6Policy = &detector.Unmanaged{} //nolint:wsl
	case len(ip6Targets) > 0 && !ip6Policy.IsManaged():
		log.Printf("😡 IPv6 is unmanaged and yet IP6_DOMAINS is not empty.")
		return nil, nil, false //nolint:nlreturn
	}

	if !quiet {
		log.Printf("📜 Policy for IPv6: %v", ip6Policy)
	}

	if !ip4Policy.IsManaged() && !ip6Policy.IsManaged() {
		log.Printf("😡 Both IPv4 and IPv6 are unmanaged.")
		return nil, nil, false //nolint:nlreturn
	}

	return ip4Policy, ip6Policy, true
}

func ReadConfig(ctx context.Context) (*Config, bool) {
	quiet, ok := GetenvAsQuiet("QUIET")
	if !ok {
		return nil, false
	}

	if quiet {
		log.Printf("🤫 Quiet mode enabled.")
	}

	auth, ok := readAuth(ctx, quiet)
	if !ok {
		return nil, false
	}

	targets, ip4Targets, ip6Targets, ok := readDomains(ctx, quiet)
	if !ok {
		return nil, false
	}

	ip4Policy, ip6Policy, ok := readPolicies(ctx, quiet, targets, ip4Targets, ip6Targets)
	if !ok {
		return nil, false
	}

	ttl, ok := GetenvAsInt("TTL", 1, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 TTL for new DNS entries: %d (1 = automatic)", ttl)
	}

	proxied, ok := GetenvAsBool("PROXIED", false, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Whether new DNS entries are proxied: %t", proxied)
	}

	refreshCron, ok := GetenvAsCron("REFRESH_CRON", cron.MustNew(DefaultRefreshCron), quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Refresh schedule: %v", refreshCron)
	}

	refreshOnStart, ok := GetenvAsBool("REFRESH_ON_START", DefaultRefreshOnStart, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Whether to refresh IP addresses on start: %t", refreshOnStart)
	}

	deleteOnStop, ok := GetenvAsBool("DELETE_ON_STOP", false, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Whether managed records are deleted on exit: %t", deleteOnStop)
	}

	updateTimeout, ok := GetenvAsPosDuration("CF_API_TIMEOUT", DefaultUpdateTimeout, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Timeout of each access to the CloudFlare API: %v", updateTimeout)
	}

	detectionTimeout, ok := GetenvAsPosDuration("DETECTION_TIMEOUT", DefaultDetectionTimeout, quiet)
	if !ok {
		return nil, false
	}

	if !quiet {
		log.Printf("📜 Timeout of each attempt to detect IP addresses: %v", detectionTimeout)
	}

	cacheExpiration, ok := GetenvAsPosDuration("CACHE_EXPIRATION", DefaultCacheExpiration, quiet)
	if !ok {
		return nil, false
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
		UpdateTimeout:    updateTimeout,
		DetectionTimeout: detectionTimeout,
		CacheExpiration:  cacheExpiration,
	}, true
}
