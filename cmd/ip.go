package main

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updator"
)

func setIP(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle, ipNet ipnet.Type, ip netip.Addr) {
	for _, target := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		_ = updator.Do(ctx, ppfmt,
			&updator.Args{
				Handle:    h,
				Domain:    target,
				IPNetwork: ipNet,
				IP:        ip,
				TTL:       c.TTL,
				Proxied:   c.Proxied,
			})
	}
}

var ipv6MessageDisplayed = false //nolint:gochecknoglobals

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle, ipNet ipnet.Type) netip.Addr {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip := c.Policy[ipNet].GetIP(ctx, ppfmt, ipNet)
	if ip.IsValid() {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())
		if !ipv6MessageDisplayed && ipNet == ipnet.IP6 {
			ipv6MessageDisplayed = true
			ppfmt.Infof(pp.EmojiConfig, "If you are using Docker, Kubernetes, or other frameworks, IPv6 networks often require additional setups.") //nolint:lll
			ppfmt.Infof(pp.EmojiConfig, "Read more about IPv6 networks in the README at https://github.com/favonia/cloudflare-ddns")                //nolint:lll
		}
	}
	return ip
}

func updateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle) {
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
			ip := detectIP(ctx, ppfmt, c, h, ipNet)
			if ip.IsValid() {
				setIP(ctx, ppfmt, c, h, ipNet, ip)
			}
		}
	}
}

func clearIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle) {
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
			setIP(ctx, ppfmt, c, h, ipNet, netip.Addr{})
		}
	}
}
