package updater

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// renderDNS describes one scope block. Body and disclosure are computed together.
func renderDNS(row dnsRow) string {
	body, _ := renderDNSBody(row)
	heading := fmt.Sprintf("%s DNS for %s", row.family.Describe(),
		pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, row.domains, "(none)"))
	return heading + "\n" + strings.Join(body, "\n")
}

func renderAddresses(addresses []netip.Addr) string {
	return pp.EnglishJoinMapOrEmptyLabel(netip.Addr.String, addresses, "no addresses")
}

func sourceLabel(label string, cached bool) string {
	if cached {
		return label + " (cached)"
	}
	return label
}

func renderDNSBody(row dnsRow) ([]string, dnsDisclosure) {
	lines := []string{}
	disclosed := dnsDisclosure{addresses: nil, knownEmpty: false}
	if row.unexplained {
		lines = append(lines, "Detected an unexpected address change")
	}
	switch {
	case row.code == setter.ResponseNoop:
		label := "Current"
		if row.unexplained {
			label = "No " + row.family.Describe() + " update needed"
		}
		if row.beforeKnown {
			if row.unexplained {
				lines = append(lines, sourceLabel(label, row.beforeCached)+": "+renderAddresses(row.before))
			} else {
				lines = append(lines, renderDNSContext(row, false))
			}
			disclosed = dnsDisclosure{addresses: slices.Clone(row.before), knownEmpty: len(row.before) == 0}
		} else {
			lines = append(lines, "No update needed")
		}
	case row.code == setter.ResponseUpdated && row.evidence.deltaComplete:
		after := slices.Clone(row.satisfied)
		for _, item := range row.evidence.operations {
			if item.operation.Kind != api.RecordDelete {
				after = append(after, item.operation.Record.IP)
			}
		}
		after = canonicalAddresses(after)
		var added, removed, unchanged []netip.Addr
		for _, address := range row.before {
			if slices.Contains(after, address) {
				unchanged = append(unchanged, address)
			} else {
				removed = append(removed, address)
			}
		}
		for _, address := range after {
			if !slices.Contains(row.before, address) {
				added = append(added, address)
			}
		}
		lines = append(lines, renderDelta(addressStrings(added), addressStrings(removed),
			addressStrings(unchanged), row.beforeCached)...)
		// A complete Added-only delta discloses the explicitly known-empty input.
		disclosed = dnsDisclosure{addresses: slices.Concat(removed, unchanged), knownEmpty: len(row.before) == 0}
		lines = append(lines, completenessEnding("address", "addresses", len(added), len(removed), len(unchanged))...)
		if len(added)+len(removed)+len(unchanged) == 0 {
			lines = append(lines, renderDNSContext(row, false))
		}
	default:
		progress, represented := renderDNSProgress(row)
		lines = append(lines, progress...)
		disclosed.addresses = represented
		if !row.evidence.coveredBy(disclosed) && row.beforeKnown {
			lines = append(lines, renderDNSContext(row, row.evidence.inputHistorical))
			disclosed = dnsDisclosure{addresses: slices.Clone(row.before), knownEmpty: len(row.before) == 0}
		}
	}
	return lines, disclosed
}

func addressStrings(values []netip.Addr) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func englishValues(values []string) string {
	return pp.EnglishJoinMapOrEmptyLabel(func(s string) string { return s }, values, "")
}

// renderDelta accepts qualified content sets, never desired target sets.
func renderDelta(added, removed, unchanged []string, cached bool) []string {
	var lines []string
	if len(added) == 1 && len(removed) == 1 {
		lines = append(lines, "Updated: "+sourceLabel(removed[0], cached)+" → "+added[0])
	} else {
		if len(added) > 0 {
			lines = append(lines, "Added: "+englishValues(added))
		}
		if len(removed) > 0 {
			lines = append(lines, sourceLabel("Removed", cached)+": "+englishValues(removed))
		}
	}
	if len(unchanged) > 0 {
		lines = append(lines, sourceLabel("Unchanged", cached)+": "+englishValues(unchanged))
	}
	return lines
}

func dnsStatus(status setter.DNSOperationStatus) string {
	switch status {
	case setter.DNSOperationCompleted:
		return "confirmed"
	case setter.DNSOperationUnconfirmed:
		return "unconfirmed"
	case setter.DNSOperationBlocked:
		return "blocked"
	case setter.DNSOperationNotAttempted:
		return "not attempted"
	}
	return "unconfirmed"
}

