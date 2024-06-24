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
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/response"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// ShouldDisplayHints determines whether help messages should be displayed.
// The help messages are to help beginners detect possible misconfiguration.
// These messages should be displayed at most once, and thus the value of this map
// will be changed to false after displaying the help messages.
//
//nolint:gochecknoglobals
var ShouldDisplayHints = map[string]bool{
	"detect-ip4-fail": true,
	"detect-ip6-fail": true,
	"update-timeout":  true,
}

func getHintIDForDetection(ipNet ipnet.Type) string {
	return map[ipnet.Type]string{
		ipnet.IP4: "detect-ip4-fail",
		ipnet.IP6: "detect-ip6-fail",
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

var errSettingTimeout = errors.New("setting timeout")

// setIP extracts relevant settings from the configuration and calls [setter.Setter.Set] with timeout.
// ip must be non-zero.
func setIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr,
) SetterResponses {
	resps := SetterResponses{}

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeoutCause(ctx, c.UpdateTimeout, errSettingTimeout)
		defer cancel()

		resp := s.Set(ctx, ppfmt, domain, ipNet, ip, c.TTL, getProxied(ppfmt, c, domain))
		resps.Register(resp, domain)
		if resp == setter.ResponseFailed {
			if ShouldDisplayHints["update-timeout"] && errors.Is(context.Cause(ctx), errSettingTimeout) {
				ppfmt.Infof(pp.EmojiHint,
					"If your network is working but with high latency, consider increasing the value of UPDATE_TIMEOUT",
				)
				ShouldDisplayHints["update-timeout"] = false
			}
		}
	}

	return resps
}

// deleteIP extracts relevant settings from the configuration and calls [setter.Setter.Delete] with a deadline.
func deleteIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type) SetterResponses {
	resps := SetterResponses{}

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		resp := s.Delete(ctx, ppfmt, domain, ipNet)
		resps.Register(resp, domain)
	}

	return resps
}

func detectIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, ipNet ipnet.Type, use1001 bool,
) (netip.Addr, bool) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet, use1001)
	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())
		if ShouldDisplayHints[getHintIDForDetection(ipNet)] {
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
	return ip, ok
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) response.Response {
	var resps []response.Response

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ip, ok := detectIP(ctx, ppfmt, c, ipNet, c.Use1001)
			// Note: If we can't detect the new IP address,
			// it's probably better to leave existing records alone.

			srs := SetterResponses{}
			if ok {
				srs = setIP(ctx, ppfmt, c, s, ipNet, ip)
			}

			resps = append(resps, srs.ToResponse(OperationUpdate, ipNet, ip))
		}
	}
	return response.Merge(resps...)
}

// DeleteIPs removes all DNS records of managed domains.
func DeleteIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) response.Response {
	var resps []response.Response

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			resp := deleteIP(ctx, ppfmt, c, s, ipNet)
			resps = append(resps, resp.ToResponse(OperationDelete, ipNet, netip.Addr{}))
		}
	}

	return response.Merge(resps...)
}
