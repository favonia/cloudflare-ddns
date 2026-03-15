package config

import (
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/sliceutil"
)

// Keep advisory value previews short in ignored-setting warnings while preserving
// exact quoted values for validation and mismatch diagnostics.
const ignoredSettingValuePreviewLimit = 48

func describeIgnoredSettingValuePreview(value string) string {
	return pp.QuotePreview(value, ignoredSettingValuePreviewLimit)
}

func describeProviderSettingValuePreview(p provider.Provider) string {
	return describeIgnoredSettingValuePreview(provider.Name(p))
}

// ReadEnv calls the relevant readers to parse all relevant environment variables except
// - timezone (TZ)
// - privileges-related ones (PGID and PUID)
// - output-related ones (QUIET and EMOJI)
// - reporter-related ones (HEALTHCHECKS, SHOUTRRR, and UPTIMEKUMA)
//
// It only parses updater settings into [RawConfig]. Call [RawConfig.BuildConfig] afterwards to
// validate cross-field invariants and derive the updater runtime configs.
// Reporter construction is handled separately by [SetupReporters].
func (c *RawConfig) ReadEnv(ppfmt pp.PP) bool {
	if ppfmt.IsShowing(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Reading settings . . .")
		ppfmt = ppfmt.Indent()
	}

	if !ReadAuth(ppfmt, &c.Auth) ||
		!ReadProviderMap(ppfmt, &c.Provider) ||
		!ReadDomains(ppfmt, "DOMAINS", &c.Domains) ||
		!ReadDomains(ppfmt, "IP4_DOMAINS", &c.IP4Domains) ||
		!ReadDomains(ppfmt, "IP6_DOMAINS", &c.IP6Domains) ||
		!ReadWAFListNames(ppfmt, "WAF_LISTS", &c.WAFLists) ||
		!ReadCron(ppfmt, "UPDATE_CRON", &c.UpdateCron) ||
		!ReadBool(ppfmt, "UPDATE_ON_START", &c.UpdateOnStart) ||
		!ReadBool(ppfmt, "DELETE_ON_STOP", &c.DeleteOnStop) ||
		!ReadNonnegDuration(ppfmt, "CACHE_EXPIRATION", &c.CacheExpiration) ||
		!ReadTTL(ppfmt, "TTL", &c.TTL) ||
		!ReadString(ppfmt, "PROXIED", &c.ProxiedExpression) ||
		!ReadString(ppfmt, "RECORD_COMMENT", &c.RecordComment) ||
		!ReadString(ppfmt, "MANAGED_RECORDS_COMMENT_REGEX", &c.ManagedRecordsCommentRegex) ||
		!ReadString(ppfmt, "WAF_LIST_DESCRIPTION", &c.WAFListDescription) ||
		!ReadString(ppfmt, "WAF_LIST_ITEM_COMMENT", &c.WAFListItemComment) ||
		!ReadString(ppfmt, "MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX", &c.ManagedWAFListItemsCommentRegex) ||
		!ReadNonnegDuration(ppfmt, "DETECTION_TIMEOUT", &c.DetectionTimeout) ||
		!ReadNonnegDuration(ppfmt, "UPDATE_TIMEOUT", &c.UpdateTimeout) {
		return false
	}

	return true
}

func normalizeDomainMap(raw *RawConfig) map[ipnet.Family][]domain.Domain {
	var ip4Domains []domain.Domain
	ip4Domains = append(ip4Domains, raw.IP4Domains...)
	ip4Domains = append(ip4Domains, raw.Domains...)
	ip4Domains = sliceutil.SortAndCompact(ip4Domains, domain.CompareDomain)

	var ip6Domains []domain.Domain
	ip6Domains = append(ip6Domains, raw.IP6Domains...)
	ip6Domains = append(ip6Domains, raw.Domains...)
	ip6Domains = sliceutil.SortAndCompact(ip6Domains, domain.CompareDomain)

	return map[ipnet.Family][]domain.Domain{
		ipnet.IP4: ip4Domains,
		ipnet.IP6: ip6Domains,
	}
}

