// Package updater implements the logic to detect and update IP addresses,
// combining the packages setter and provider.
package updater

import (
	"context"
	"fmt"
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

// setIP extracts relevant settings from the configuration and calls [setter.Setter.Set] with timeout.
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

// clearIP extracts relevant settings from the configuration and calls [setter.Setter.Clear] with a deadline.
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

// ShouldDisplayHelpMessages determines whether help messages should be displayed.
// The help messages are to help beginners detect possible misconfiguration.
// These messages should be displayed at most once, and thus the value of this map
// will be changed to false after displaying the help messages.
//
//nolint:gochecknoglobals
var ShouldDisplayHelpMessages = map[ipnet.Type]bool{
	ipnet.IP4: true,
	ipnet.IP6: true,
}

func detectIP(ctx context.Context, ppfmt pp.PP,
	c *config.Config, ipNet ipnet.Type, use1001 bool,
) (netip.Addr, bool, string) {
	ctx, cancel := context.WithTimeout(ctx, c.DetectionTimeout)
	defer cancel()

	msg := ""
	ip, ok := c.Provider[ipNet].GetIP(ctx, ppfmt, ipNet, use1001)
	if ok {
		ppfmt.Infof(pp.EmojiInternet, "Detected the %s address: %v", ipNet.Describe(), ip)
	} else {
		msg = fmt.Sprintf("Failed to detect the %s address", ipNet.Describe())
		ppfmt.Errorf(pp.EmojiError, "%s", msg)
		if ShouldDisplayHelpMessages[ipNet] {
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
	ShouldDisplayHelpMessages[ipNet] = false
	return ip, ok, msg
}

// UpdateIPs detect IP addresses and update DNS records of managed domains.
func UpdateIPs(ctx context.Context, ppfmt pp.PP, c *config.Config, s setter.Setter) (bool, string) {
	allOk := true

	// [msgs[false]] collects all the error messages and [msgs[true]] collects all the success messages.
	msgs := map[bool][]string{}

	for _, ipNet := range [...]ipnet.Type{ipnet.IP4, ipnet.IP6} {
		if c.Provider[ipNet] != nil {
			ip, ok, msg := detectIP(ctx, ppfmt, c, ipNet, c.Use1001)
			if len(msg) != 0 {
				msgs[ok] = append(msgs[ok], msg)
			}
			if !ok {
				// We can't detect the new IP address. It's probably better to leave existing IP addresses alone.
				allOk = false
				continue
			}

			ok, msg = setIP(ctx, ppfmt, c, s, ipNet, ip)
			if len(msg) != 0 {
				msgs[ok] = append(msgs[ok], msg)
			}
			allOk = allOk && ok
		}
	}

	var allMsg string
	if len(msgs[false]) != 0 {
		allMsg = strings.Join(msgs[false], "\n")
	} else {
		allMsg = strings.Join(msgs[true], "\n")
	}
	return allOk, allMsg
}

// ClearIPs removes all DNS records of managed domains.
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
