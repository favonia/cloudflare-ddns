package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

type Config struct {
	Auth             api.Auth
	Provider         map[ipnet.Type]provider.Provider
	Domains          map[ipnet.Type][]domain.Domain
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTLTemplate      string
	TTL              map[domain.Domain]api.TTL
	ProxiedTemplate  string
	Proxied          map[domain.Domain]bool
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
		Domains: map[ipnet.Type][]domain.Domain{
			ipnet.IP4: nil,
			ipnet.IP6: nil,
		},
		UpdateCron:       cron.MustNew("@every 5m"),
		UpdateOnStart:    true,
		DeleteOnStop:     false,
		CacheExpiration:  time.Hour * 6, //nolint:gomnd
		TTLTemplate:      "1",
		TTL:              map[domain.Domain]api.TTL{},
		ProxiedTemplate:  "false",
		Proxied:          map[domain.Domain]bool{},
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
func deduplicate(list []domain.Domain) []domain.Domain {
	domain.SortDomains(list)

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

func ReadDomainMap(ppfmt pp.PP, field *map[ipnet.Type][]domain.Domain) bool {
	var domains, ip4Domains, ip6Domains []domain.Domain

	if !ReadDomains(ppfmt, "DOMAINS", &domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &ip4Domains) ||
		!ReadDomains(ppfmt, "IP6_DOMAINS", &ip6Domains) {
		return false
	}

	ip4Domains = deduplicate(append(ip4Domains, domains...))
	ip6Domains = deduplicate(append(ip6Domains, domains...))

	*field = map[ipnet.Type][]domain.Domain{
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

func ParseProxied(ppfmt pp.PP, domain domain.Domain, val string) (bool, bool) {
	res, err := strconv.ParseBool(strings.TrimSpace(val))

	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "Proxy setting of %s (%q) is not a boolean value: %v", domain.Describe(), val, err)
		return false, false

	default:
		return res, true
	}
}

// ParseTTL turns a string into a valid TTL value.
//
// According to [API documentation], the valid range is 1 (auto) and [60, 86400].
// According to [DNS documentation], the valid range is "Auto" and [30, 86400].
// We thus accept the union of both ranges---1 (auto) and [30, 86400].
//
// [API documentation] https://api.cloudflare.com/#dns-records-for-a-zone-create-dns-record
// [DNS documentation] https://developers.cloudflare.com/dns/manage-dns-records/reference/ttl
func ParseTTL(ppfmt pp.PP, domain domain.Domain, val string) (api.TTL, bool) {
	val = strings.TrimSpace(val)
	res, err := strconv.Atoi(val)

	switch {
	case err != nil:
		ppfmt.Errorf(pp.EmojiUserError, "TTL of %s (%q) is not a number: %v", domain.Describe(), val, err)
		return 0, false

	case res != 1 && (res < 30 || res > 86400):
		ppfmt.Errorf(pp.EmojiUserError, "TTL of %s (%d) should be 1 (auto) or between 30 and 86400", domain.Describe(), res)
		return 0, false

	default:
		return api.TTL(res), true
	}
}

func describeDomains(domains []domain.Domain) string {
	if len(domains) == 0 {
		return "(none)"
	}

	descriptions := make([]string, 0, len(domains))
	for _, domain := range domains {
		descriptions = append(descriptions, domain.Describe())
	}
	return strings.Join(descriptions, ", ")
}

func inverseMap[K comparable](m map[domain.Domain]K) ([]K, map[K][]domain.Domain) {
	inverse := map[K][]domain.Domain{}

	for dom, val := range m {
		inverse[val] = append(inverse[val], dom)
	}

	keys := make([]K, 0, len(inverse))
	for val := range inverse {
		domain.SortDomains(inverse[val])
		keys = append(keys, val)
	}

	return keys, inverse
}

func (c *Config) Print(ppfmt pp.PP) {
	if !ppfmt.IsEnabledFor(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.IncIndent()

	inner := ppfmt.IncIndent()

	ppfmt.Infof(pp.EmojiConfig, "Policies:")
	inner.Infof(pp.EmojiBullet, "IPv4 provider:             %s", provider.Name(c.Provider[ipnet.IP4]))
	if c.Provider[ipnet.IP4] != nil {
		inner.Infof(pp.EmojiBullet, "IPv4 domains:              %s", describeDomains(c.Domains[ipnet.IP4]))
	}
	inner.Infof(pp.EmojiBullet, "IPv6 provider:             %s", provider.Name(c.Provider[ipnet.IP6]))
	if c.Provider[ipnet.IP6] != nil {
		inner.Infof(pp.EmojiBullet, "IPv6 domains:              %s", describeDomains(c.Domains[ipnet.IP6]))
	}

	ppfmt.Infof(pp.EmojiConfig, "Scheduling:")
	inner.Infof(pp.EmojiBullet, "Timezone:                  %s", cron.DescribeLocation(time.Local))
	inner.Infof(pp.EmojiBullet, "Update frequency:          %v", c.UpdateCron)
	inner.Infof(pp.EmojiBullet, "Update on start?           %t", c.UpdateOnStart)
	inner.Infof(pp.EmojiBullet, "Delete on stop?            %t", c.DeleteOnStop)
	inner.Infof(pp.EmojiBullet, "Cache expiration:          %v", c.CacheExpiration)

	ppfmt.Infof(pp.EmojiConfig, "New DNS records:")
	{
		vals, inverseMap := inverseMap[api.TTL](c.TTL)
		api.SortTTLs(vals)
		for _, val := range vals {
			inner.Infof(
				pp.EmojiBullet,
				"%-26s %s",
				fmt.Sprintf("Domains with TTL %s:", val.Describe()),
				describeDomains(inverseMap[val]),
			)
		}
	}
	{
		_, inverseMap := inverseMap[bool](c.Proxied)
		inner.Infof(pp.EmojiBullet, "Proxied domains:           %s", describeDomains(inverseMap[true]))
		inner.Infof(pp.EmojiBullet, "Non-proxied domains:       %s", describeDomains(inverseMap[false]))
	}

	ppfmt.Infof(pp.EmojiConfig, "Timeouts:")
	inner.Infof(pp.EmojiBullet, "IP detection:              %v", c.DetectionTimeout)
	inner.Infof(pp.EmojiBullet, "Record updating:           %v", c.UpdateTimeout)

	if len(c.Monitors) > 0 {
		ppfmt.Infof(pp.EmojiConfig, "Monitors:")
		for _, m := range c.Monitors {
			inner.Infof(pp.EmojiBullet, "%-26s (URL redacted)", m.DescribeService()+":")
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
		!ReadString(ppfmt, "TTL", &c.TTLTemplate) ||
		!ReadString(ppfmt, "PROXIED", &c.ProxiedTemplate) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadHealthChecksURL(ppfmt, "HEALTHCHECKS", &c.Monitors) {
		return false
	}

	return true
}

// NormalizeDomains normalizes the fields Provider, TTL and Proxied.
// When errors are reported, the original configuration remain unchanged.
//
//nolint:funlen,gocognit,cyclop
func (c *Config) NormalizeDomains(ppfmt pp.PP) bool {
	// New maps
	providerMap := map[ipnet.Type]provider.Provider{}
	ttlMap := map[domain.Domain]api.TTL{}
	proxiedMap := map[domain.Domain]bool{}
	activeDomainSet := map[domain.Domain]bool{}

	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "No domains were specified")
		return false
	}

	// fill in providerMap and activeDomainSet
	for ipNet, domains := range c.Domains {
		if c.Provider[ipNet] == nil {
			continue
		}

		if len(domains) == 0 {
			ppfmt.Warningf(pp.EmojiUserWarning, "IP%d_PROVIDER was changed to %q because no domains were set for %s",
				ipNet.Int(), provider.Name(nil), ipNet.Describe())

			continue
		}

		providerMap[ipNet] = c.Provider[ipNet]
		for _, domain := range domains {
			activeDomainSet[domain] = true
		}
	}

	// check if all policies are none
	if providerMap[ipnet.IP4] == nil && providerMap[ipnet.IP6] == nil {
		ppfmt.Errorf(pp.EmojiUserError, "Both IPv4 and IPv6 are disabled")
		return false
	}

	// check if some domains are unused
	for ipNet, domains := range c.Domains {
		if providerMap[ipNet] != nil {
			continue
		}

		for _, domain := range domains {
			if activeDomainSet[domain] {
				continue
			}

			ppfmt.Warningf(pp.EmojiUserWarning,
				"Domain %q is ignored because it is only for %s but %s is disabled",
				domain.Describe(), ipNet.Describe(), ipNet.Describe())
		}
	}

	// fill in ttlMap and proxyMap
	for dom := range activeDomainSet {
		{
			ttlString, ok := domain.ExecTemplate(ppfmt, c.TTLTemplate, dom)
			if !ok {
				return false
			}
			ttl, ok := ParseTTL(ppfmt, dom, ttlString)
			if !ok {
				return false
			}
			ttlMap[dom] = ttl
		}

		{
			proxiedString, ok := domain.ExecTemplate(ppfmt, c.ProxiedTemplate, dom)
			if !ok {
				return false
			}

			proxied, ok := ParseProxied(ppfmt, dom, proxiedString)
			if !ok {
				return false
			}
			proxiedMap[dom] = proxied
		}
	}

	c.Provider = providerMap
	c.TTL = ttlMap
	c.Proxied = proxiedMap

	return true
}
