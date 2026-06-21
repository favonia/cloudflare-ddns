// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func getMessageIDForDetection(ipFamily ipnet.Family) pp.ID {
	return map[ipnet.Family]pp.ID{
		ipnet.IP4: pp.MessageIP4DetectionFails,
		ipnet.IP6: pp.MessageIP6DetectionFails,
	}[ipFamily]
}

func getMessageIDForDetectionFilter(ipFamily ipnet.Family) pp.ID {
	return map[ipnet.Family]pp.ID{
		ipnet.IP4: pp.MessageIP4DetectionFilterEmpties,
		ipnet.IP6: pp.MessageIP6DetectionFilterEmpties,
	}[ipFamily]
}

func addressWord(n int) string {
	if n == 1 {
		return "address"
	}
	return "addresses"
}

func deriveDNSAddresses(rawData provider.DetectionResult) []netip.Addr {
	addresses := make([]netip.Addr, 0, len(rawData.RawEntries))
	for _, entry := range rawData.RawEntries {
		addresses = append(addresses, entry.Addr())
	}
	slices.SortFunc(addresses, netip.Addr.Compare)
	return slices.Compact(addresses)
}

func sharedDNSTargets(domains []domain.Domain, addresses []netip.Addr) dnsTargetsByDomain {
	targets := make(dnsTargetsByDomain, len(domains))
	for _, configuredDomain := range domains {
		targets[configuredDomain] = addresses
	}
	return targets
}

func deriveWAFTargets(rawData provider.DetectionResult) setter.WAFTargets {
	if !rawData.Available {
		return setter.NewUnavailableWAFTargets()
	}

	prefixes := make([]netip.Prefix, 0, len(rawData.RawEntries))
	for _, entry := range rawData.RawEntries {
		prefixes = append(prefixes, entry.Masked())
	}
	slices.SortFunc(prefixes, netip.Prefix.Compare)
	prefixes = slices.Compact(prefixes)
	return setter.NewAvailableWAFTargets(prefixes)
}

func detectRawData(
	ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, ipFamily ipnet.Family,
) (provider.DetectionResult, Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	rawData := c.Provider[ipFamily].GetRawData(ctx, ppfmt, ipFamily, c.DefaultPrefixLen[ipFamily])
	filter := c.DetectionFilter[ipFamily]
	filterAbort := false
	if rawData.Available && len(rawData.RawEntries) > 0 && !filter.IsDefault() {
		kept, dropped := filter.Partition(rawData.RawEntries)
		// Report the dropped addresses, not the kept ones: the dropped set is logged
		// nowhere else, while the kept set is reported by the later "Detected" line.
		// This is the operator's only signal for an over-aggressive filter, so it is
		// emitted whenever anything is dropped, including the all-dropped abort.
		reportDropped := func() {
			ppfmt.Infof(pp.EmojiInternet, "Dropped %d %s %s after filtering: %s",
				len(dropped), ipFamily.Describe(), addressWord(len(dropped)),
				pp.JoinMap(func(e ipnet.RawEntry) string {
					return e.Describe(c.DefaultPrefixLen[ipFamily])
				}, dropped))
		}
		switch {
		case len(kept) == 0:
			ppfmt.Noticef(pp.EmojiError,
				"No detected %s addresses remain after filtering; %s update aborted",
				ipFamily.Describe(), ipFamily.Describe())
			reportDropped()
			ppfmt.NoticeOncef(getMessageIDForDetectionFilter(ipFamily), pp.EmojiHint,
				"Check IP%d_DETECTION_FILTER if this was unexpected",
				ipFamily.Int())
			rawData = provider.NewUnavailableDetectionResult()
			filterAbort = true
		case len(dropped) > 0:
			rawData.RawEntries = kept
			reportDropped()
		default:
			rawData.RawEntries = kept
		}
	}
	addresses := deriveDNSAddresses(rawData)

	switch {
	case filterAbort:
		return rawData, generateFilterAbortDetectMessage(ipFamily)

	case rawData.Available && len(rawData.RawEntries) == 0:
		ppfmt.Infof(pp.EmojiClear, "Clearing %s addresses . . .", ipFamily.Describe())
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Fast path: one detected address.
	case rawData.Available && len(addresses) == 1:
		ppfmt.Infof(pp.EmojiInternet, "Detected %s address: %s",
			ipFamily.Describe(), rawData.RawEntries[0].Describe(c.DefaultPrefixLen[ipFamily]))
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Multi-address path: report the full deterministic set.
	case rawData.Available && len(addresses) > 1:
		defaultPrefixLen := c.DefaultPrefixLen[ipFamily]
		ppfmt.Infof(pp.EmojiInternet, "Detected %d %s addresses: %s",
			len(addresses), ipFamily.Describe(),
			pp.JoinMap(func(e ipnet.RawEntry) string { return e.Describe(defaultPrefixLen) }, rawData.RawEntries))
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Failure path: emit hints and timeout guidance.
	default:
		rawData = provider.NewUnavailableDetectionResult()
		ppfmt.Noticef(pp.EmojiError, "No valid %s addresses were detected; will try again", ipFamily.Describe())

		switch ipFamily {
		case ipnet.IP6:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipFamily), pp.EmojiHint,
				"If you are using Docker or Kubernetes, IPv6 might need extra setup. Read more at %s. "+
					"If your network doesn't support IPv6, you can stop managing it by setting IP6_PROVIDER=none",
				pp.ManualURL)

		case ipnet.IP4:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipFamily), pp.EmojiHint,
				"If your network does not support IPv4, you can stop managing it with IP4_PROVIDER=none")
		}

		if errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.NoticeOncef(pp.MessageDetectionTimeouts, pp.EmojiHint,
				"If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v",
				c.DetectionTimeout,
			)
		}
	}
	return rawData, generateDetectMessage(ipFamily, rawData.Available)
}

