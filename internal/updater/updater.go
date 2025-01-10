// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"net/netip"

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

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) (netip.Addr, Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet)

	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address %v", ipNet.Describe(), ip)
		ppfmt.Suppress(getMessageIDForDetection(ipNet))
	} else {
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
	return ip, generateDetectMessage(ipNet, ok)
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

// setIP extracts relevant settings from the configuration and calls [setter.Setter.Set] with timeout.
// ip must be non-zero.
func setIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr,
) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipNet] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.Set(ctx, ppfmt, ipNet, domain, ip, c.TTL, c.Proxied[domain], c.RecordComment)
			}),
		)
	}

	return generateUpdateMessage(ipNet, ip, resps)
}

// finalDeleteIP extracts relevant settings from the configuration
// and calls [setter.Setter.FinalDelete] with a deadline.
func finalDeleteIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type) Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipNet] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.FinalDelete(ctx, ppfmt, ipNet, domain, c.TTL, c.Proxied[domain], c.RecordComment)
			}),
		)
	}

	return generateFinalDeleteMessage(ipNet, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, detectedIP map[ipnet.Type]netip.Addr,
) Message {
	resps := emptySetterWAFListResponses()

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetWAFList(ctx, ppfmt, l, c.WAFListDescription, detectedIP, "")
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
	detectedIP := map[ipnet.Type]netip.Addr{}
	numManagedNetworks := 0
	numValidIPs := 0
	for ipNet, provider := range ipnet.Bindings(c.Provider) {
		if provider != nil {
			numManagedNetworks++
			ip, msg := detectIP(ctx, ppfmt, c, ipNet)
			detectedIP[ipNet] = ip
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.MonitorMessage.OK {
				numValidIPs++
				msgs = append(msgs, setIP(ctx, ppfmt, c, s, ipNet, ip))
			}
		}
	}

	// Close all idle connections after the IP detection
	provider.CloseIdleConnections()

	// Update WAF lists
	if !(numManagedNetworks == 2 && numValidIPs == 0) {
		msgs = append(msgs, setWAFLists(ctx, ppfmt, c, s, detectedIP))
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
