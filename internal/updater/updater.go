package updater

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func setIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr) bool {
	ok := true

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		if !s.Set(ctx, ppfmt, domain, ipNet, ip) {
			ok = false
		}
	}

	return ok
}

var IPv6MessageDisplayed = false //nolint:gochecknoglobals

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) netip.Addr {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip := c.Policy[ipNet].GetIP(ctx, ppfmt, ipNet)
	if ip.IsValid() {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())
		if !IPv6MessageDisplayed && ipNet == ipnet.IP6 {
			IPv6MessageDisplayed = true
			ppfmt.Infof(pp.EmojiConfig, "If you are using Docker, Kubernetes, or other frameworks, IPv6 networks often require additional setups.") //nolint:lll
			ppfmt.Infof(pp.EmojiConfig, "Read more about IPv6 networks in the README at https://github.com/favonia/cloudflare-ddns")                //nolint:lll
		}
	}
	return ip
}

func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) bool {
	ok := true

	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
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

	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
			if !setIP(ctx, ppfmt, c, s, ipNet, netip.Addr{}) {
				ok = false
			}
		}
	}

	return ok
}
