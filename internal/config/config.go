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
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type TTL int

func (t TTL) String() string {
	if t == 1 {
		return "automatic"
	}
	return fmt.Sprintf("%d", t)
}

type Config struct {
	Quiet            quiet.Quiet
	Auth             api.Auth
	Policy           map[ipnet.Type]detector.Policy
	Targets          map[ipnet.Type][]api.Target
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              TTL
	Proxied          bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
}

const (
	DefaultTTL              = 1
	DefaultProxied          = false
	DefaultUpdateCron       = "@every 5m"
	DefaultUpdateOnStart    = true
	DefaultDeleteOnStop     = false
	DefaultUpdateTimeout    = time.Second * 15
	DefaultDetectionTimeout = time.Second * 5
	DefaultCacheExpiration  = time.Hour * 6
)

func readAuthToken(_ context.Context, _ quiet.Quiet) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	switch {
	case token != "" && tokenFile != "":
		log.Printf("😡 Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.")
		return "", false
	case token != "":
		return token, true
	case tokenFile != "":
		token, ok := file.ReadFileAsString(tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			log.Printf("😡 The token in the file specified by CF_API_TOKEN_FILE is empty.")
			return "", false
		}

		return token, true
	default:
		log.Printf("😡 Needs either CF_API_TOKEN or CF_API_TOKEN_FILE.")
		return "", false
	}
}

func readAuth(ctx context.Context, quiet quiet.Quiet) (api.Auth, bool) {
	token, ok := readAuthToken(ctx, quiet)
	if !ok {
		return nil, false
	}

	accountID := Getenv("CF_ACCOUNT_ID")

	return &api.TokenAuth{Token: token, AccountID: accountID}, true
}

func readDomains(_ context.Context, quiet quiet.Quiet) (ip4Targets, ip6Targets []api.Target, allOk bool) {
	var (
		rawDomains    = GetenvAsNormalizedDomains("DOMAINS", quiet)
		rawIP4Domains = GetenvAsNormalizedDomains("IP4_DOMAINS", quiet)
		rawIP6Domains = GetenvAsNormalizedDomains("IP6_DOMAINS", quiet)

		ip4DomainSet = map[string]bool{}
		ip6DomainSet = map[string]bool{}
	)

	for _, domain := range rawDomains {
		if ip4DomainSet[domain] || ip6DomainSet[domain] {
			log.Printf("😡 Domain %s has duplicates in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS.", domain)
			continue
		}

		ip4DomainSet[domain] = true

		ip4Targets = append(ip4Targets, &api.FQDNTarget{Domain: domain})
		ip6Targets = append(ip6Targets, &api.FQDNTarget{Domain: domain})
	}

	for _, domain := range rawIP4Domains {
		if ip4DomainSet[domain] {
			log.Printf("😡 Domain %s has duplicates in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS.", domain)
			continue
		}

		ip4DomainSet[domain] = true

		ip4Targets = append(ip4Targets, &api.FQDNTarget{Domain: domain})
	}

	for _, domain := range rawIP6Domains {
		if ip6DomainSet[domain] {
			log.Printf("😡 Domain %s has duplicates in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS.", domain)
			continue
		}

		ip6DomainSet[domain] = true

		ip6Targets = append(ip6Targets, &api.FQDNTarget{Domain: domain})
	}

	if len(ip4Targets) == 0 && len(ip6Targets) == 0 {
		log.Printf("😡 DOMAINS, IP4_DOMAINS, and IP6_DOMAINS are all empty or unset.")
		return nil, nil, false
	}

	return ip4Targets, ip6Targets, true
}

func readPolicy(
	_ context.Context, quiet quiet.Quiet,
	ipNet ipnet.Type, key string, targets []api.Target,
) (detector.Policy, bool) {
	var defaultPolicy detector.Policy
	switch {
	case len(targets) > 0:
		defaultPolicy = &detector.Cloudflare{Net: ipNet}
	default:
		defaultPolicy = &detector.Unmanaged{}
	}

	policy, ok := GetenvAsPolicy(ipnet.IP6, key, defaultPolicy, quiet)
	switch {
	case !ok:
		return nil, false
	case len(targets) == 0 && policy.IsManaged():
		if !quiet {
			log.Printf("🤔 No domains set for %s; %s=%s is ignored.", ipNet, key, policy)
		}
		policy = &detector.Unmanaged{}
	}

	return policy, true
}

