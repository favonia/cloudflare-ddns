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

const (
	HintIP4DetectionFails string = "detect-ip4-fail"
	HintIP6DetectionFails string = "detect-ip6-fail"
	HintDetectionTimeouts string = "detect-timeout"
	HintUpdateTimeouts    string = "update-timeout"
)

// ShouldDisplayHints determines whether help messages should be displayed.
// The help messages are to help beginners detect possible misconfiguration.
// These messages should be displayed at most once, and thus the value of this map
// will be changed to false after displaying the help messages.
//
//nolint:gochecknoglobals
var ShouldDisplayHints = map[string]bool{
	HintIP4DetectionFails: true,
	HintIP6DetectionFails: true,
	HintDetectionTimeouts: true,
	HintUpdateTimeouts:    true,
}

func getHintIDForDetection(ipNet ipnet.Type) string {
	return map[ipnet.Type]string{
		ipnet.IP4: HintIP4DetectionFails,
		ipnet.IP6: HintIP6DetectionFails,
	}[ipNet]
}

func getProxied(ppfmt pp.PP, c *config.Config, domain domain.Domain) bool {
	if proxied, ok := c.Proxied[domain]; ok {
		return proxied
	}

	ppfmt.Warningf(pp.EmojiImpossible,
		"Proxied[%s] not initialized; this should not happen; please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new", //nolint:lll
		domain.Describe(),
	)
	return false
}

var errTimeout = errors.New("timeout")

// setIP extracts relevant settings from the configuration and calls [setter.Setter.Set] with timeout.
// ip must be non-zero.
func setIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr,
) message.Message {
	resps := SetterResponses{}

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeoutCause(ctx, c.UpdateTimeout, errTimeout)
		defer cancel()

		resp := s.Set(ctx, ppfmt, domain, ipNet, ip, c.TTL, getProxied(ppfmt, c, domain), c.RecordComment)
		resps.Register(resp, domain)
		if resp == setter.ResponseFailed {
			if ShouldDisplayHints[HintUpdateTimeouts] && errors.Is(context.Cause(ctx), errTimeout) {
				ppfmt.Infof(pp.EmojiHint,
					"If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v",
					c.UpdateTimeout,
				)
				ShouldDisplayHints[HintUpdateTimeouts] = false
			}
		}
	}

	return GenerateUpdateMessage(ipNet, ip, resps)
}

// deleteIP extracts relevant settings from the configuration and calls [setter.Setter.Delete] with a deadline.
func deleteIP(
	ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type,
) message.Message {
	resps := SetterResponses{}

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeoutCause(ctx, c.UpdateTimeout, errTimeout)
		defer cancel()

		resp := s.Delete(ctx, ppfmt, domain, ipNet)
		resps.Register(resp, domain)
		if resp == setter.ResponseFailed {
			if ShouldDisplayHints[HintUpdateTimeouts] && errors.Is(context.Cause(ctx), errTimeout) {
				ppfmt.Infof(pp.EmojiHint,
					"If your network is experiencing high latency, consider increasing UPDATE_TIMEOUT=%v",
					c.UpdateTimeout,
				)
				ShouldDisplayHints[HintUpdateTimeouts] = false
			}
		}
	}

	return GenerateDeleteMessage(ipNet, resps)
}

func detectIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, ipNet ipnet.Type, use1001 bool,
) (netip.Addr, message.Message) {
	ctx, cancel := context.WithTimeoutCause(ctx, c.DetectionTimeout, errTimeout)
	defer cancel()

	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet, use1001)
	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())

		if ShouldDisplayHints[HintDetectionTimeouts] && errors.Is(context.Cause(ctx), errTimeout) {
			ppfmt.Infof(pp.EmojiHint,
				"If your network is experiencing high latency, consider increasing DETECTION_TIMEOUT=%v",
				c.DetectionTimeout,
			)
			ShouldDisplayHints[HintDetectionTimeouts] = false
		} else if ShouldDisplayHints[getHintIDForDetection(ipNet)] {
			switch ipNet {
			case ipnet.IP6:
				ppfmt.Infof(pp.EmojiHint, "If you are using Docker or Kubernetes, IPv6 often requires additional setups")     //nolint:lll
				ppfmt.Infof(pp.EmojiHint, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns")      //nolint:lll
				ppfmt.Infof(pp.EmojiHint, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none") //nolint:lll
			case ipnet.IP4:
				ppfmt.Infof(pp.EmojiHint, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none") //nolint:lll
			}
		}
	}
	ShouldDisplayHints[getHintIDForDetection(ipNet)] = false
	return ip, GenerateDetectMessage(ipNet, ok)
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) message.Message {
	var msgs []message.Message

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			use1001 := c.ShouldWeUse1001Now(ctx, ppfmt)
			ip, msg := detectIP(ctx, ppfmt, c, ipNet, use1001)
			msgs = append(msgs, msg)

			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.
			if msg.Ok {
				msgs = append(msgs, setIP(ctx, ppfmt, c, s, ipNet, ip))
			}
		}
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

	return message.Merge(msgs...)
}
