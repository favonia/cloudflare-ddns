// Package config reads and parses configurations.
package config

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Config holds the configuration of the updater except for the timezone.
// (The timezone is handled directly by the standard library reading the TZ environment variable.)
type Config struct {
	Auth             api.Auth
	Provider         map[ipnet.Type]provider.Provider
	Domains          map[ipnet.Type][]domain.Domain
	UpdateCron       cron.Schedule
	UpdateOnStart    bool
	DeleteOnStop     bool
	CacheExpiration  time.Duration
	TTL              api.TTL
	ProxiedTemplate  string
	Proxied          map[domain.Domain]bool
	DetectionTimeout time.Duration
	UpdateTimeout    time.Duration
	Monitor          monitor.Monitor
}

// Default gives the default configuration.
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
		TTL:              api.TTLAuto,
		ProxiedTemplate:  "false",
		Proxied:          map[domain.Domain]bool{},
		UpdateTimeout:    time.Second * 30, //nolint:gomnd
		DetectionTimeout: time.Second * 5,  //nolint:gomnd
		Monitor:          monitor.Monitors{},
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

// ReadAuth reads environment variables CF_API_TOKEN, CF_API_TOKEN_FILE, and CF_ACCOUNT_ID
// and creates an [api.CloudflareAuth].
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
	return slices.Compact(list)
}

// ReadDomainMap reads environment variables DOMAINS, IP4_DOMAINS, and IP6_DOMAINS
// and consolidate the domains into a map.
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

// ReadProviderMap reads the environment variables IP4_PROVIDER and IP6_PROVIDER,
// with support of deprecated environment variables IP4_POLICY and IP6_POLICY.
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

func getInverseMap[V comparable](m map[domain.Domain]V) ([]V, map[V][]domain.Domain) {
	inverse := map[V][]domain.Domain{}

	for dom, val := range m {
		inverse[val] = append(inverse[val], dom)
	}

	vals := make([]V, 0, len(inverse))
	for val := range inverse {
		domain.SortDomains(inverse[val])
		vals = append(vals, val)
	}

	return vals, inverse
}

const itemTitleWidth = 24

// Print prints the Config on the screen.
func (c *Config) Print(ppfmt pp.PP) {
	if !ppfmt.IsEnabledFor(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.IncIndent()
	inner := ppfmt.IncIndent()

	section := func(title string) { ppfmt.Infof(pp.EmojiConfig, title) }
	item := func(title string, format string, values ...any) {
		inner.Infof(pp.EmojiBullet, "%-*s %s", itemTitleWidth, title, fmt.Sprintf(format, values...))
	}

	section("Domains and IP providers:")
	if c.Provider[ipnet.IP4] != nil {
		item("IPv4 domains:", "%s", describeDomains(c.Domains[ipnet.IP4]))
		item("IPv4 provider:", "%s", provider.Name(c.Provider[ipnet.IP4]))
	}
	if c.Provider[ipnet.IP6] != nil {
		item("IPv6 domains:", "%s", describeDomains(c.Domains[ipnet.IP6]))
		item("IPv6 provider:", "%s", provider.Name(c.Provider[ipnet.IP6]))
	}

	section("Scheduling:")
	item("Timezone:", "%s", cron.DescribeLocation(time.Local))
	item("Update frequency:", "%s", cron.DescribeSchedule(c.UpdateCron))
	item("Update on start?", "%t", c.UpdateOnStart)
	item("Delete on stop?", "%t", c.DeleteOnStop)
	item("Cache expiration:", "%v", c.CacheExpiration)

	section("New DNS records:")
	item("TTL:", "%s", c.TTL.Describe())
	if len(c.Proxied) > 0 {
		_, inverseMap := getInverseMap(c.Proxied)
		item("Proxied domains:", "%s", describeDomains(inverseMap[true]))
		item("Unproxied domains:", "%s", describeDomains(inverseMap[false]))
	}

	section("Timeouts:")
	item("IP detection:", "%v", c.DetectionTimeout)
	item("Record updating:", "%v", c.UpdateTimeout)

	var monitors [][2]string
	if c.Monitor != nil {
		c.Monitor.Describe(func(service, params string) {
			monitors = append(monitors, [2]string{service, params})
		})
	}
	if len(monitors) > 0 {
		section("Monitors:")
		for _, m := range monitors {
			item(m[0]+":", "%s", m[1])
		}
	}
}

// ReadEnv calls the relevant readers to read all relevant environment variables except TZ
// and update relevant fields. One should subsequently call [Config.NormalizeConfig] to maintain
// invariants across different fields.
func (c *Config) ReadEnv(ppfmt pp.PP) bool {
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
		!ReadTTL(ppfmt, "TTL", &c.TTL) ||
		!ReadString(ppfmt, "PROXIED", &c.ProxiedTemplate) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadHealthchecksURL(ppfmt, "HEALTHCHECKS", &c.Monitor) {
		return false
	}

	return true
}

// NormalizeConfig checks and normalizes the fields [Config.Provider], [Config.Proxied], and [Config.DeleteOnStop].
// When any error is reported, the original configuration remain unchanged.
//
//nolint:funlen
func (c *Config) NormalizeConfig(ppfmt pp.PP) bool {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	// Part 1: check DELETE_ON_STOP
	if c.UpdateCron == nil && c.DeleteOnStop {
		ppfmt.Errorf(
			pp.EmojiUserError,
			"DELETE_ON_STOP=true will immediately delete all DNS records when UPDATE_CRON=@disabled")
		return false
	}

	// Part 2: normalize domain maps
	// New domain maps
	providerMap := map[ipnet.Type]provider.Provider{}
	proxiedMap := map[domain.Domain]bool{}
	activeDomainSet := map[domain.Domain]bool{}

	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "No domains were specified in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS")
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

	// check if all providers are nil
	if providerMap[ipnet.IP4] == nil && providerMap[ipnet.IP6] == nil {
		ppfmt.Errorf(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
			provider.Name(nil))
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

	// fill in proxyMap
	proxiedPred, ok := domainexp.ParseExpression(ppfmt, "PROXIED", c.ProxiedTemplate)
	if !ok {
		return false
	}
	for dom := range activeDomainSet {
		proxiedMap[dom] = proxiedPred(dom)
	}

	// Part 3: override the old values
	c.Provider = providerMap
	c.Proxied = proxiedMap

	return true
}
