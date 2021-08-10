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

func setIP(ctx context.Context, indent pp.Indent, c *config.Config, h api.Handle, ipNet ipnet.Type, ip net.IP) {
	for _, target := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		_ = updator.Do(ctx, indent, c.Quiet,
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

func updateIP(ctx context.Context, indent pp.Indent, c *config.Config, h api.Handle, ipNet ipnet.Type) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip := c.Policy[ipNet].GetIP(ctx, indent, ipNet)
	if ip == nil {
		pp.TopPrintf(pp.EmojiError, "Failed to detect the %s address.", ipNet.Describe())
		return
	}

	if !c.Quiet {
		pp.TopPrintf(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	}

	setIP(ctx, indent, c, h, ipNet, ip)
}

func updateIPs(ctx context.Context, indent pp.Indent, c *config.Config, h api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		updateIP(ctx, indent, c, h, ipnet.IP4)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		updateIP(ctx, indent, c, h, ipnet.IP6)
	}
}

func clearIPs(ctx context.Context, indent pp.Indent, c *config.Config, h api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		setIP(ctx, indent, c, h, ipnet.IP4, nil)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		setIP(ctx, indent, c, h, ipnet.IP6, nil)
	}
}
