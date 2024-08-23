// Package config reads and parses configurations.
package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

const itemTitleWidth = 24

func describeComment(c string) string {
	if c == "" {
		return "(empty)"
	}
	return strconv.Quote(c)
}

func computeInverseMap[V comparable](m map[domain.Domain]V) ([]V, map[V][]domain.Domain) {
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
	if !ppfmt.IsShowing(pp.Info) {
		return
	}

	ppfmt.Infof(pp.EmojiEnvVars, "Current settings:")
	ppfmt = ppfmt.Indent()
	inner := ppfmt.Indent()

	section := func(title string) { ppfmt.Infof(pp.EmojiConfig, "%s", title) }
	item := func(title string, format string, values ...any) {
		inner.Infof(pp.EmojiBullet, "%-*s %s", itemTitleWidth, title, fmt.Sprintf(format, values...))
	}

	section("Domains, IP providers, and WAF lists:")
	if c.Provider[ipnet.IP4] != nil {
		item("IPv4-enabled domains:", "%s", pp.JoinMap(domain.Domain.Describe, c.Domains[ipnet.IP4]))
		item("IPv4 provider:", "%s", provider.Name(c.Provider[ipnet.IP4]))
	}
	if c.Provider[ipnet.IP6] != nil {
		item("IPv6-enabled domains:", "%s", pp.JoinMap(domain.Domain.Describe, c.Domains[ipnet.IP6]))
		item("IPv6 provider:", "%s", provider.Name(c.Provider[ipnet.IP6]))
	}
	item("WAF lists:", "%s", pp.JoinMap(api.WAFList.Describe, c.WAFLists))

	section("Scheduling:")
	item("Timezone:", "%s", cron.DescribeLocation(time.Local))
	item("Update schedule:", "%s", cron.DescribeSchedule(c.UpdateCron))
	item("Update on start?", "%t", c.UpdateOnStart)
	item("Delete on stop?", "%t", c.DeleteOnStop)
	item("Cache expiration:", "%v", c.CacheExpiration)

	section("Parameters of new DNS records and WAF lists:")
	item("TTL:", "%s", c.TTL.Describe())
	{
		_, inverseMap := computeInverseMap(c.Proxied)
		item("Proxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[true]))
		item("Unproxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[false]))
	}
	item("DNS record comment:", "%s", describeComment(c.RecordComment))
	item("WAF list description:", "%s", describeComment(c.WAFListDescription))

	section("Timeouts:")
	item("IP detection:", "%v", c.DetectionTimeout)
	item("Record/list updating:", "%v", c.UpdateTimeout)

	if len(c.Monitors) > 0 {
		section("Monitors:")
		monitor.DescribeAll(func(service, params string) {
			item(service+":", "%s", params)
		}, c.Monitors)
	}

	if len(c.Notifiers) > 0 {
		section("Notification services (via shoutrrr):")
		notifier.DescribeAll(func(service, params string) {
			item(service+":", "%s", params)
		}, c.Notifiers)
	}
}
