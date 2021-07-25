package config

import (
	"log"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/cron"
	"github.com/favonia/cloudflare-ddns-go/internal/detector"
	"github.com/favonia/cloudflare-ddns-go/internal/file"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

type Config struct {
	Quiet            quiet.Quiet
	Auth             api.Auth
	Policy           map[ipnet.Type]detector.Policy
	Domains          map[ipnet.Type][]api.FQDN
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	Proxied          bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
}

// Default gives default values.
func Default() *Config {
	return &Config{
		Quiet: quiet.Quiet(false),
		Auth:  nil,
		Policy: map[ipnet.Type]detector.Policy{
			ipnet.IP4: &detector.Cloudflare{Net: ipnet.IP4},
			ipnet.IP6: &detector.Cloudflare{Net: ipnet.IP6},
		},
		Domains: map[ipnet.Type][]api.FQDN{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTL:              api.TTL(1),
		Proxied:          false,
		UpdateTimeout:    time.Hour,
		DetectionTimeout: time.Second * 5, //nolint:gomnd
	}
}

func readAuthToken(_ quiet.Quiet) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	// foolproof checks
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		log.Printf("😡 You need to provide a real API token as CF_API_TOKEN.")
		return "", false
	}

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

func readAuth(quiet quiet.Quiet, field *api.Auth) bool {
	token, ok := readAuthToken(quiet)
	if !ok {
		return false
	}

	accountID := Getenv("CF_ACCOUNT_ID")

	*field = &api.TokenAuth{Token: token, AccountID: accountID}
	return true
}

func readDomains(quiet quiet.Quiet, field *map[ipnet.Type][]api.FQDN) bool { //nolint:funlen,cyclop
	var rawDomains, rawIP4Domains, rawIP6Domains []string

	if !ReadDomains(quiet, "DOMAINS", &rawDomains) ||
		!ReadDomains(quiet, "IP4_DOMAINS", &rawIP4Domains) ||
		!ReadDomains(quiet, "IP6_DOMAINS", &rawIP6Domains) {
		return false
	}

	var (
		domainSet    = map[ipnet.Type]map[string]bool{ipnet.IP4: {}, ipnet.IP6: {}}
		duplicateSet = map[string]bool{}
		ip4Domains   = make([]api.FQDN, 0, len(rawDomains)+len(rawIP4Domains))
		ip6Domains   = make([]api.FQDN, 0, len(rawDomains)+len(rawIP6Domains))
	)

	for _, domain := range rawDomains {
		if domainSet[ipnet.IP4][domain] || domainSet[ipnet.IP6][domain] {
			duplicateSet[domain] = true
			continue
		}

		domainSet[ipnet.IP4][domain] = true
		domainSet[ipnet.IP6][domain] = true
		ip4Domains = append(ip4Domains, api.FQDN(domain))
		ip6Domains = append(ip6Domains, api.FQDN(domain))
	}

	for _, domain := range rawIP4Domains {
		if domainSet[ipnet.IP4][domain] {
			duplicateSet[domain] = true
			continue
		}

		domainSet[ipnet.IP4][domain] = true
		ip4Domains = append(ip4Domains, api.FQDN(domain))
	}

	for _, domain := range rawIP6Domains {
		if domainSet[ipnet.IP6][domain] {
			duplicateSet[domain] = true
			continue
		}

		domainSet[ipnet.IP6][domain] = true
		ip6Domains = append(ip6Domains, api.FQDN(domain))
	}

	if !quiet {
		if len(duplicateSet) > 0 {
			duplicates := make([]string, 0, len(duplicateSet))
			for domain := range duplicateSet {
				duplicates = append(duplicates, domain)
			}
			log.Printf("🤔 Found duplicates of these domains: %v", duplicates)
		}
	}

	(*field)[ipnet.IP4] = ip4Domains
	(*field)[ipnet.IP6] = ip6Domains

	return true
}

func readPolicies(quiet quiet.Quiet, field *map[ipnet.Type]detector.Policy) bool {
	ip4Policy := (*field)[ipnet.IP4]
	ip6Policy := (*field)[ipnet.IP6]

	if !ReadPolicy(quiet, ipnet.IP4, "IP4_POLICY", &ip4Policy) {
		return false
	}

	if !ReadPolicy(quiet, ipnet.IP6, "IP6_POLICY", &ip6Policy) {
		return false
	}

	if !ip4Policy.IsManaged() && !ip6Policy.IsManaged() {
		log.Printf("😡 Both IPv4 and IPv6 are unmanaged.")
		return false
	}

	(*field)[ipnet.IP4] = ip4Policy
	(*field)[ipnet.IP6] = ip6Policy

	return true
}

func PrintConfig(c *Config) {
	log.Printf("🔧 Policies:")
	log.Printf("   🔸 IPv4 policy:      %v", c.Policy[ipnet.IP4])
	if c.Policy[ipnet.IP4].IsManaged() {
		log.Printf("   🔸 IPv4 domains:     %v", c.Domains[ipnet.IP4])
	}
	log.Printf("   🔸 IPv6 policy:      %v", c.Policy[ipnet.IP6])
	if c.Policy[ipnet.IP6].IsManaged() {
		log.Printf("   🔸 IPv6 domains:     %v", c.Domains[ipnet.IP6])
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
}

func (c *Config) ReadEnv() bool { //nolint:cyclop
	if !ReadQuiet("QUIET", &c.Quiet) {
		return false
	}

	if c.Quiet {
		log.Printf("🔇 Quiet mode enabled.")
	}

	if !readAuth(c.Quiet, &c.Auth) ||
		!readPolicies(c.Quiet, &c.Policy) ||
		!readDomains(c.Quiet, &c.Domains) ||
		!ReadCron(c.Quiet, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(c.Quiet, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(c.Quiet, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(c.Quiet, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadNonnegInt(c.Quiet, "TTL", (*int)(&c.TTL)) ||
		!ReadBool(c.Quiet, "PROXIED", &c.Proxied) ||
		!ReadNonnegDuration(c.Quiet, "DETECTION_TIMEOUT", &c.DetectionTimeout) {
		return false
	}

	return true
}

func (c *Config) checkUselessDomains() {
	var (
		domainSet    = map[ipnet.Type]map[string]bool{ipnet.IP4: {}, ipnet.IP6: {}}
		unionSet     = map[string]bool{}
		intersectSet = map[string]bool{}
	)
	// calculate domainSet[IP4], domainSet[IP6], and unionSet
	for ipNet, domains := range c.Domains {
		for _, domain := range domains {
			domainString := domain.String()
			domainSet[ipNet][domainString] = true
			unionSet[domainString] = true
		}
	}

	// calculate intersectSet
	for domain := range unionSet {
		intersectSet[domain] = domainSet[ipnet.IP4][domain] && domainSet[ipnet.IP6][domain]
	}

	for ipNet := range c.Domains {
		if !c.Policy[ipNet].IsManaged() {
			for domain := range domainSet[ipNet] {
				if !intersectSet[domain] {
					log.Printf("😡 Domain %v is ignored because it is only for %v but %v is unmanaged.", domain, ipNet, ipNet)
				}
			}
		}
	}
}

func (c *Config) Normalize() bool {
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		log.Printf("😡 No domains were specified.")
		return false
	}

	// change useless policies to
	for ipNet, domains := range c.Domains {
		if len(domains) == 0 && c.Policy[ipNet].IsManaged() {
			c.Policy[ipNet] = &detector.Unmanaged{}
			log.Printf(`🤔 IP%v_POLICY was changed to "%v" because no domains were set for %v.`,
				ipNet.Int(), c.Policy[ipNet], ipNet)
		}
	}

	if !c.Policy[ipnet.IP4].IsManaged() && !c.Policy[ipnet.IP6].IsManaged() {
		log.Printf("😡 Both IPv4 and IPv6 are unmanaged.")
		return false
	}

	c.checkUselessDomains()

	return true
}
