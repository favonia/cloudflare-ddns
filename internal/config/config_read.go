package config

import (
	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// ReadEnv calls the relevant readers to read all relevant environment variables except
// - timezone (TZ)
// - privileges-related ones (PGID and PUID)
// - output-related ones (QUIET and EMOJI)
// One should subsequently call [Config.Normalize] to restore invariants across fields.
func (c *Config) ReadEnv(ppfmt pp.PP) bool {
	if ppfmt.IsShowing(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Reading settings . . .")
		ppfmt = ppfmt.Indent()
	}

	if !ReadAuth(ppfmt, &c.Auth) ||
		!ReadProviderMap(ppfmt, &c.Provider) ||
		!ReadIP6PrefixLen(ppfmt, "IP6_PREFIX_LEN", &c.IP6PrefixLen) ||
		!ReadDomainMap(ppfmt, &c.Domains, &c.IP6HostID) ||
		!ReadAndAppendWAFListNames(ppfmt, "WAF_LISTS", &c.WAFLists) ||
		!ReadCron(ppfmt, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(ppfmt, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(ppfmt, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(ppfmt, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadTTL(ppfmt, "TTL", &c.TTL) ||
		!ReadString(ppfmt, "PROXIED", &c.ProxiedTemplate) ||
		!ReadString(ppfmt, "RECORD_COMMENT", &c.RecordComment) ||
		!ReadString(ppfmt, "WAF_LIST_DESCRIPTION", &c.WAFListDescription) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadAndAppendHealthchecksURL(ppfmt, "HEALTHCHECKS", &c.Monitor) ||
		!ReadAndAppendUptimeKumaURL(ppfmt, "UPTIMEKUMA", &c.Monitor) ||
		!ReadAndAppendShoutrrrURL(ppfmt, "SHOUTRRR", &c.Notifier) {
		return false
	}

	return true
}

// Normalize checks and normalizes the fields [Config.Provider], [Config.Proxied], and [Config.DeleteOnStop].
// When any error is reported, the original configuration remain unchanged.
func (c *Config) Normalize(ppfmt pp.PP) bool {
	if ppfmt.IsShowing(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.Indent()
	}

	// Step 1: is there something to do?
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 && len(c.WAFLists) == 0 {
		ppfmt.Noticef(pp.EmojiUserError, "Nothing was specified in DOMAINS, IP4_DOMAINS, IP6_DOMAINS, or WAF_LISTS")
		return false
	}

	// Part 2: check DELETE_ON_STOP and UpdateOnStart
	if c.UpdateCron == nil {
		if !c.UpdateOnStart {
			ppfmt.Noticef(
				pp.EmojiUserError,
				"UPDATE_ON_START=false is incompatible with UPDATE_CRON=@once")
			return false
		}
		if c.DeleteOnStop {
			ppfmt.Noticef(
				pp.EmojiUserError,
				"DELETE_ON_STOP=true will immediately delete all domains and WAF lists when UPDATE_CRON=@once")
			return false
		}
	}

	// Step 3: normalize domains, host IDs, and providers
	//
	// Step 3.1: fill in providerMap and activeDomainSet
	providerMap := map[ipnet.Type]provider.Provider{}
	hostIDMap := map[domain.Domain]ipnet.HostID{}
	activeDomainSet := map[domain.Domain]bool{}
	wafListPrefixLen := map[ipnet.Type]int{}
	for ipNet, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			domains := c.Domains[ipNet]

			if len(domains) == 0 && len(c.WAFLists) == 0 {
				ppfmt.Noticef(pp.EmojiUserWarning,
					"IP%d_PROVIDER was changed to %q because no domains or WAF lists use %s",
					ipNet.Int(), provider.Name(nil), ipNet.Describe())

				continue
			}

			providerMap[ipNet] = p
			for _, domain := range domains {
				activeDomainSet[domain] = true
			}
		}
	}

	// Step 3.2: check if all providers were turned off
	if providerMap[ipnet.IP4] == nil && providerMap[ipnet.IP6] == nil {
		ppfmt.Noticef(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
			provider.Name(nil))
		return false
	}

	// Step 3.3: check about very small or large IP6PrefixLen used with WAF lists
	// Prefix length ranges for WAF: 12 to 64
	if len(c.WAFLists) > 0 {
		if providerMap[ipnet.IP4] != nil {
			wafListPrefixLen[ipnet.IP4] = 32
		}

		if providerMap[ipnet.IP6] != nil {
			switch {
			case c.IP6PrefixLen > 64: // 64
				ppfmt.Noticef(pp.EmojiUserWarning, "The detected IPv6 range in WAF lists will be forced to have a prefix length of 64 (instead of IP6_PREFIX_LEN (%d)) due to Cloudflare's limitations", c.IP6PrefixLen) //nolint:lll
				wafListPrefixLen[ipnet.IP6] = 64

			case c.IP6PrefixLen < 12: // 12
				ppfmt.Noticef(pp.EmojiUserError, "WAF lists do not support IPv6 ranges with a prefix length as small as IP6_PREFIX_LEN (%d); it must be at least 12", c.IP6PrefixLen) //nolint:lll
				return false

			default:
				wafListPrefixLen[ipnet.IP6] = c.IP6PrefixLen
			}
		}
	}

	// Step 3.4: normalize host IDs
	if providerMap[ipnet.IP6] != nil {
		for domain, hostID := range c.IP6HostID {
			if hostID == nil {
				continue
			}

			normalized, ok := hostID.Normalize(ppfmt, domain, c.IP6PrefixLen)
			if !ok {
				return false
			}

			hostIDMap[domain] = normalized
		}
	}

	// Step 3.5: check if some domains are unused
	for ipNet, domains := range ipnet.Bindings(c.Domains) {
		if providerMap[ipNet] == nil {
			for _, domain := range domains {
				if !activeDomainSet[domain] {
					ppfmt.Noticef(pp.EmojiUserWarning,
						"The domain %q is ignored because it is only for %s but %s is disabled",
						domain.Describe(), ipNet.Describe(), ipNet.Describe())
				}
			}
		}
	}

	// Step 3.6: check useless host IDs for IPv4-only domains
	// (that is, the domains are active but their host IDs are not used)
	if providerMap[ipnet.IP6] == nil {
		for domain, host := range c.IP6HostID {
			if activeDomainSet[domain] {
				ppfmt.Noticef(pp.EmojiUserWarning,
					"The host ID %q of %q is ignored because IPv6 is disabled",
					host.Describe(), domain.Describe())
			}
		}
	}

	// Step 3.6: check useless IP6PrefixLen
	if len(hostIDMap) == 0 && len(c.WAFLists) == 0 && c.IP6PrefixLen != 64 { // 64 is the default value
		ppfmt.Noticef(pp.EmojiUserWarning, "IP6_PREFIX_LEN=%d is ignored because no domains use IPv6 host IDs", c.IP6PrefixLen) //nolint:lll
	}

	// Step 4: regenerate proxiedMap from [Config.Proxied]
	proxiedMap := map[domain.Domain]bool{}
	if len(activeDomainSet) > 0 {
		proxiedPredicate, ok := domainexp.ParseExpression(ppfmt, "PROXIED", c.ProxiedTemplate)
		if !ok {
			return false
		}

		for dom := range activeDomainSet {
			proxiedMap[dom] = proxiedPredicate(dom)
		}
	}

	// Step 5: check if new parameters are unused
	if len(activeDomainSet) == 0 { // We are only updating WAF lists
		if c.TTL != api.TTLAuto {
			ppfmt.Noticef(pp.EmojiUserWarning, "TTL=%d is ignored because no domains will be updated", c.TTL.Int())
		}
		if c.ProxiedTemplate != "false" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"PROXIED=%s is ignored because no domains will be updated", c.ProxiedTemplate)
		}
		if c.RecordComment != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"RECORD_COMMENT=%s is ignored because no domains will be updated", c.RecordComment)
		}
	}
	if len(c.WAFLists) == 0 { // We are only updating domains
		if c.WAFListDescription != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"WAF_LIST_DESCRIPTION=%s is ignored because no WAF lists will be updated", c.WAFListDescription)
		}
	}

	// Final Part: override the old values
	c.Provider = providerMap
	c.IP6HostID = hostIDMap
	c.WAFListPrefixLen = wafListPrefixLen
	c.Proxied = proxiedMap

	return true
}
