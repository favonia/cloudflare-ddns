package updater

import (
	"context"
	"net/netip"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

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

// setIP extracts relevant settings from the configuration and calls setter.Set with timeout.
// ip must be non-zero.
func setIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, s setter.Setter, ipNet ipnet.Type, ip netip.Addr,
) (bool, string) {
	allOk := true
	var msgs []string

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		ok, msg := s.Set(ctx, ppfmt, domain, ipNet, ip, c.TTL, getProxied(ppfmt, c, domain))
		allOk = allOk && ok
		if msg != "" {
			msgs = append(msgs, msg)
		}
	}

	return allOk, strings.Join(msgs, "\n")
}

// clearIP extracts relevant settings from the config struct and calls setter.Clear with a deadline.
// ip must be non-zero.
func clearIP(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter, ipNet ipnet.Type) (bool, string) {
	allOk := true
	var msgs []string

	for _, domain := range c.Domains[ipNet] {
		ctx, cancel := context.WithTimeout(ctx, c.UpdateTimeout)
		defer cancel()

		ok, msg := s.Clear(ctx, ppfmt, domain, ipNet)
		allOk = allOk && ok
		if msg != "" {
			msgs = append(msgs, msg)
		}
	}

	return allOk, strings.Join(msgs, "\n")
}

var MessageShouldDisplay = map[ipnet.Type]bool{ipnet.IP4: true, ipnet.IP6: true} //nolint:gochecknoglobals

func detectIP(ctx context.Context, ppfmt pp.PP, c *config.Config, ipNet ipnet.Type) (netip.Addr, bool) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet)
	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		ppfmt.Errorf(pp.EmojiError, "Failed to detect the %s address", ipNet.Describe())
		if MessageShouldDisplay[ipNet] {
			switch ipNet {
			case ipnet.IP6:
				ppfmt.Infof(pp.EmojiConfig, "If you are using Docker or Kubernetes, IPv6 often requires additional setups")     //nolint:lll
				ppfmt.Infof(pp.EmojiConfig, "Read more about IPv6 networks at https://github.com/favonia/cloudflare-ddns")      //nolint:lll
				ppfmt.Infof(pp.EmojiConfig, "If your network does not support IPv6, you can disable it with IP6_PROVIDER=none") //nolint:lll
			case ipnet.IP4:
				ppfmt.Infof(pp.EmojiConfig, "If your network does not support IPv4, you can disable it with IP4_PROVIDER=none") //nolint:lll
			}
		}
	}
	MessageShouldDisplay[ipNet] = false
	return ip, ok
}

func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) (bool, string) {
	allOk := true
	var msgs []string

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ip, ok := detectIP(ctx, ppfmt, c, ipNet)
			if !ok {
				// We can't detect the new IP address. It's probably better to leave existing IP addresses alone.
				allOk = false
				continue
			}

			ok, msg := setIP(ctx, ppfmt, c, s, ipNet, ip)
			allOk = allOk && ok
			if msg != "" {
				msgs = append(msgs, msg)
			}
		}
	}

	return allOk, strings.Join(msgs, "\n")
}

func ClearIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) (bool, string) {
	allOk := true
	var msgs []string

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ok, msg := clearIP(ctx, ppfmt, c, s, ipNet)
			allOk = allOk && ok
			if msg != "" {
				msgs = append(msgs, msg)
			}
		}
	}

	return allOk, strings.Join(msgs, "\n")
}
