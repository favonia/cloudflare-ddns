package updater

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func getProxied(ppfmt pp.PP, c *config.Config, domain api.Domain) bool {
	if proxied, ok := c.ProxiedByDomain[domain]; ok {
		return proxied
	}

	c.ProxiedByDomain[domain] = c.DefaultProxied
	ppfmt.Warningf(pp.EmojiImpossible,
		"Internal failure: ProxiedByDomain[%s] was not set, and is reset to %t",
		domain.Describe(), c.DefaultProxied,
	)
	ppfmt.Warningf(pp.EmojiImpossible,
		"Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new",
	)
	return c.DefaultProxied
}

func setIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr) bool {
	ok := true

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		if !s.Set(ctx, ppfmt, domain, ipNet, ip, getProxied(ppfmt, c, domain)) {
			ok = false
		}
	}

	return ok
}

var MessageShouldDisplay = map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true} //nolint:gochecknoglobals

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) netip.Addr {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet)
	if ip.IsValid() {
		MessageShouldDisplay[ipNet] = false
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())

		if MessageShouldDisplay[ipNet] {
			MessageShouldDisplay[ipNet] = false
			switch ipNet {
			case ipnet.IP6:
				ppfmt.Infof(pp.EmojiConfig, "If you are using Docker, Kubernetes, or other frameworks, IPv6 networks often require additional setups") //nolint:lll
				ppfmt.Infof(pp.EmojiConfig, "Read more about IPv6 networks in the README at https://github.com/favonia/cloudflare-ddns")               //nolint:lll
				ppfmt.Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable IPv6 with IP6_PROVIDER=none")                      //nolint:lll
			case ipnet.IP4:
				ppfmt.Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable IPv4 with IP4_PROVIDER=none") //nolint:lll
			}
		}
	}
	return ip
}

func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) bool {
	ok := true

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ip := detectIP(ctx, ppfmt, c, ipNet)
			if !ip.IsValid() {
				ok = false
				continue
			}

			if !setIP(ctx, ppfmt, c, s, ipNet, ip) {
				ok = false
			}
		}
	}

	return ok
}

func ClearIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) bool {
	ok := true

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			if !setIP(ctx, ppfmt, c, s, ipNet, netip.Addr{}) {
				ok = false
			}
		}
	}

	return ok
}
