// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"errors"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func getHintIDForDetection(ipNet ipnet.Type) pp.Hint {
	return map[ipnet.Type]pp.Hint{
		ipnet.IP4: pp.HintIP4DetectionFails,
		ipnet.IP6: pp.HintIP6DetectionFails,
	}[ipNet]
}

func detectIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, ipNet ipnet.Type,
) (netip.Addr, message.Message) {
	use1001 := c.ShouldWeUse1001Now(ctx, ppfmt)

	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet, use1001)
	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
		ppfmt.SuppressHint(getHintIDForDetection(ipNet))
	} else {
		ppfmt.Warningf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())

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

func getProxied(ppfmt pp.PP, c *config.Config, domain domain.Domain) bool {
	if proxied, ok := c.Proxied[domain]; ok {
		return proxied
	}

	ppfmt.Warningf(pp.EmojiImpossible,
		"Proxied[%s] not initialized; this should not happen; please report this at %s",
		domain.Describe(), pp.IssueReportingURL,
	)
	return false
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
				return s.Set(ctx, ppfmt, domain, ipNet, ip, c.TTL, getProxied(ppfmt, c, domain), c.RecordComment)
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
				return s.Delete(ctx, ppfmt, domain, ipNet)
			}),
		)
	}

	return generateDeleteMessage(ipNet, resps)
}

// setWAFList extracts relevant settings from the configuration and calls [setter.Setter.SetWAFList] with timeout.
func setWAFLists(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, detectedIPs map[ipnet.Type]netip.Addr,
) message.Message {
	resps := emptySetterWAFListResponses()

	for _, name := range c.WAFLists {
		resps.register(name,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.SetWAFList(ctx, ppfmt, name, c.WAFListDescription, detectedIPs, "")
			}),
		)
	}

	return generateUpdateWAFListMessage(resps)
}

// deleteWAFLists extracts relevant settings from the configuration
// and calls [setter.Setter.DeleteWAFList] with a deadline.
func deleteWAFLists(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	resps := emptySetterWAFListResponses()

	for _, name := range c.WAFLists {
		resps.register(name,
			wrapUpdateWithTimeout(ctx, ppfmt, c, func(ctx context.Context) setter.ResponseCode {
				return s.DeleteWAFList(ctx, ppfmt, name)
			}),
		)
	}

	return generateDeleteWAFListMessage(resps)
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	var msgs []message.Message
	detectedIPs := make(map[ipnet.Type]netip.Addr)

	detectedAnyIP := false
	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ip, msg := detectIP(ctx, ppfmt, c, ipNet)
			detectedIPs[ipNet] = ip
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.OK {
				detectedAnyIP = true
				msgs = append(msgs, setIP(ctx, ppfmt, c, s, ipNet, ip))
			}
		}
	}

	// Update WAF lists
	if detectedAnyIP {
		msgs = append(msgs, setWAFLists(ctx, ppfmt, c, s, detectedIPs))
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

	// Delete WAF lists
	msgs = append(msgs, deleteWAFLists(ctx, ppfmt, c, s))

	return message.Merge(msgs...)
}
