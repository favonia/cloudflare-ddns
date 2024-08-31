// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func getHintIDForDetection(ipNet ipnet.Type) pp.Hint {
	return map[ipnet.Type]pp.Hint{
		ipnet.IP4: pp.HintIP4DetectionFails,
		ipnet.IP6: pp.HintIP6DetectionFails,
	}[ipNet]
}

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) (netip.Addr, message.Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	ip, method, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet)

	if ok {
		switch method {
		case protocol.MethodAlternative:
			ppfmt.Infof(pp.EmojiInternet, "Detected the %s address %v (using 1.0.0.1)", ipNet.Describe(), ip)
			provider.Hint1111Blockage(ppfmt)
		default:
			ppfmt.Infof(pp.EmojiInternet, "Detected the %s address %v", ipNet.Describe(), ip)
		}
		ppfmt.SuppressHint(getHintIDForDetection(ipNet))
	} else {
		ppfmt.Noticef(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())

		switch ipNet {
		case ipnet.IP6:
			ppfmt.Hintf(getHintIDForDetection(ipNet),
				"If you are using Docker or Kubernetes, IPv6 often requires additional steps to set up; read more at %s. "+
					"If your network does not support IPv6, you can disable it with IP6_PROVIDER=none",
				pp.ManualURL)

		case ipnet.IP4:
			ppfmt.Hintf(getHintIDForDetection(ipNet),
				"If your network does not support IPv4, you can disable it with IP4_PROVIDER=none")
		}

		if errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.Hintf(pp.HintDetectionTimeouts,
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
			ppfmt.Hintf(pp.HintUpdateTimeouts,
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
) message.Message {
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

// deleteIP extracts relevant settings from the configuration and calls [setter.Setter.Delete] with a deadline.
func deleteIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type) message.Message {
	resps := emptySetterResponses()

	for _, domain := range c.Domains[ipNet] {
		resps.register(domain,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.Delete(ctx, ppfmt, ipNet, domain)
			}),
		)
	}

	return generateDeleteMessage(ipNet, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, detectedIP map[ipnet.Type]netip.Addr,
) message.Message {
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

// clearWAFLists extracts relevant settings from the configuration
// and calls [setter.Setter.ClearWAFList] with a deadline.
func clearWAFLists(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	resps := emptySetterWAFListResponses()

	for _, l := range c.WAFLists {
		resps.register(l.Describe(),
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.ClearWAFList(ctx, ppfmt, l)
			}),
		)
	}

	return generateClearWAFListsMessage(resps)
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	var msgs []message.Message
	detectedIP := map[ipnet.Type]netip.Addr{}
	numManagedNetworks := 0
	numValidIPs := 0
	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
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

	// Update WAF lists
	if !(numManagedNetworks == 2 && numValidIPs == 0) {
		msgs = append(msgs, setWAFLists(ctx, ppfmt, c, s, detectedIP))
	}

	return message.Merge(msgs...)
}

// DeleteIPs removes all DNS records of managed domains.
func DeleteIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	var msgs []message.Message

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			msgs = append(msgs, deleteIP(ctx, ppfmt, c, s, ipNet))
		}
	}

	// Clear WAF lists
	msgs = append(msgs, clearWAFLists(ctx, ppfmt, c, s))

	return message.Merge(msgs...)
}
