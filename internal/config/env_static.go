package config

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadStaticIPs reads a comma-separated list of static IP addresses.
func ReadStaticIPs(ppfmt pp.PP, key string, field *[]string) bool {
	val := Getenv(key)
	if val == "" {
		ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, "")
		*field = nil
		return true
	}

	// Split by comma and trim whitespace
	parts := strings.Split(val, ",")
	var ips []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			ips = append(ips, part)
		}
	}

	*field = ips
	return true
}

// ReadStaticIPMap reads environment variables IP4_STATIC and IP6_STATIC
// and consolidates the static IPs into a map.
func ReadStaticIPMap(ppfmt pp.PP, field *map[ipnet.Type][]string) bool {
	var ip4Static, ip6Static []string

	if !ReadStaticIPs(ppfmt, "IP4_STATIC", &ip4Static) ||
		!ReadStaticIPs(ppfmt, "IP6_STATIC", &ip6Static) {
		return false
	}

	*field = map[ipnet.Type][]string{
		ipnet.IP4: ip4Static,
		ipnet.IP6: ip6Static,
	}

	return true
}
