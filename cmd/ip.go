package main

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

func setIP(ctx context.Context, indent pp.Indent, c *config.Config, h *api.Handle, ipNet ipnet.Type, ip net.IP) {
	for _, target := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		_ = h.Update(ctx, indent,
			&api.UpdateArgs{
				Quiet:     c.Quiet,
				Domain:    target,
				IPNetwork: ipNet,
				IP:        ip,
				TTL:       c.TTL,
				Proxied:   c.Proxied,
			})
	}
}

func updateIP(ctx context.Context, indent pp.Indent, c *config.Config, h *api.Handle, ipNet ipnet.Type) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip, ok := c.Policy[ipNet].GetIP(ctx, indent, ipNet)
	if !ok {
		pp.TopPrintf(pp.EmojiError, "Failed to detect the %s address.", ipNet)
		return
	}

	if !c.Quiet {
		pp.TopPrintf(pp.EmojiInternet, "Detected the %s address: %v", ipNet, ip)
	}

	setIP(ctx, indent, c, h, ipNet, ip)
}

func updateIPs(ctx context.Context, indent pp.Indent, c *config.Config, h *api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		updateIP(ctx, indent, c, h, ipnet.IP4)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		updateIP(ctx, indent, c, h, ipnet.IP6)
	}
}

func clearIPs(ctx context.Context, indent pp.Indent, c *config.Config, h *api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		setIP(ctx, indent, c, h, ipnet.IP4, nil)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		setIP(ctx, indent, c, h, ipnet.IP6, nil)
	}
}
