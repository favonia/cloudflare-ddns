// Package config reads and parses configurations.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

const itemTitleWidth = 24

func describeDomains(domains []domain.Domain) string {
	if len(domains) == 0 {
		return "(none)"
	}

	descriptions := make([]string, 0, len(domains))
	for _, domain := range domains {
		descriptions = append(descriptions, domain.Describe())
	}
	return strings.Join(descriptions, ", ")
}

func getInverseMap[V comparable](m map[domain.Domain]V) ([]V, map[V][]domain.Domain) {
	inverse := map[V][]domain.Domain{}

	for dom, val := range m {
		inverse[val] = append(inverse[val], dom)
	}

	vals := make([]V, 0, len(inverse))
	for val := range inverse {
		domain.SortDomains(inverse[val])
		vals = append(vals, val)
	}

	return vals, inverse
}

// Print prints the Config on the screen.
func (c *Config) Print(ppfmt pp.PP) {
	if !ppfmt.IsEnabledFor(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.IncIndent()
	inner := ppfmt.IncIndent()

	section := func(title string) { ppfmt.Infof(pp.EmojiConfig, title) }
	item := func(title string, format string, values ...any) {
		inner.Infof(pp.EmojiBullet, "%-*s %s", itemTitleWidth, title, fmt.Sprintf(format, values...))
	}

	section("Domains and IP providers:")
	if c.Provider[ipnet.IP4] != nil {
		item("IPv4 domains:", "%s", describeDomains(c.Domains[ipnet.IP4]))
		item("IPv4 provider:", "%s", provider.Name(c.Provider[ipnet.IP4]))
	}
	if c.Provider[ipnet.IP6] != nil {
		item("IPv6 domains:", "%s", describeDomains(c.Domains[ipnet.IP6]))
		item("IPv6 provider:", "%s", provider.Name(c.Provider[ipnet.IP6]))
	}

	section("Scheduling:")
	item("Timezone:", "%s", cron.DescribeLocation(time.Local))
	item("Update frequency:", "%s", cron.DescribeSchedule(c.UpdateCron))
	item("Update on start?", "%t", c.UpdateOnStart)
	item("Delete on stop?", "%t", c.DeleteOnStop)
	item("Cache expiration:", "%v", c.CacheExpiration)

	section("New DNS records:")
	item("TTL:", "%s", c.TTL.Describe())
	if len(c.Proxied) > 0 {
		_, inverseMap := getInverseMap(c.Proxied)
		item("Proxied domains:", "%s", describeDomains(inverseMap[true]))
		item("Unproxied domains:", "%s", describeDomains(inverseMap[false]))
	}

	section("Timeouts:")
	item("IP detection:", "%v", c.DetectionTimeout)
	item("Record updating:", "%v", c.UpdateTimeout)

	if len(c.Monitors) > 0 {
		section("Monitors:")
		monitor.DescribeAll(func(service, params string) {
			item(service+":", "%s", params)
		}, c.Monitors)
	}

	if len(c.Notifiers) > 0 {
		section("Notifiers (via shoutrrr):")
		monitor.DescribeAll(func(service, params string) {
			item(service+":", "%s", params)
		}, c.Monitors)
	}
}
