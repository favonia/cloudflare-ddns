package main

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/updator"
)

func setIP(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle, ipNet ipnet.Type, ip net.IP) {
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

func updateIP(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle, ipNet ipnet.Type) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip := c.Policy[ipNet].GetIP(ctx, ppfmt, ipNet)
	if ip == nil {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())
		return
	}

	ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	setIP(ctx, ppfmt, c, h, ipNet, ip)
}

func updateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle) {
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
			updateIP(ctx, ppfmt, c, h, ipNet)
		}
	}
}

func clearIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, h api.Handle) {
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Policy[ipNet] != nil {
			setIP(ctx, ppfmt, c, h, ipNet, nil)
		}
	}
}