var errTimeout = errors.New("timeout")

func wrapUpdateWithTimeout(ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig,
	f func(context.Context) setter.ResponseCode,
) setter.ResponseCode {
	ctx, cancel := context.WithTimeoutCause(ctx, c.UpdateTimeout, errTimeout)
	defer cancel()

	resp := f(ctx)
	if resp == setter.ResponseFailed {
		if errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.NoticeOncef(pp.MessageUpdateTimeouts, pp.EmojiHint,
				"If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v",
				c.UpdateTimeout,
			)
		}
	}
	return resp
}

// setIPs extracts relevant settings from the configuration and calls [setter.Setter.SetIPs] with timeout.
func setIPs(ctx context.Context, ppfmt pp.PP,
	c *config.UpdateConfig, s setter.Setter, ipFamily ipnet.Family, targets map[domain.Domain][]netip.Addr,
) Message {
	type targetGroup struct {
		ips   []netip.Addr
		resps setterResponses
	}
	var groups []targetGroup
	var missingDomains []domain.Domain

	for _, configuredDomain := range c.Domains[ipFamily] {
		// A present-but-empty target set legitimately means "clear this domain's
		// records" (for example, when no address is detected), so an absent key
		// must not collapse into the same empty slice: that would let a future
		// regression desynchronizing this map from c.Domains silently delete real
		// DNS records instead of failing loudly. Treat absence as a reportable fault.
		ips, ok := targets[configuredDomain]
		if !ok {
			ppfmt.Noticef(pp.EmojiImpossible,
				"No target set was provided for managed domain %s; this should not happen. Please report it at %s",
				configuredDomain.Describe(), pp.IssueReportingURL)
			missingDomains = append(missingDomains, configuredDomain)
			continue
		}

		groupIndex := slices.IndexFunc(groups, func(group targetGroup) bool {
			return slices.Equal(group.ips, ips)
		})
		if groupIndex < 0 {
			groups = append(groups, targetGroup{ips: ips, resps: emptySetterResponses()})
			groupIndex = len(groups) - 1
		}

		groups[groupIndex].resps.register(configuredDomain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetIPs(ctx, ppfmt, ipFamily, configuredDomain, ips, api.RecordParams{
					TTL:     c.TTL,
					Proxied: c.Proxied[configuredDomain],
					Comment: c.RecordComment,
					// The config surface does not expose non-empty fallback DNS tags yet.
					// Nil here therefore means "the effective fallback tag set is empty", not "clear tags".
					Tags: nil,
				})
			}),
		)
	}

	msgs := make([]Message, 0, len(groups)+1)
	if len(missingDomains) > 0 {
		msgs = append(msgs, generateMissingTargetSetsMessage(ipFamily, missingDomains))
	}
	for _, group := range groups {
		msgs = append(msgs, generateClearOrUpdateMessage(ipFamily, group.ips, group.resps))
	}
	return mergeMessages(msgs...)
}

func reportHostID6Problems(ppfmt pp.PP, problems []hostID6ProblemGroup, hasWAFLists bool) {
	var macShortPrefixHints []hostID6ProblemGroup
	for _, problem := range problems {
		domains := pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, problem.Domains, "(none)")
		derivations := problem.Derivations.ConfigString()
		observed := pp.EnglishJoinMapOrEmptyLabel(ipnet.RawEntry.String, problem.Observed, "(none)")

		switch problem.Kind {
		case hostid6.LiteralPrefixTooLong:
			ppfmt.Noticef(pp.EmojiError,
				"No AAAA records were changed because hostid6=%s for %s requires detected prefixes no longer than /%d, "+
					"but detected %s; change that hostid6 setting or change IP6_PROVIDER",
				derivations, domains, problem.PrefixLenBound, observed)
		case hostid6.MACPrefixTooLong:
			ppfmt.Noticef(pp.EmojiError,
				"No AAAA records were changed because hostid6=%s for %s requires detected prefixes no longer than /%d, "+
					"but detected %s; change that hostid6 setting or change IP6_PROVIDER",
				derivations, domains, problem.PrefixLenBound, observed)
		case hostid6.MACPrefixTooShort:
			ppfmt.Noticef(pp.EmojiError,
				"No AAAA records were changed because hostid6=%s for %s requires a detected /64 prefix, "+
					"but detected %s; change that hostid6 setting or change IP6_PROVIDER",
				derivations, domains, observed)
			macShortPrefixHints = append(macShortPrefixHints, problem)
		default:
			panic(fmt.Sprintf("invalid host-ID incompatibility kind %d", problem.Kind))
		}
	}

	if hasWAFLists {
		// The hostid6 errors name AAAA records; keep this quiet-visible so
		// WAF operators also see that IPv6 list items were preserved.
		ppfmt.NoticeOncef(pp.MessageHostID6WAFItemsPreserved, pp.EmojiHint,
			"Existing IPv6 WAF list items were preserved for this update")
	}
	for _, problem := range macShortPrefixHints {
		if len(problem.Observed) > 0 {
			hostid6.EmitMACShortPrefixHint(ppfmt, problem.Derivations, problem.Observed[0])
		}
	}
}