func readPolicies(
	ctx context.Context, quiet quiet.Quiet,
	ip4Targets, ip6Targets []api.Target,
) (ip4Policy, ip6Policy detector.Policy, allOk bool) {
	ip4Policy, ip4Ok := readPolicy(ctx, quiet, ipnet.IP4, "IP4_POLICY", ip4Targets)
	if !ip4Ok {
		return nil, nil, false
	}

	ip6Policy, ip6Ok := readPolicy(ctx, quiet, ipnet.IP6, "IP6_POLICY", ip6Targets)
	if !ip6Ok {
		return nil, nil, false
	}

	if !ip4Policy.IsManaged() && !ip6Policy.IsManaged() {
		log.Printf("😡 Both IPv4 and IPv6 are unmanaged.")
		return nil, nil, false
	}

	return ip4Policy, ip6Policy, true
}

func PrintConfig(ctx context.Context, c *Config) {
	log.Printf("🔧 Policies:")
	log.Printf("   🔸 IPv4 policy:      %v", c.Policy[ipnet.IP4])
	if c.Policy[ipnet.IP4].IsManaged() {
		log.Printf("   🔸 IPv4 domains:     %v", c.Targets[ipnet.IP4])
	}
	log.Printf("   🔸 IPv6 policy:      %v", c.Policy[ipnet.IP6])
	if c.Policy[ipnet.IP6].IsManaged() {
		log.Printf("   🔸 IPv6 domains:     %v", c.Targets[ipnet.IP6])
	}
	log.Printf("🔧 Timing:")
	log.Printf("   🔸 Update frequency: %v", c.UpdateCron)
	log.Printf("   🔸 Update on start?  %t", c.UpdateOnStart)
	log.Printf("   🔸 Delete on stop?   %t", c.DeleteOnStop)
	log.Printf("   🔸 Cache expiration: %v", c.CacheExpiration)
	log.Printf("🔧 New DNS records:")
	log.Printf("   🔸 TTL:              %v", c.TTL)
	log.Printf("   🔸 Proxied:          %t", c.Proxied)
	log.Printf("🔧 Timeouts")
	log.Printf("   🔸 IP detection:     %v", c.DetectionTimeout)
	log.Printf("   🔸 Record updating:  %v", c.UpdateTimeout)
}

func ReadConfig(ctx context.Context) (*Config, bool) { //nolint:funlen,cyclop
	quiet, ok := GetenvAsQuiet("QUIET")
	if !ok {
		return nil, false
	}

	if quiet {
		log.Printf("🔇 Quiet mode enabled.")
	}

	auth, ok := readAuth(ctx, quiet)
	if !ok {
		return nil, false
	}

	ip4Targets, ip6Targets, ok := readDomains(ctx, quiet)
	if !ok {
		return nil, false
	}

	ip4Policy, ip6Policy, ok := readPolicies(ctx, quiet, ip4Targets, ip6Targets)
	if !ok {
		return nil, false
	}

	updateCron, ok := GetenvAsCron("UPDATE_CRON", cron.MustNew(DefaultUpdateCron), quiet)
	if !ok {
		return nil, false
	}

	updateOnStart, ok := GetenvAsBool("UPDATE_ON_START", DefaultUpdateOnStart, quiet)
	if !ok {
		return nil, false
	}

	deleteOnStop, ok := GetenvAsBool("DELETE_ON_STOP", DefaultDeleteOnStop, quiet)
	if !ok {
		return nil, false
	}

	cacheExpiration, ok := GetenvAsPosDuration("CACHE_EXPIRATION", DefaultCacheExpiration, quiet)
	if !ok {
		return nil, false
	}

	ttl, ok := GetenvAsInt("TTL", DefaultTTL, quiet)
	if !ok {
		return nil, false
	}

	proxied, ok := GetenvAsBool("PROXIED", DefaultProxied, quiet)
	if !ok {
		return nil, false
	}

	detectionTimeout, ok := GetenvAsPosDuration("DETECTION_TIMEOUT", DefaultDetectionTimeout, quiet)
	if !ok {
		return nil, false
	}

	updateTimeout, ok := GetenvAsPosDuration("UPDATE_TIMEOUT", DefaultUpdateTimeout, quiet)
	if !ok {
		return nil, false
	}

	return &Config{
		Quiet: quiet,
		Auth:  auth,
		Policy: map[ipnet.Type]detector.Policy{
			ipnet.IP4: ip4Policy,
			ipnet.IP6: ip6Policy,
		},
		Targets: map[ipnet.Type][]api.Target{
			ipnet.IP4: ip4Targets,
			ipnet.IP6: ip6Targets,
		},
		UpdateCron:       updateCron,
		UpdateOnStart:    updateOnStart,
		DeleteOnStop:     deleteOnStop,
		CacheExpiration:  cacheExpiration,
		TTL:              TTL(ttl),
		Proxied:          proxied,
		DetectionTimeout: detectionTimeout,
		UpdateTimeout:    updateTimeout,
	}, true
}
