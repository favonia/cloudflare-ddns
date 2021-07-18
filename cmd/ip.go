package main

import (
	"context"
	"log"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/config"
)

func setIPs(ctx context.Context, c *config.Config, h *api.Handle, ip4 net.IP, ip6 net.IP) {
	for _, target := range c.Targets {
		ctx, cancel := context.WithTimeout(ctx, c.APITimeout)
		_ = h.Update(&api.UpdateArgs{
			Context:    ctx,
			Quiet:      c.Quiet,
			Target:     target,
			IP4Managed: c.IP4Policy.IsManaged(),
			IP4:        ip4,
			IP6Managed: c.IP6Policy.IsManaged(),
			IP6:        ip6,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
		})
		cancel()
	}
	for _, target := range c.IP4Targets {
		ctx, cancel := context.WithTimeout(ctx, c.APITimeout)
		_ = h.Update(&api.UpdateArgs{
			Context:    ctx,
			Quiet:      c.Quiet,
			Target:     target,
			IP4Managed: c.IP4Policy.IsManaged(),
			IP4:        ip4,
			IP6Managed: false,
			IP6:        nil,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
		})
		cancel()
	}
	for _, target := range c.IP6Targets {
		ctx, cancel := context.WithTimeout(ctx, c.APITimeout)
		_ = h.Update(&api.UpdateArgs{
			Context:    ctx,
			Quiet:      c.Quiet,
			Target:     target,
			IP4Managed: false,
			IP4:        nil,
			IP6Managed: c.IP6Policy.IsManaged(),
			IP6:        ip6,
			TTL:        c.TTL,
			Proxied:    c.Proxied,
		})
		cancel()
	}
}

func detectIPs(ctx context.Context, c *config.Config, h *api.Handle) (ip4 net.IP, ip6 net.IP) {
	if c.IP4Policy.IsManaged() {
		ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
		ip, ok := c.IP4Policy.GetIP4(ctx)
		cancel()
		if !ok {
			log.Printf("🤔 Could not detect the IPv4 address.")
		} else {
			if !c.Quiet {
				log.Printf("🧐 Detected the IPv4 address: %v", ip.To4())
			}
			ip4 = ip
		}
	}

	if c.IP6Policy.IsManaged() {
		ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
		ip, ok := c.IP6Policy.GetIP6(ctx)
		cancel()
		if !ok {
			log.Printf("🤔 Could not detect the IPv6 address.")
		} else {
			if !c.Quiet {
				log.Printf("🧐 Detected the IPv6 address: %v", ip.To16())
			}
			ip6 = ip
		}
	}

	return ip4, ip6
}

func updateIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	ip4, ip6 := detectIPs(ctx, c, h)
	setIPs(ctx, c, h, ip4, ip6)
}

func clearIPs(ctx context.Context, c *config.Config, h *api.Handle) {
	setIPs(ctx, c, h, nil, nil)
}
