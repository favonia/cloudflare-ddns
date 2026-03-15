// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"net/netip"

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

func detectIPs(
	ctx context.Context, ppfmt pp.PP, c *config.UpdateConfig, ipFamily ipnet.Family,
) (provider.Targets, Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	targets := c.Provider[ipFamily].GetIPs(ctx, ppfmt, ipFamily)

	switch {
	case targets.Available && len(targets.IPs) == 0:
		ppfmt.Infof(pp.EmojiInternet, "The desired %s target set is empty", ipFamily.Describe())
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Fast path: one detected target.
	case targets.Available && len(targets.IPs) == 1:
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address %v", ipFamily.Describe(), targets.IPs[0])
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Multi-target path: report the full deterministic set.
	case targets.Available && len(targets.IPs) > 1:
		ppfmt.Infof(pp.EmojiInternet, "Detected %d %s addresses: %s",
			len(targets.IPs), ipFamily.Describe(), pp.JoinMap(netip.Addr.String, targets.IPs))
		ppfmt.Suppress(getMessageIDForDetection(ipFamily))

	// Failure path: emit hints and timeout guidance.
	default:
		targets = provider.NewUnavailableTargets()
		ppfmt.Noticef(pp.EmojiError, "Failed to detect any %s addresses", ipFamily.Describe())

		switch ipFamily {
		case ipnet.IP6:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipFamily), pp.EmojiHint,
				"If you are using Docker or Kubernetes, IPv6 might need extra setup. Read more at %s. "+
					"If your network doesn't support IPv6, you can turn it off by setting IP6_PROVIDER=none",
				pp.ManualURL)

		case ipnet.IP4:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipFamily), pp.EmojiHint,
				"If your network does not support IPv4, you can disable it with IP4_PROVIDER=none")
		}

		if errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.NoticeOncef(pp.MessageDetectionTimeouts, pp.EmojiHint,
				"If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v",
				c.DetectionTimeout,
			)
		}
	}
	return targets, generateDetectMessage(ipFamily, targets.Available)
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
					Tags:    nil,
				})
			}),
		)
	}

	return generateUpdateMessage(ipFamily, ips, resps)
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
					Tags:    nil,
				})
			}),
		)
	}

	return generateFinalDeleteMessage(ipFamily, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.UpdateConfig, s setter.Setter, targets map[ipnet.Family]provider.Targets,
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
	targetsForWAF := map[ipnet.Family]provider.Targets{}
	shouldUpdateWAF := false
	for ipFamily, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			targets, msg := detectIPs(ctx, ppfmt, c, ipFamily)
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.HeartbeatMessage.OK {
				targetsForWAF[ipFamily] = targets
				shouldUpdateWAF = true
				msgs = append(msgs, setIPs(ctx, ppfmt, c, s, ipFamily, targets.IPs))
			} else {
				targetsForWAF[ipFamily] = targets
			}
		}
	}

	// Close all idle connections after the IP detection
	provider.CloseIdleConnections()

	// Update WAF lists only when at least one family has a usable desired target set.
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
