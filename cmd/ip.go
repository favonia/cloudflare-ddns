package main

import (
	"context"
	"log"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/config"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

func setIP(ctx context.Context, c *config.Config, h *api.Handle, ipNet ipnet.Type, ip net.IP) {
	for _, target := range c.Targets[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		_ = h.Update(ctx, &api.UpdateArgs{
			Quiet:     c.Quiet,
			Target:    target,
			IPNetwork: ipNet,
			IP:        ip,
			TTL:       c.TTL,
			Proxied:   c.Proxied,
		})
	}
}

func updateIP(ctx context.Context, c *config.Config, h *api.Handle, ipNet ipnet.Type) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip, ok := c.Policy[ipNet].GetIP(ctx)
	if !ok {
		log.Printf("🤔 Could not detect the %v address.", ipNet)
		return //nolint:nlreturn
	}

	if !c.Quiet {
		log.Printf("🧐 Detected the %v address: %v", ipNet, ip)
	}

	setIP(ctx, c, h, ipNet, ip)
}

func updateIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		updateIP(ctx, c, h, ipnet.IP4)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		updateIP(ctx, c, h, ipnet.IP6)
	}
}

func clearIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	if c.Policy[ipnet.IP4].IsManaged() {
		setIP(ctx, c, h, ipnet.IP4, nil)
	}

	if c.Policy[ipnet.IP6].IsManaged() {
		setIP(ctx, c, h, ipnet.IP6, nil)
	}
}
