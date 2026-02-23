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

func getMessageIDForDetection(ipNet ipnet.Type) pp.ID {
	return map[ipnet.Type]pp.ID{
		ipnet.IP4: pp.MessageIP4DetectionFails,
		ipnet.IP6: pp.MessageIP6DetectionFails,
	}[ipNet]
}

func detectIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) ([]netip.Addr, Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	ips, ok := c.Provider[ipNet].GetIPs(ctx, ppfmt, ipNet)

	switch {
	// Fast path: one detected target.
	case ok && len(ips) == 1:
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address %v", ipNet.Describe(), ips[0])
		ppfmt.Suppress(getMessageIDForDetection(ipNet))

	// Multi-target path: report the full deterministic set.
	case ok && len(ips) > 1:
		ppfmt.Infof(pp.EmojiInternet, "Detected %d %s addresses: %s",
			len(ips), ipNet.Describe(), pp.JoinMap(netip.Addr.String, ips))
		ppfmt.Suppress(getMessageIDForDetection(ipNet))

	// Failure path: emit hints and timeout guidance.
	default:
		ok = false
		ppfmt.Noticef(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())

		switch ipNet {
		case ipnet.IP6:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipNet), pp.EmojiHint,
				"If you are using Docker or Kubernetes, IPv6 might need extra setup. Read more at %s. "+
					"If your network doesn't support IPv6, you can turn it off by setting IP6_PROVIDER=none",
				pp.ManualURL)

		case ipnet.IP4:
			ppfmt.NoticeOncef(getMessageIDForDetection(ipNet), pp.EmojiHint,
				"If your network does not support IPv4, you can disable it with IP4_PROVIDER=none")
		}

		if errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.NoticeOncef(pp.MessageDetectionTimeouts, pp.EmojiHint,
				"If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v",
				c.DetectionTimeout,
			)
		}
	}
	return ips, generateDetectMessage(ipNet, ok)
}

var errTimeout = errors.New("timeout")

func wrapUpdateWithTimeout(ctx context.Context, ppfmt pp.PP, c *config.Config,
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
	c *config.Config, s setter.Setter, ipNet ipnet.Type, ips []netip.Addr,
) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipNet] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetIPs(ctx, ppfmt, ipNet, domain, ips, api.RecordParams{
					TTL:     c.TTL,
					Proxied: c.Proxied[domain],
					Comment: c.RecordComment,
				})
			}),
		)
	}

	return generateUpdateMessage(ipNet, ips, resps)
}

// finalDeleteIP extracts relevant settings from the configuration
// and calls [setter.Setter.FinalDelete] with a deadline.
func finalDeleteIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipNet] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.FinalDelete(ctx, ppfmt, ipNet, domain, api.RecordParams{
					TTL:     c.TTL,
					Proxied: c.Proxied[domain],
					Comment: c.RecordComment,
				})
			}),
		)
	}

	return generateFinalDeleteMessage(ipNet, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, detectedIPs map[ipnet.Type][]netip.Addr,
) Message {
	resps := emptySetterWAFListResponses()

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetWAFList(ctx, ppfmt, l, c.WAFListDescription, detectedIPs, "")
			}),
		)
	}

	return generateUpdateWAFListsMessage(resps)
}

// finalClearWAFLists extracts relevant settings from the configuration
// and calls [setter.Setter.ClearWAFList] with a deadline.
func finalClearWAFLists(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) Message {
	resps := emptySetterWAFListResponses()

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.FinalClearWAFList(ctx, ppfmt, l, c.WAFListDescription)
			}),
		)
	}

	return generateFinalClearWAFListsMessage(resps)
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) Message {
	var msgs []Message
	detectedIPsForWAF := map[ipnet.Type][]netip.Addr{}
	numManagedNetworks := 0
	numValidIPs := 0
	for ipNet, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			numManagedNetworks++
			ips, msg := detectIPs(ctx, ppfmt, c, ipNet)
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.MonitorMessage.OK {
				numValidIPs++
				detectedIPsForWAF[ipNet] = ips
				msgs = append(msgs, setIPs(ctx, ppfmt, c, s, ipNet, ips))
			} else {
				detectedIPsForWAF[ipNet] = nil
			}
		}
	}

	// Close all idle connections after the IP detection
	provider.CloseIdleConnections()

	// Update WAF lists
	if numValidIPs > 0 || numManagedNetworks < ipnet.NetworkCount {
		msgs = append(msgs, setWAFLists(ctx, ppfmt, c, s, detectedIPsForWAF))
	}

	return MergeMessages(msgs...)
}

// FinalDeleteIPs removes all DNS records of managed domains.
func FinalDeleteIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) Message {
	var msgs []Message

	for ipNet, provider := range ipnet.Bindings(c.Provider) {
		if provider != nil {
			msgs = append(msgs, finalDeleteIP(ctx, ppfmt, c, s, ipNet))
		}
	}

	// Clear WAF lists
	msgs = append(msgs, finalClearWAFLists(ctx, ppfmt, c, s))

	return MergeMessages(msgs...)
}