// finalDeleteIP extracts relevant settings from the configuration
// and calls [setter.Setter.FinalDelete] with a deadline.
func finalDeleteIP(
	ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, s setter.Setter, ipFamily ipnet.Family,
) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipFamily] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.FinalDelete(ctx, ppfmt, ipFamily, domain, api.RecordParams{
					TTL:     c.TTL,
					Proxied: c.Proxied[domain],
					Comment: c.RecordComment,
					// Keep final-delete reconciliation aligned with steady-state updates:
					// current config can preserve/inherit existing tags but cannot specify
					// non-empty fallback tags.
					Tags: nil,
				})
			}),
		)
	}

	return generateFinalDeleteMessage(ipFamily, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.UpdateConfig, s setter.Setter, targets map[ipnet.Family]setter.WAFTargets,
) Message {
	resps := emptySetterWAFListResponses()

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetWAFList(ctx, ppfmt, l, c.WAFListDescription, targets, c.WAFListItemComment)
			}),
		)
	}

	return generateUpdateWAFListsMessage(resps)
}

// finalClearWAFLists extracts relevant settings from the configuration
// and calls [setter.Setter.FinalClearWAFList] with a deadline.
func finalClearWAFLists(ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, s setter.Setter) Message {
	resps := emptySetterWAFListResponses()
	managedFamilies := map[ipnet.Family]bool{}
	for ipFamily, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			managedFamilies[ipFamily] = true
		}
	}

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.FinalClearWAFList(ctx, ppfmt, l, c.WAFListDescription, managedFamilies)
			}),
		)
	}

	return generateFinalClearWAFListsMessage(resps)
}

// UpdateIPs detects IP addresses and updates DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, s setter.Setter) Message {
	var msgs []Message
	targetsForWAF := map[ipnet.Family]setter.WAFTargets{}
	shouldUpdateWAF := false
	for ipFamily, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			rawData, msg := detectRawData(ctx, ppfmt, c, ipFamily)
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.HeartbeatMessage.OK {
				targetsForWAF[ipFamily] = deriveWAFTargets(rawData)
				switch ipFamily {
				case ipnet.IP4:
					shouldUpdateWAF = true
					targets := sharedDNSTargets(c.Domains[ipFamily], deriveDNSAddresses(rawData))
					msgs = append(msgs, setIPs(ctx, ppfmt, c, s, ipFamily, targets))

				case ipnet.IP6:
					targets, problems := deriveIP6DNSTargets(c.Domains[ipFamily], c.HostID6, rawData)
					if len(problems) > 0 {
						reportHostID6Problems(ppfmt, problems, len(c.WAFLists) > 0)
						targetsForWAF[ipFamily] = setter.NewUnavailableWAFTargets()
						msgs = append(msgs, generateIP6DerivationFailureMessage())
						continue
					}
					shouldUpdateWAF = true
					msgs = append(msgs, setIPs(ctx, ppfmt, c, s, ipFamily, targets))
				}
			} else {
				targetsForWAF[ipFamily] = deriveWAFTargets(rawData)
			}
		}
	}

	// Close all idle connections after the IP detection
	provider.CloseIdleConnections()

	// Update WAF lists only when at least one family has usable derived targets.
	if shouldUpdateWAF {
		msgs = append(msgs, setWAFLists(ctx, ppfmt, c, s, targetsForWAF))
	}

	return mergeMessages(msgs...)
}

// FinalDeleteIPs removes all DNS records of managed domains.
func FinalDeleteIPs(ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, s setter.Setter) Message {
	var msgs []Message

	for ipFamily, provider := range ipnet.Bindings(c.Provider) {
		if provider != nil {
			msgs = append(msgs, finalDeleteIP(ctx, ppfmt, c, s, ipFamily))
		}
	}

	// Clear WAF lists
	msgs = append(msgs, finalClearWAFLists(ctx, ppfmt, c, s))

	return mergeMessages(msgs...)
}
