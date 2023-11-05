package config

import (
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// ReadEnv calls the relevant readers to read all relevant environment variables except TZ
// and update relevant fields. One should subsequently call [Config.NormalizeConfig]
// to maintain invariants across different fields.
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
		!ReadAndAppendHealthchecksURL(ppfmt, "HEALTHCHECKS", &c.Monitors) ||
		!ReadAndAppendUptimeKumaURL(ppfmt, "UPTIMEKUMA", &c.Monitors) ||
		!ReadAndAppendShoutrrrURL(ppfmt, "SHOUTRRR", &c.Notifiers) {
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

	// Part 1: check DELETE_ON_STOP and UpdateOnStart
	if c.UpdateCron == nil {
		if !c.UpdateOnStart {
			ppfmt.Errorf(
				pp.EmojiUserError,
				"UPDATE_ON_START=false is incompatible with UPDATE_CRON=@once")
			return false
		}
		if c.DeleteOnStop {
			ppfmt.Errorf(
				pp.EmojiUserError,
				"DELETE_ON_STOP=true will immediately delete all updated DNS records when UPDATE_CRON=@once")
			return false
		}
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