// Operation pairs retain encounter order. Removal Other qualifications use only
// earlier trustworthy operation indices in this original scope.
func renderDNSProgress(row dnsRow) ([]string, []netip.Addr) {
	var lines []string
	var represented []netip.Addr
	var visible []int
	seen := make(map[setter.DNSOperation]bool)
	for i, item := range row.evidence.operations {
		op := item.operation
		if op.Kind == api.RecordDelete {
			continue
		}
		label := "Update "
		value := op.Record.IP.String()
		if op.Kind == api.RecordCreate {
			label = "Addition "
		} else if item.paired {
			value = sourceLabel(item.old.IP.String(), row.beforeCached) + " → " + value
			represented = append(represented, item.old.IP)
			visible = append(visible, i)
		}
		line := label + dnsStatus(op.Status) + ": " + value
		if !seen[op] {
			lines = append(lines, line)
			seen[op] = true
		}
		if op.Status == setter.DNSOperationCompleted && op.Membership == api.MembershipUnmanaged {
			lines = append(lines, "Record for "+op.Record.IP.String()+
				" excluded from management: its unexpected new comment does not match the filter.")
		}
	}
	for _, status := range []setter.DNSOperationStatus{
		setter.DNSOperationCompleted, setter.DNSOperationUnconfirmed,
		setter.DNSOperationBlocked, setter.DNSOperationNotAttempted,
	} {
		var addresses []netip.Addr
		for _, item := range row.evidence.operations {
			if item.operation.Kind == api.RecordDelete && item.operation.Status == status {
				addresses = append(addresses, item.operation.Record.IP)
			}
		}
		values := make([]string, 0, len(addresses))
		unqualified := make([]string, 0, len(addresses))
		for _, address := range canonicalAddresses(addresses) {
			var indices []int
			for i, item := range row.evidence.operations {
				if item.operation.Kind == api.RecordDelete && item.operation.Status == status &&
					item.operation.Record.IP == address {
					indices = append(indices, i)
				}
			}
			facts := row.evidence.removalFacts(indices, visible)
			value := address.String()
			if facts.countKnown && (!facts.omitOne || facts.other) {
				noun := "record"
				if facts.count != 1 {
					noun = "records"
				}
				if facts.other {
					noun = "other " + noun
				}
				value += fmt.Sprintf(" (%d %s)", facts.count, noun)
			}
			admitted := true
			for _, index := range indices {
				item := row.evidence.operations[index]
				admitted = admitted && item.identityKnown && item.old.ID != "" && item.old.IP == address
			}
			if admitted {
				values = append(values, value)
				represented = append(represented, address)
			} else {
				// Unassociated operation operands do not inherit snapshot provenance.
				unqualified = append(unqualified, value)
			}
		}
		if len(values) > 0 {
			lines = append(lines, sourceLabel("Removal "+dnsStatus(status), row.beforeCached)+": "+englishValues(values))
		}
		if len(unqualified) > 0 {
			lines = append(lines, "Removal "+dnsStatus(status)+": "+englishValues(unqualified))
		}
	}
	if len(row.satisfied) > 0 {
		lines = append(lines, sourceLabel("Unchanged", row.beforeCached)+": "+renderAddresses(row.satisfied))
		represented = append(represented, row.satisfied...)
	}
	if len(lines) == 0 {
		if row.cleanup || len(row.target) == 0 {
			lines = append(lines, "Cleanup blocked")
		} else {
			lines = append(lines, "Could not start the update to "+renderAddresses(row.target))
		}
	}
	return lines, represented
}

// completenessEnding supplements a complete content delta with the result-set
// fact that pure additions and complete removals do not express on their own.
func completenessEnding(singular, plural string, added, removed, retained int) []string {
	if removed == 0 && retained == 0 && added > 0 {
		if added == 1 {
			return []string{"This is the only managed " + singular + "."}
		}
		return []string{"These are all the managed " + plural + "."}
	}
	if added == 0 && retained == 0 && removed > 0 {
		return []string{"No managed " + plural + " remain."}
	}
	return nil
}

// historicalInventory identifies the admitted observation's time independently
// of whether the current operation succeeded. Empty input is a sentence.
func historicalInventory(noun string, values []string, cached, cleanup bool) string {
	when := "before the update"
	if cleanup {
		when = "before cleanup"
	}
	if len(values) == 0 {
		return sourceLabel("No "+strings.ToLower(noun)+" "+when, cached) + "."
	}
	return sourceLabel(noun+" "+when, cached) + ": " + englishValues(values)
}

func renderDNSContext(row dnsRow, historical bool) string {
	noun := "Addresses"
	if row.filtered {
		noun = "Managed addresses"
	}
	if historical {
		return historicalInventory(noun, addressStrings(row.before), row.beforeCached, row.cleanup)
	}
	if row.filtered {
		return sourceLabel("Current managed addresses", row.beforeCached) + ": " +
			pp.EnglishJoinMapOrEmptyLabel(netip.Addr.String, row.before, "none")
	}
	return sourceLabel("Current", row.beforeCached) + ": " + renderAddresses(row.before)
}

func renderProblem(p operationProblem) string {
	outcome := "address detection failed"
	switch p.kind {
	case problemDetection:
	case problemFiltering:
		outcome = "detection filter rejected all detected addresses"
	case problemDerivation:
		outcome = "DNS target derivation failed for " +
			pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, p.domains, "(none)")
	case problemMissingTarget:
		outcome = "DNS targets were unavailable for " +
			pp.EnglishJoinMapOrEmptyLabel(domain.Domain.Describe, p.domains, "(none)")
	}
	return p.family.Describe() + " " + outcome
}

func renderLifecycle(lifecycle LifecycleContext) string {
	switch lifecycle.Kind {
	case LifecycleStartup:
		if lifecycle.Schedule == "" {
			return "Cloudflare DDNS started"
		}
		return "Cloudflare DDNS started with schedule " + lifecycle.Schedule
	case LifecycleShutdown:
		return "Cloudflare DDNS stopped"
	case LifecycleSchedulingFailure:
		return "Cloudflare DDNS stopped because no future updates could be scheduled with " + lifecycle.Schedule
	case LifecycleNone:
		return ""
	}
	return ""
}
