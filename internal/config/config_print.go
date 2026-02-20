package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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
	for _, ipNet := range []ipnet.Type{ipnet.IP4, ipnet.IP6} {
		p := c.Provider[ipNet]
		if p != nil {
			item(ipNet.Describe()+"-enabled domains:", "%s", pp.JoinMap(domain.Domain.Describe, c.Domains[ipNet]))
			item(ipNet.Describe()+" provider:", "%s", provider.Name(p))
			if len(c.StaticIPs[ipNet]) > 0 {
				item(ipNet.Describe()+" static IPs:", "%s", strings.Join(c.StaticIPs[ipNet], ", "))
			}
		}
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

	if c.Monitor != nil {
		section("Monitors:")
		c.Monitor.Describe(func(name, params string) bool {
			item(name+":", "%s", params)
			return true
		})
	}

	if c.Notifier != nil {
		section("Notification services (via shoutrrr):")
		c.Notifier.Describe(func(name, params string) bool {
			item(name+":", "%s", params)
			return true
		})
	}
}
