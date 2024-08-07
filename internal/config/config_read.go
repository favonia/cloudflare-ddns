package config

import (
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
		!ReadString(ppfmt, "RECORD_COMMENT", &c.RecordComment) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) ||
		!ReadAndAppendHealthchecksURL(ppfmt, "HEALTHCHECKS", &c.Monitors) ||
		!ReadAndAppendUptimeKumaURL(ppfmt, "UPTIMEKUMA", &c.Monitors) ||
		!ReadAndAppendShoutrrrURL(ppfmt, "SHOUTRRR", &c.Notifiers) {
		return false
	}

	return true
}

// Normalize checks and normalizes the fields [Config.Provider], [Config.Proxied], and [Config.DeleteOnStop].
// When any error is reported, the original configuration remain unchanged.
//
//nolint:funlen
func (c *Config) Normalize(ppfmt pp.PP) bool {
	if ppfmt.IsEnabledFor(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.IncIndent()
	}

	// Step 1: is there something to do, and do wo have an auth?
	if c.Auth == nil {
		// if c.Auth == nil, the user should have been warned.
		ppfmt.Errorf(pp.EmojiImpossible, "Authorization info is missing, but this should be impossible")
		ppfmt.Errorf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new")
		return false
	}
	if len(c.Domains[ipnet.IP4]) == 0 && len(c.Domains[ipnet.IP6]) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "Nothing was specified in DOMAINS, IP4_DOMAINS, or IP6_DOMAINS")
		return false
	}

	// Part 2: check DELETE_ON_STOP and UpdateOnStart
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
				"DELETE_ON_STOP=true will immediately delete all domains when UPDATE_CRON=@once")
			return false
		}
	}

	// Step 3: normalize domains and providers
	//
	// Step 3.1: fill in providerMap and activeDomainSet
	providerMap := map[ipnet.Type]provider.Provider{}
	activeDomainSet := map[domain.Domain]bool{}
	for ipNet := range c.Provider {
		if c.Provider[ipNet] != nil {
			domains := c.Domains[ipNet]

			if len(domains) == 0 {
				ppfmt.Warningf(pp.EmojiUserWarning,
					"IP%d_PROVIDER was changed to %q because no domains use %s",
					ipNet.Int(), provider.Name(nil), ipNet.Describe())

				continue
			}

			providerMap[ipNet] = c.Provider[ipNet]
			for _, domain := range domains {
				activeDomainSet[domain] = true
			}
		}
	}

	// Step 3.2: check if all providers were turned off
	if providerMap[ipnet.IP4] == nil && providerMap[ipnet.IP6] == nil {
		ppfmt.Errorf(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
			provider.Name(nil))
		return false
	}

	// Step 3.3: check if some domains are unused
	for ipNet, domains := range c.Domains {
		if providerMap[ipNet] == nil {
			for _, domain := range domains {
				if activeDomainSet[domain] {
					continue
				}

				ppfmt.Warningf(pp.EmojiUserWarning,
					"Domain %q is ignored because it is only for %s but %s is disabled",
					domain.Describe(), ipNet.Describe(), ipNet.Describe())
			}
		}
	}

	// Step 4: fill in proxiedMap
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

	// Part 5: override the old values
	c.Provider = providerMap
	c.Proxied = proxiedMap

	return true
}
