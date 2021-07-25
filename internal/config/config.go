package config

import (
	"fmt"
	"sort"
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
		fmt.Printf("😡 You need to provide a real API token as CF_API_TOKEN.\n")
		return "", false
	}

	switch {
	case token != "" && tokenFile != "":
		fmt.Printf("😡 Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set.\n")
		return "", false
	case token != "":
		return token, true
	case tokenFile != "":
		token, ok := file.ReadFileAsString(tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			fmt.Printf("😡 The token in the file specified by CF_API_TOKEN_FILE is empty.\n")
			return "", false
		}

		return token, true
	default:
		fmt.Printf("😡 Needs either CF_API_TOKEN or CF_API_TOKEN_FILE.\n")
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

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list *[]string) bool {
	sort.Strings(*list)

	if len(*list) == 0 {
		return true
	}

	j := 0
	for i := range *list {
		if i == 0 || (*list)[j] == (*list)[i] {
			continue
		}
		j++
		(*list)[j] = (*list)[i]
	}

	if len(*list) == j+1 {
		return true
	}

	*list = (*list)[:j+1]
	return false
}

func readDomains(quiet quiet.Quiet, field map[ipnet.Type][]api.FQDN) bool {
	var domains, ip4Domains, ip6Domains []string

	if !ReadDomains(quiet, "DOMAINS", &domains) ||
		!ReadDomains(quiet, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(quiet, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = append(ip4Domains, domains...)
	ip6Domains = append(ip6Domains, domains...)

	ip4HasDuplicates := deduplicate(&ip4Domains)
	ip6HasDuplicates := deduplicate(&ip6Domains)

	if ip4HasDuplicates || ip6HasDuplicates {
		if !quiet {
			fmt.Printf("🤔 Duplicate domains are ignored.\n")
		}
	}

	field[ipnet.IP4] = make([]api.FQDN, 0, len(ip4Domains))
	for _, domain := range ip4Domains {
		field[ipnet.IP4] = append(field[ipnet.IP4], api.FQDN(domain))
	}

	field[ipnet.IP6] = make([]api.FQDN, 0, len(ip6Domains))
	for _, domain := range ip6Domains {
		field[ipnet.IP6] = append(field[ipnet.IP6], api.FQDN(domain))
	}

	return true
}

func readPolicies(quiet quiet.Quiet, field map[ipnet.Type]detector.Policy) bool {
	ip4Policy := field[ipnet.IP4]
	ip6Policy := field[ipnet.IP6]

	if !ReadPolicy(quiet, ipnet.IP4, "IP4_POLICY", &ip4Policy) {
		return false
	}

	if !ReadPolicy(quiet, ipnet.IP6, "IP6_POLICY", &ip6Policy) {
		return false
	}

	field[ipnet.IP4] = ip4Policy
	field[ipnet.IP6] = ip6Policy
	return true
}

func PrintConfig(c *Config) {
	fmt.Printf("🔧 Policies:\n")
	fmt.Printf("   🔸 IPv4 policy:      %v\n", c.Policy[ipnet.IP4])
	if c.Policy[ipnet.IP4].IsManaged() {
		fmt.Printf("   🔸 IPv4 domains:     %v\n", c.Domains[ipnet.IP4])
	}
	fmt.Printf("   🔸 IPv6 policy:      %v\n", c.Policy[ipnet.IP6])
	if c.Policy[ipnet.IP6].IsManaged() {
		fmt.Printf("   🔸 IPv6 domains:     %v\n", c.Domains[ipnet.IP6])
	}
	fmt.Printf("🔧 Timing:\n")
	fmt.Printf("   🔸 Update frequency: %v\n", c.UpdateCron)
	fmt.Printf("   🔸 Update on start?  %t\n", c.UpdateOnStart)
	fmt.Printf("   🔸 Delete on stop?   %t\n", c.DeleteOnStop)
	fmt.Printf("   🔸 Cache expiration: %v\n", c.CacheExpiration)
	fmt.Printf("🔧 New DNS records:\n")
	fmt.Printf("   🔸 TTL:              %v\n", c.TTL)
	fmt.Printf("   🔸 Proxied:          %t\n", c.Proxied)
	fmt.Printf("🔧 Timeouts\n")
	fmt.Printf("   🔸 IP detection:     %v\n", c.DetectionTimeout)
}

func (c *Config) ReadEnv() bool { //nolint:cyclop
	if !ReadQuiet("QUIET", &c.Quiet) {
		return false
	}

	if c.Quiet {
		fmt.Printf("🔇 Quiet mode enabled.\n")
	}

	if !readAuth(c.Quiet, &c.Auth) ||
		!readPolicies(c.Quiet, c.Policy) ||
		!readDomains(c.Quiet, c.Domains) ||
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
					fmt.Printf("😡 Domain %v is ignored because it is only for %v but %v is unmanaged.\n", domain, ipNet, ipNet)
				}
			}
		}
	}
}

func (c *Config) Normalize() bool {
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		fmt.Printf("😡 No domains were specified.\n")
		return false
	}

	// change useless policies to
	for ipNet, domains := range c.Domains {
		if len(domains) == 0 && c.Policy[ipNet].IsManaged() {
			c.Policy[ipNet] = &detector.Unmanaged{}
			fmt.Printf("🤔 IP%v_POLICY was changed to %q because no domains were set for %v.\n",
				ipNet.Int(), c.Policy[ipNet], ipNet)
		}
	}

	if !c.Policy[ipnet.IP4].IsManaged() && !c.Policy[ipnet.IP6].IsManaged() {
		fmt.Printf("😡 Both IPv4 and IPv6 are unmanaged.\n")
		return false
	}

	c.checkUselessDomains()

	return true
}
