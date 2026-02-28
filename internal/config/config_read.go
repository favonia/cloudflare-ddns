package config

import (
	"regexp"

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
		!ReadDomainMap(ppfmt, &c.Domains) ||
		!ReadWAFListNames(ppfmt, "WAF_LISTS", &c.WAFLists) ||
		!ReadCron(ppfmt, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(ppfmt, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(ppfmt, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(ppfmt, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadTTL(ppfmt, "TTL", &c.TTL) ||
		!ReadString(ppfmt, "PROXIED", &c.ProxiedTemplate) ||
		!ReadString(ppfmt, "RECORD_COMMENT", &c.RecordComment) ||
		!ReadString(ppfmt, "MANAGED_RECORDS_COMMENT_REGEX", &c.ManagedRecordsCommentRegexTemplate) ||
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

// Normalize checks and normalizes configuration invariants, including:
// - [Config.Provider] and [Config.Proxied] canonicalization
// - [Config.ManagedRecordsCommentRegex] compilation
// - scheduling consistency constraints such as [Config.DeleteOnStop]
//
// When any error is reported, the original configuration remains unchanged.
// On success, [Config.ManagedRecordsCommentRegex] is guaranteed non-nil.
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

	// Step 2.5: compile regex for selecting managed records.
	managedRecordsCommentRegex, err := regexp.Compile(c.ManagedRecordsCommentRegexTemplate)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError,
			"MANAGED_RECORDS_COMMENT_REGEX=%q is invalid: %v",
			c.ManagedRecordsCommentRegexTemplate, err)
		return false
	}
	if !managedRecordsCommentRegex.MatchString(c.RecordComment) {
		ppfmt.Noticef(pp.EmojiUserError,
			"RECORD_COMMENT=%q does not match MANAGED_RECORDS_COMMENT_REGEX=%q",
			c.RecordComment, c.ManagedRecordsCommentRegexTemplate)
		return false
	}

	// Step 3: normalize domains and providers
	//
	// Step 3.1: fill in providerMap and activeDomainSet
	providerMap := map[ipnet.Type]provider.Provider{}
	activeDomainSet := map[domain.Domain]bool{}
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

	// Step 3.3: check if some domains are unused
	for ipNet, domains := range ipnet.Bindings(c.Domains) {
		if providerMap[ipNet] == nil {
			for _, domain := range domains {
				if activeDomainSet[domain] {
					continue
				}

				ppfmt.Noticef(pp.EmojiUserWarning,
					"Domain %q is ignored because it is only for %s but %s is disabled",
					domain.Describe(), ipNet.Describe(), ipNet.Describe())
			}
		}
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
			ppfmt.Noticef(pp.EmojiUserWarning, "TTL=%v is ignored because no domains will be updated", c.TTL)
		}
		if c.ProxiedTemplate != "false" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"PROXIED=%s is ignored because no domains will be updated", c.ProxiedTemplate)
		}
		if c.RecordComment != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"RECORD_COMMENT=%s is ignored because no domains will be updated", c.RecordComment)
		}
		if c.ManagedRecordsCommentRegexTemplate != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"MANAGED_RECORDS_COMMENT_REGEX=%s is ignored because no domains will be updated",
				c.ManagedRecordsCommentRegexTemplate)
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
	c.Proxied = proxiedMap
	// Decision: do not optimize the empty template into nil.
	// Keep one canonical representation (a compiled match-all regex) to avoid
	// nil-vs-empty special cases in behavior and tests.
	c.ManagedRecordsCommentRegex = managedRecordsCommentRegex

	return true
}
