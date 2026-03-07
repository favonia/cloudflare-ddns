package config

import (
	"fmt"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/cron"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// Keep titles aligned for the longest built-in key:
// "WAF list item comment regex:".
const itemTitleWidth = 28

// These helpers format values for the human-facing startup summary.
// They are display helpers, not serialization helpers.
// Show literal text as quoted text so humans can see exact boundaries and
// escaping. The empty string gets a dedicated label because that is easier to
// scan than two quote characters in logs.
func describeLiteralText(s string) string {
	return pp.QuoteOrEmptyLabel(s, "(empty)")
}

// Ownership regex settings are RE2 regexes, not literal comments. Show non-empty
// regexes in the form humans usually read regexes: raw RE2 syntax when that stays
// readable on one line, and a quoted fallback when escaping or whitespace would
// otherwise be ambiguous.
func describeNonemptyCommentRegex(regex string) string {
	return pp.QuoteIfNotHumanReadable(regex)
}

func describeDNSRecordCommentRegex(regex string) string {
	if regex == "" {
		return "(empty regex; manages all DNS records)"
	}
	return describeNonemptyCommentRegex(regex)
}

func describeWAFListItemCommentRegex(regex string) string {
	if regex == "" {
		return "(empty regex; manages all WAF list items)"
	}
	return describeNonemptyCommentRegex(regex)
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

// Print prints a human-facing summary of the validated config and the reporting
// services currently wired into the process.
func Print(ppfmt pp.PP, built *BuiltConfig, hb heartbeat.Heartbeat, nt notifier.Notifier) {
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

	handle := built.Handle
	lifecycle := built.Lifecycle
	update := built.Update

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
	managedWAFListItemsCommentRegex := ""
	if handle.Options.ManagedWAFListItemsCommentRegex != nil {
		managedWAFListItemsCommentRegex = handle.Options.ManagedWAFListItemsCommentRegex.String()
	}

	// Hide inactive filters to keep the default output focused.
	if managedRecordsCommentRegex != "" || managedWAFListItemsCommentRegex != "" {
		section("Ownership filters:")
		// These regexes select which existing DNS records and WAF list items this
		// instance considers managed.
		if managedRecordsCommentRegex != "" {
			item("DNS record comment regex:", "%s", describeDNSRecordCommentRegex(managedRecordsCommentRegex))
		}
		if managedWAFListItemsCommentRegex != "" {
			item("WAF list item comment regex:", "%s", describeWAFListItemCommentRegex(managedWAFListItemsCommentRegex))
		}
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
	item("WAF list item comment:", "%s", describeLiteralText(update.WAFListItemComment))

	section("Timeouts:")
	item("IP detection:", "%v", update.DetectionTimeout)
	item("Record/list updating:", "%v", update.UpdateTimeout)

	if hb != nil {
		count := 0
		for name, params := range hb.Describe {
			count++
			if count == 1 {
				section("Heartbeats:")
			}
			item(name+":", "%s", params)
		}
	}

	if nt != nil {
		count := 0
		for name, params := range nt.Describe {
			count++
			if count == 1 {
				section("Notification services (via shoutrrr):")
			}
			item(name+":", "%s", params)
		}
	}
}
