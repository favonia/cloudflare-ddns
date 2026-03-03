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

// Ownership regex settings are RE2 regexes, not literal comments. Show them in
// the form humans usually read regexes: raw RE2 syntax when that stays
// readable on one line, and a quoted fallback when escaping or whitespace would
// otherwise be ambiguous.
func describeCommentRegex(regex string) string {
	if regex == "" {
		return "(empty; matches all comments)"
	}
	if isHumanReadableRegex(regex) {
		return regex
	}
	return strconv.Quote(regex)
}

func isHumanReadableRegex(regex string) bool {
	if strings.TrimSpace(regex) != regex {
		return false
	}
	for _, r := range regex {
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

// Print prints a human-facing summary of the validated handle, lifecycle, and
// update configs.
func Print(ppfmt pp.PP, handle *HandleConfig, lifecycle *LifecycleConfig, update *UpdateConfig) {
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
	for ipNet, p := range ipnet.Bindings(update.Provider) {
		if p != nil {
			item(ipNet.Describe()+"-enabled domains:", "%s", pp.JoinMap(domain.Domain.Describe, update.Domains[ipNet]))
			item(ipNet.Describe()+" provider:", "%s", provider.Name(p))
		}
	}
	item("WAF lists:", "%s", pp.JoinMap(api.WAFList.Describe, update.WAFLists))

	managedRecordsCommentRegex := ""
	if handle.Options.ManagedRecordsCommentRegex != nil {
		managedRecordsCommentRegex = handle.Options.ManagedRecordsCommentRegex.String()
	}

	// Hide inactive filters to keep the default output focused.
	if managedRecordsCommentRegex != "" {
		section("Ownership filters:")
		// This regex selects which existing DNS records this instance considers managed.
		item("DNS record comment regex:", "%s", describeCommentRegex(managedRecordsCommentRegex))
	}

	section("Scheduling:")
	item("Timezone:", "%s", cron.DescribeLocation(time.Local))
	item("Update schedule:", "%s", cron.DescribeSchedule(lifecycle.UpdateCron))
	item("Update on start?", "%t", lifecycle.UpdateOnStart)
	item("Delete on stop?", "%t", lifecycle.DeleteOnStop)
	item("Cache expiration:", "%v", handle.Options.CacheExpiration)

	section("Parameters of new DNS records and WAF lists:")
	// These settings are defaults or targets for managed objects when creating or updating.
	item("TTL:", "%s", update.TTL.Describe())
	{
		_, inverseMap := computeInverseMap(update.Proxied)
		item("Proxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[true]))
		item("Unproxied domains:", "%s", pp.JoinMap(domain.Domain.Describe, inverseMap[false]))
	}
	item("DNS record comment:", "%s", describeLiteralText(update.RecordComment))
	item("WAF list description:", "%s", describeLiteralText(update.WAFListDescription))

	section("Timeouts:")
	item("IP detection:", "%v", update.DetectionTimeout)
	item("Record/list updating:", "%v", update.UpdateTimeout)

	if lifecycle.Monitor != nil {
		count := 0
		for name, params := range lifecycle.Monitor.Describe {
			count++
			if count == 1 {
				section("Monitors:")
			}
			item(name+":", "%s", params)
		}
	}

	if lifecycle.Notifier != nil {
		count := 0
		for name, params := range lifecycle.Notifier.Describe {
			count++
			if count == 1 {
				section("Notification services (via shoutrrr):")
			}
			item(name+":", "%s", params)
		}
	}
}
