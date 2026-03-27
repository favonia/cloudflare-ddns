// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
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

func deriveDNSAddresses(rawData provider.DetectionResult) []netip.Addr {
	addresses := make([]netip.Addr, 0, len(rawData.RawEntries))
	for _, entry := range rawData.RawEntries {
		addresses = append(addresses, entry.Addr())
	}
	slices.SortFunc(addresses, netip.Addr.Compare)
	return slices.Compact(addresses)
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
	addresses := deriveDNSAddresses(rawData)

	switch {
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
	c *config.UpdateConfig, s setter.Setter, ipFamily ipnet.Family, ips []netip.Addr,
) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipFamily] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetIPs(ctx, ppfmt, ipFamily, domain, ips, api.RecordParams{
					TTL:     c.TTL,
					Proxied: c.Proxied[domain],
					Comment: c.RecordComment,
					// The config surface does not expose non-empty fallback DNS tags yet.
					// Nil here therefore means "no configured fallback tags", not "clear tags".
					Tags: nil,
				})
			}),
		)
	}

	return generateClearOrUpdateMessage(ipFamily, ips, resps)
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
			addresses := deriveDNSAddresses(rawData)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.HeartbeatMessage.OK {
				targetsForWAF[ipFamily] = deriveWAFTargets(rawData)
				shouldUpdateWAF = true
				msgs = append(msgs, setIPs(ctx, ppfmt, c, s, ipFamily, addresses))
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

	return MergeMessages(msgs...)
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

	return MergeMessages(msgs...)
}