// BuildConfig checks and derives configuration invariants, including:
// - provider and domain canonicalization
// - [HandleConfig.Options]'s managed-record selector compilation
// - scheduling consistency constraints such as [LifecycleConfig.DeleteOnStop]
//
// It is intentionally config-only: heartbeat and notifier services are built by
// [SetupReporters], not by this method.
//
// When any error is reported, the original [RawConfig] remains unchanged.
func (c *RawConfig) BuildConfig(ppfmt pp.PP) (*BuiltConfig, bool) {
	if ppfmt.IsShowing(pp.Info) {
		ppfmt.Infof(pp.EmojiEnvVars, "Checking settings . . .")
		ppfmt = ppfmt.Indent()
	}

	domains := normalizeDomainMap(c)

	// Step 1: is there something to do?
	if len(domains[ipnet.IP4]) == 0 && len(domains[ipnet.IP6]) == 0 && len(c.WAFLists) == 0 {
		ppfmt.Noticef(pp.EmojiUserError, "Nothing was specified in DOMAINS, IP4_DOMAINS, IP6_DOMAINS, or WAF_LISTS")
		return nil, false
	}

	// Part 2: check DELETE_ON_STOP and UpdateOnStart
	if c.UpdateCron == nil {
		if !c.UpdateOnStart {
			ppfmt.Noticef(
				pp.EmojiUserError,
				"UPDATE_ON_START=false is incompatible with UPDATE_CRON=@once")
			return nil, false
		}
		if c.DeleteOnStop {
			ppfmt.Noticef(
				pp.EmojiUserError,
				"DELETE_ON_STOP=true with UPDATE_CRON=@once would immediately delete managed domains and WAF content")
			return nil, false
		}
	}

	// Step 2.5: compile the ownership selectors for managed DNS records and WAF list items.
	managedRecordsCommentRegex, err := regexp.Compile(c.ManagedRecordsCommentRegex)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError,
			"MANAGED_RECORDS_COMMENT_REGEX=%q is invalid: %v",
			c.ManagedRecordsCommentRegex, err)
		return nil, false
	}
	if !managedRecordsCommentRegex.MatchString(c.RecordComment) {
		ppfmt.Noticef(pp.EmojiUserError,
			"RECORD_COMMENT=%q does not match MANAGED_RECORDS_COMMENT_REGEX=%q",
			c.RecordComment, c.ManagedRecordsCommentRegex)
		return nil, false
	}
	managedWAFListItemsCommentRegex, err := regexp.Compile(c.ManagedWAFListItemsCommentRegex)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError,
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=%q is invalid: %v",
			c.ManagedWAFListItemsCommentRegex, err)
		return nil, false
	}
	if !managedWAFListItemsCommentRegex.MatchString(c.WAFListItemComment) {
		ppfmt.Noticef(pp.EmojiUserError,
			"WAF_LIST_ITEM_COMMENT=%q does not match MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=%q",
			c.WAFListItemComment, c.ManagedWAFListItemsCommentRegex)
		return nil, false
	}

	// Step 3: normalize domains and providers.
	providerMap := map[ipnet.Family]provider.Provider{}
	activeDomainSet := map[domain.Domain]bool{}
	for ipFamily, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			ipNetDomains := domains[ipFamily]

			if len(ipNetDomains) == 0 && len(c.WAFLists) == 0 {
				ppfmt.Noticef(pp.EmojiUserWarning,
					"IP%d_PROVIDER (%s) is ignored because no domains or WAF lists use %s",
					ipFamily.Int(), describeProviderSettingValuePreview(p), ipFamily.Describe())

				continue
			}

			providerMap[ipFamily] = p
			for _, domain := range ipNetDomains {
				activeDomainSet[domain] = true
			}
		}
	}

	// Step 3.2: check if all providers were turned off.
	if providerMap[ipnet.IP4] == nil && providerMap[ipnet.IP6] == nil {
		ppfmt.Noticef(pp.EmojiUserError, "Nothing to update because both IP4_PROVIDER and IP6_PROVIDER are %q",
			provider.Name(nil))
		return nil, false
	}
	if provider.IsStaticEmpty(providerMap[ipnet.IP4]) && provider.IsStaticEmpty(providerMap[ipnet.IP6]) {
		switch {
		case len(activeDomainSet) > 0 && len(c.WAFLists) > 0:
			ppfmt.Noticef(pp.EmojiUserWarning,
				`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed DNS records and WAF IP items for the configured scope`)
		case len(activeDomainSet) > 0:
			ppfmt.Noticef(pp.EmojiUserWarning,
				`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed DNS records for the configured domains`)
		case len(c.WAFLists) > 0:
			ppfmt.Noticef(pp.EmojiUserWarning,
				`Both IP4_PROVIDER and IP6_PROVIDER are "static.empty"; this updater will clear managed WAF IP items for the configured lists`)
		}
	}

	// Step 3.3: check if some domains are unused.
	for ipFamily, ipNetDomains := range ipnet.Bindings(domains) {
		if providerMap[ipFamily] == nil {
			for _, domain := range ipNetDomains {
				if activeDomainSet[domain] {
					continue
				}

				ppfmt.Noticef(pp.EmojiUserWarning,
					"Domain %q is ignored because it is only for %s but %s is disabled",
					domain.Describe(), ipFamily.Describe(), ipFamily.Describe())
			}
		}
	}

	// Step 4: regenerate proxiedMap from the raw PROXIED expression.
	proxiedMap := map[domain.Domain]bool{}
	if len(activeDomainSet) > 0 {
		proxiedPredicate, ok := domainexp.ParseExpression(ppfmt, "PROXIED", c.ProxiedExpression)
		if !ok {
			return nil, false
		}

		for dom := range activeDomainSet {
			proxiedMap[dom] = proxiedPredicate(dom)
		}
	}

	// Step 5: check if new parameters are unused.
	if len(activeDomainSet) == 0 { // We are only updating WAF lists.
		if c.TTL != api.TTLAuto {
			ppfmt.Noticef(pp.EmojiUserWarning, "TTL=%v is ignored because no domains will be updated", c.TTL)
		}
		if c.ProxiedExpression != "false" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"PROXIED (%s) is ignored because no domains will be updated",
				describeIgnoredSettingValuePreview(c.ProxiedExpression))
		}
		if c.RecordComment != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"RECORD_COMMENT (%s) is ignored because no domains will be updated",
				describeIgnoredSettingValuePreview(c.RecordComment))
		}
		if c.ManagedRecordsCommentRegex != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"MANAGED_RECORDS_COMMENT_REGEX (%s) is ignored because no domains will be updated",
				describeIgnoredSettingValuePreview(c.ManagedRecordsCommentRegex))
		}
	}
	if len(c.WAFLists) == 0 { // We are only updating domains.
		if c.WAFListDescription != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"WAF_LIST_DESCRIPTION (%s) is ignored because WAF_LISTS is empty",
				describeIgnoredSettingValuePreview(c.WAFListDescription))
		}
		if c.WAFListItemComment != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"WAF_LIST_ITEM_COMMENT (%s) is ignored because WAF_LISTS is empty",
				describeIgnoredSettingValuePreview(c.WAFListItemComment))
		}
		if c.ManagedWAFListItemsCommentRegex != "" {
			ppfmt.Noticef(pp.EmojiUserWarning,
				"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX (%s) is ignored because WAF_LISTS is empty",
				describeIgnoredSettingValuePreview(c.ManagedWAFListItemsCommentRegex))
		}
	}

	handleConfig := &HandleConfig{
		Auth: c.Auth,
		Options: api.HandleOptions{
			CacheExpiration: c.CacheExpiration,
			HandleOwnershipPolicy: api.HandleOwnershipPolicy{
				ManagedRecordsCommentRegex:        managedRecordsCommentRegex,
				ManagedWAFListItemsCommentRegex:   managedWAFListItemsCommentRegex,
				AllowWholeWAFListDeleteOnShutdown: c.ManagedWAFListItemsCommentRegex == "",
			},
		},
	}
	lifecycleConfig := &LifecycleConfig{
		UpdateCron:    c.UpdateCron,
		UpdateOnStart: c.UpdateOnStart,
		DeleteOnStop:  c.DeleteOnStop,
	}
	updateConfig := &UpdateConfig{
		Provider:           providerMap,
		Domains:            domains,
		WAFLists:           c.WAFLists,
		TTL:                c.TTL,
		Proxied:            proxiedMap,
		RecordComment:      c.RecordComment,
		WAFListDescription: c.WAFListDescription,
		WAFListItemComment: c.WAFListItemComment,
		DetectionTimeout:   c.DetectionTimeout,
		UpdateTimeout:      c.UpdateTimeout,
	}

	return &BuiltConfig{
		Handle:    handleConfig,
		Lifecycle: lifecycleConfig,
		Update:    updateConfig,
	}, true
}
