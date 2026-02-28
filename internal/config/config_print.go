package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Keep titles aligned for the longest built-in key:
// "DNS record comment regex:".
const itemTitleWidth = 28

// These helpers format values for the human-facing startup summary.
// They are display helpers, not serialization helpers.
// Show literal text as quoted text so humans can see exact boundaries and
// escaping. The empty string gets a dedicated label because that is easier to
// scan than two quote characters in logs.
func describeLiteralText(s string) string {
	if s == "" {
		return "(empty)"
	}
	return strconv.Quote(s)
}

// MANAGED_RECORDS_COMMENT_REGEX is a RE2 template, not a literal comment.
// Show regex filters in the form humans usually read them: raw RE2 syntax when
// that stays readable on one line, and a quoted fallback when escaping or
// whitespace would otherwise be ambiguous.
func describeCommentRegexTemplate(template string) string {
	if template == "" {
		return "(empty; matches all comments)"
	}
	if isHumanReadableRegex(template) {
		return template
	}
	return strconv.Quote(template)
}

func isHumanReadableRegex(template string) bool {
	if strings.TrimSpace(template) != template {
		return false
	}
	for _, r := range template {
		if unicode.IsGraphic(r) || r == ' ' {
			continue
		}
		return false
	}
	return true
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

// Print prints a human-facing summary of [Config].
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
	for ipNet, p := range ipnet.Bindings(c.Provider) {
		if p != nil {
			item(ipNet.Describe()+"-enabled domains:", "%s", pp.JoinMap(domain.Domain.Describe, c.Domains[ipNet]))
			item(ipNet.Describe()+" provider:", "%s", provider.Name(p))
		}
	}
	item("WAF lists:", "%s", pp.JoinMap(api.WAFList.Describe, c.WAFLists))

	// Hide inactive filters to keep the default output focused.
	if c.ManagedRecordsCommentRegexTemplate != "" {
		section("Ownership filters:")
		// This regex selects which existing DNS records this instance considers managed.
		item("DNS record comment regex:", "%s", describeCommentRegexTemplate(c.ManagedRecordsCommentRegexTemplate))
	}

	section("Scheduling:")
	item("Timezone:", "%s", cron.DescribeLocation(time.Local))
	item("Update schedule:", "%s", cron.DescribeSchedule(c.UpdateCron))
	item("Update on start?", "%t", c.UpdateOnStart)
	item("Delete on stop?", "%t", c.DeleteOnStop)
	item("Cache expiration:", "%v", c.CacheExpiration)

	section("Parameters of new DNS records and WAF lists:")
	// These settings are defaults/targets for managed objects when creating or updating.
	item("TTL:", "%s", c.TTL.Describe())
	{
		_, inverseMap := computeInverseMap(c.Proxied)
		item("Proxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[true]))
		item("Unproxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[false]))
	}
	item("DNS record comment:", "%s", describeLiteralText(c.RecordComment))
	item("WAF list description:", "%s", describeLiteralText(c.WAFListDescription))

	section("Timeouts:")
	item("IP detection:", "%v", c.DetectionTimeout)
	item("Record/list updating:", "%v", c.UpdateTimeout)

	if c.Monitor != nil {
		count := 0
		for name, params := range c.Monitor.Describe {
			count++
			if count == 1 {
				section("Monitors:")
			}
			item(name+":", "%s", params)
		}
	}

	if c.Notifier != nil {
		count := 0
		for name, params := range c.Notifier.Describe {
			count++
			if count == 1 {
				section("Notification services (via shoutrrr):")
			}
			item(name+":", "%s", params)
		}
	}
}
