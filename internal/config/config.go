package config

import (
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

type Config struct {
	Auth             api.Auth
	Provider         map[ipnet.Type]provider.Provider
	Domains          map[ipnet.Type][]api.Domain
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	DefaultProxied   bool
	ProxiedByDomain  map[api.Domain]bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
	Monitors         []monitor.Monitor
}

// Default gives default values.
func Default() *Config {
	return &Config{
		Auth: nil,
		Provider: map[ipnet.Type]provider.Provider{
			ipnet.IP4: provider.NewCloudflareTrace(),
			ipnet.IP6: provider.NewCloudflareTrace(),
		},
		Domains: map[ipnet.Type][]api.Domain{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTL:              api.TTL(1),
		DefaultProxied:   false,
		ProxiedByDomain:  map[api.Domain]bool{},
		UpdateTimeout:    time.Second * 30, //nolint:gomnd
		DetectionTimeout: time.Second * 5,  //nolint:gomnd
		Monitors:         nil,
	}
}

func readAuthToken(ppfmt pp.PP) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)

	// foolproof checks
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Errorf(pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN")
		return "", false
	}

	switch {
	case token != "" && tokenFile != "":
		ppfmt.Errorf(pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set")
		return "", false
	case token != "":
		return token, true
	case tokenFile != "":
		token, ok := file.ReadString(ppfmt, tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			ppfmt.Errorf(pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty")
			return "", false
		}

		return token, true
	default:
		ppfmt.Errorf(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE")
		return "", false
	}
}

func ReadAuth(ppfmt pp.PP, field *api.Auth) bool {
	token, ok := readAuthToken(ppfmt)
	if !ok {
		return false
	}

	accountID := Getenv("CF_ACCOUNT_ID")

	*field = &api.CloudflareAuth{Token: token, AccountID: accountID, BaseURL: ""}
	return true
}

// deduplicate always sorts and deduplicates the input list,
// returning true if elements are already distinct.
func deduplicate(list []api.Domain) []api.Domain {
	api.SortDomains(list)

	if len(list) == 0 {
		return list
	}

	j := 0
	for i := range list {
		if i == 0 || list[j] == list[i] {
			continue
		}
		j++
		list[j] = list[i]
	}

	if len(list) == j+1 {
		return list
	}

	return list[:j+1]
}

func ReadDomainMap(ppfmt pp.PP, field *map[ipnet.Type][]api.Domain) bool {
	var domains, ip4Domains, ip6Domains []api.Domain

	if !ReadDomains(ppfmt, "DOMAINS", &domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(ppfmt, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = deduplicate(append(ip4Domains, domains...))
	ip6Domains = deduplicate(append(ip6Domains, domains...))

	*field = map[ipnet.Type][]api.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}

	return true
}

func ReadProviderMap(ppfmt pp.PP, field *map[ipnet.Type]provider.Provider) bool {
	ip4Provider := (*field)[ipnet.IP4]
	ip6Provider := (*field)[ipnet.IP6]

	if !ReadProvider(ppfmt, "IP4_PROVIDER", "IP4_POLICY", &ip4Provider) ||
		!ReadProvider(ppfmt, "IP6_PROVIDER", "IP6_POLICY", &ip6Provider) {
		return false
	}

	*field = map[ipnet.Type]provider.Provider{
		ipnet.IP4: ip4Provider,
		ipnet.IP6: ip6Provider,
	}
	return true
}

func ReadTTL(ppfmt pp.PP, field *api.TTL) bool {
	if !ReadNonnegInt(ppfmt, "TTL", (*int)(field)) {
		return false
	}

	// According to [API documentation], the valid range is 1 (auto) and [60, 86400].
	// According to [DNS documentation], the valid range is "Auto" and [30, 86400].
	// We thus accept the union of both ranges---1 (auto) and [30, 86400].
	//
	// [API documentation] https://api.cloudflare.com/#dns-records-for-a-zone-create-dns-record
	// [DNS documentation] https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl

	if *field != 1 && (*field < 30 || *field > 86400) {
		ppfmt.Warningf(pp.EmojiUserWarning, "TTL value (%i) should be 1 (automatic) or between 30 and 86400", int(*field))
	}
	return true
}

func ReadProxiedByDomain(ppfmt pp.PP, field *map[api.Domain]bool) bool {
	var proxiedDomains, nonProxiedDomains []api.Domain

	if !ReadDomains(ppfmt, "PROXIED_DOMAINS", &proxiedDomains) ||
		!ReadDomains(ppfmt, "NON_PROXIED_DOMAINS", &nonProxiedDomains) {
		return false
	}

	proxiedDomains = deduplicate(proxiedDomains)
	nonProxiedDomains = deduplicate(nonProxiedDomains)

	if len(proxiedDomains) > 0 || len(nonProxiedDomains) > 0 {
		ppfmt.Warningf(pp.EmojiExperimental, "PROXIED_DOMAINS and NON_PROXIED_DOMAINS are experimental features")
		ppfmt.Warningf(pp.EmojiExperimental, "Please share your case at https://github.com/favonia/cloudflare-ddns/issues/199") //nolint:lll
		ppfmt.Warningf(pp.EmojiExperimental, "We might remove these features based on your (lack of) feedback")
	}

	// the new map to be created
	m := map[api.Domain]bool{}

	// all proxied domains
	for _, proxiedDomain := range proxiedDomains {
		m[proxiedDomain] = true
	}

	// non-proxied domains
	for _, nonProxiedDomain := range nonProxiedDomains {
		if proxied, ok := m[nonProxiedDomain]; ok && proxied {
			ppfmt.Errorf(pp.EmojiUserError,
				"Domain %q appeared in both PROXIED_DOMAINS and NON_PROXIED_DOMAINS",
				nonProxiedDomain.Describe())
			return false
		}

		m[nonProxiedDomain] = false
	}

	*field = m

	return true
}

func describeDomains(domains []api.Domain) string {
	if len(domains) == 0 {
		return "(none)"
	}

	descriptions := make([]string, 0, len(domains))
	for _, domain := range domains {
		descriptions = append(descriptions, domain.Describe())
	}
	return strings.Join(descriptions, ", ")
}

func (c *Config) Print(ppfmt pp.PP) {
	if !ppfmt.IsEnabledFor(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.IncIndent()

	inner := ppfmt.IncIndent()

	ppfmt.Infof(pp.EmojiConfig, "Policies:")
	inner.Infof(pp.EmojiBullet, "IPv4 provider:    %s", provider.Name(c.Provider[ipnet.IP4]))
	if c.Provider[ipnet.IP4] != nil {
		inner.Infof(pp.EmojiBullet, "IPv4 domains:     %s", describeDomains(c.Domains[ipnet.IP4]))
	}
	inner.Infof(pp.EmojiBullet, "IPv6 provider:    %s", provider.Name(c.Provider[ipnet.IP6]))
	if c.Provider[ipnet.IP6] != nil {
		inner.Infof(pp.EmojiBullet, "IPv6 domains:     %s", describeDomains(c.Domains[ipnet.IP6]))
	}

	ppfmt.Infof(pp.EmojiConfig, "Scheduling:")
	inner.Infof(pp.EmojiBullet, "Timezone:         %s", cron.DescribeLocation(time.Local))
	inner.Infof(pp.EmojiBullet, "Update frequency: %v", c.UpdateCron)
	inner.Infof(pp.EmojiBullet, "Update on start?  %t", c.UpdateOnStart)
	inner.Infof(pp.EmojiBullet, "Delete on stop?   %t", c.DeleteOnStop)
	inner.Infof(pp.EmojiBullet, "Cache expiration: %v", c.CacheExpiration)

	ppfmt.Infof(pp.EmojiConfig, "New DNS records:")
	inner.Infof(pp.EmojiBullet, "TTL:              %s", c.TTL.Describe())
	{
		proxiedMapping := map[bool][]api.Domain{}
		proxiedMapping[true] = make([]api.Domain, 0, len(c.ProxiedByDomain))
		proxiedMapping[false] = make([]api.Domain, 0, len(c.ProxiedByDomain))
		for domain, proxied := range c.ProxiedByDomain {
			proxiedMapping[proxied] = append(proxiedMapping[proxied], domain)
		}
		for b := range proxiedMapping {
			proxiedMapping[b] = deduplicate(proxiedMapping[b])
		}
		inner.Infof(pp.EmojiBullet, "Proxied:          %s", describeDomains(proxiedMapping[true]))
		inner.Infof(pp.EmojiBullet, "Non-proxied:      %s", describeDomains(proxiedMapping[false]))
	}

	ppfmt.Infof(pp.EmojiConfig, "Timeouts:")
	inner.Infof(pp.EmojiBullet, "IP detection:     %v", c.DetectionTimeout)
	inner.Infof(pp.EmojiBullet, "Record updating:  %v", c.UpdateTimeout)

	if len(c.Monitors) > 0 {
		ppfmt.Infof(pp.EmojiConfig, "Monitors:")
		for _, m := range c.Monitors {
			inner.Infof(pp.EmojiBullet, "%-17s (URL redacted)", m.DescribeService()+":")
		}
	}
}

func (c *Config) ReadEnv(ppfmt pp.PP) bool { //nolint:cyclop
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Reading settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if !ReadAuth(ppfmt, &c.Auth) ||
		!ReadProviderMap(ppfmt, &c.Provider) ||
		!ReadDomainMap(ppfmt, &c.Domains) ||
		!ReadCron(ppfmt, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(ppfmt, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(ppfmt, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(ppfmt, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadTTL(ppfmt, &c.TTL) ||
		!ReadBool(ppfmt, "PROXIED", &c.DefaultProxied) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadHealthChecksURL(ppfmt, "HEALTHCHECKS", &c.Monitors) ||
		!ReadProxiedByDomain(ppfmt, &c.ProxiedByDomain) {
		return false
	}

	return true
}

//nolint:funlen,gocognit,cyclop
func (c *Config) NormalizeDomains(ppfmt pp.PP) bool {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "No domains were specified")
		return false
	}

	// change useless policies to none
	for ipNet, domains := range c.Domains {
		if len(domains) == 0 && c.Provider[ipNet] != nil {
			c.Provider[ipNet] = nil
			ppfmt.Warningf(pp.EmojiUserWarning, "IP%d_PROVIDER was changed to %q because no domains were set for %s",
				ipNet.Int(), provider.Name(c.Provider[ipNet]), ipNet.Describe())
		}
	}

	// check if all policies are none
	if c.Provider[ipnet.IP4] == nil && c.Provider[ipnet.IP6] == nil {
		ppfmt.Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are disabled")
		return false
	}

	// domainSet is the set of managed domains.
	domainSet := map[api.Domain]bool{}
	for ipNet, domains := range c.Domains {
		if c.Provider[ipNet] != nil {
			for _, domain := range domains {
				domainSet[domain] = true
			}
		}
	}

	// check if some domains are unused
	for ipNet, domains := range c.Domains {
		if c.Provider[ipNet] == nil {
			for _, domain := range domains {
				if !domainSet[domain] {
					ppfmt.Warningf(pp.EmojiUserWarning,
						"Domain %q is ignored because it is only for %s but %s is disabled",
						domain.Describe(), ipNet.Describe(), ipNet.Describe())
				}
			}
		}
	}

	// fill in the default "proxied"
	if c.ProxiedByDomain == nil {
		c.ProxiedByDomain = map[api.Domain]bool{}
		ppfmt.Warningf(pp.EmojiImpossible,
			"Internal failure: ProxiedByDomain is re-initialized because it was nil",
		)
		ppfmt.Warningf(pp.EmojiImpossible,
			"Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new",
		)
	}
	for domain := range domainSet {
		if _, ok := c.ProxiedByDomain[domain]; !ok {
			c.ProxiedByDomain[domain] = c.DefaultProxied
		}
	}

	// check if some domain-specific "proxied" setting is not used
	envMap := map[bool]string{true: "PROXIED_DOMAINS", false: "NON_PROXIED_DOMAINS"}
	if len(c.ProxiedByDomain) > len(domainSet) {
		for domain, proxied := range c.ProxiedByDomain {
			if !domainSet[domain] {
				delete(c.ProxiedByDomain, domain)
				ppfmt.Warningf(pp.EmojiUserWarning,
					"Domain %q was listed in %s, but it is ignored because it is not managed by the updater",
					domain.Describe(), envMap[proxied])
			}
		}
	}

	return true
}
