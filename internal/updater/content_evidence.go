package updater

import (
	"net/netip"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

// dnsEvidence retains one original scope's identity evidence before grouping.
// Pairing is based on consistent nonempty IDs, never address-set ordering.
// Conflicting local identity witnesses disable claims; they do not alter cleanup.
type dnsEvidence struct {
	records       []api.RecordFact
	operations    []dnsOperationEvidence
	deltaComplete bool
	known         bool
	// inputHistorical distinguishes known-empty input before any possible write
	// from historical empty input after a completed or unconfirmed mutation.
	inputHistorical     bool
	snapshotTrustworthy bool
}

type dnsOperationEvidence struct {
	operation     setter.DNSOperation
	old           api.RecordFact
	paired        bool
	identityKnown bool
}

func makeDNSEvidence(o dnsOutcome) dnsEvidence {
	e := dnsEvidence{
		records: nil, operations: nil, deltaComplete: false, known: o.result.Before != nil,
		snapshotTrustworthy: true, inputHistorical: false,
	}
	before := make(map[api.ID]api.RecordFact)
	conflicts := make(map[api.ID]bool)
	if o.result.Before != nil {
		e.records = slices.Clone(o.result.Before.Records)
		for i := range e.records {
			record := &e.records[i]
			record.IP = record.IP.Unmap()
			previous, exists := before[record.ID]
			if record.ID == "" || !record.IP.IsValid() || exists && previous != *record {
				conflicts[record.ID] = true
				e.snapshotTrustworthy = false
			}
			before[record.ID] = *record
		}
	}
	if o.result.Progress == nil {
		return e
	}
	seen := make(map[api.ID]setter.DNSOperation)
	for _, operation := range o.result.Progress.Operations {
		operation.Record.IP = operation.Record.IP.Unmap()
		e.inputHistorical = e.inputHistorical || operation.Status == setter.DNSOperationCompleted ||
			operation.Status == setter.DNSOperationUnconfirmed
		previous, exists := seen[operation.Record.ID]
		if exists && previous != operation {
			conflicts[operation.Record.ID] = true
		}
		seen[operation.Record.ID] = operation
	}
	represented := make(map[api.ID]bool)
	for _, operation := range o.result.Progress.Operations {
		operation.Record.IP = operation.Record.IP.Unmap()
		old, exists := before[operation.Record.ID]
		identity := operation.Record.ID != "" && !conflicts[operation.Record.ID] && operation.Record.IP.IsValid()
		if operation.Kind == api.RecordDelete && exists && old.IP != operation.Record.IP {
			identity = false
		}
		paired := identity && exists && operation.Kind == api.RecordUpdate
		e.operations = append(e.operations, dnsOperationEvidence{
			operation: operation, old: old, paired: paired, identityKnown: identity,
		})
		if identity && exists && operation.Status == setter.DNSOperationCompleted {
			if operation.Kind == api.RecordDelete && operation.Record.IP == old.IP ||
				paired && operation.Membership == api.MembershipManaged {
				represented[old.ID] = true
			}
		}
	}
	e.deltaComplete = e.known && e.snapshotTrustworthy
	for _, item := range e.operations {
		if !item.identityKnown || item.operation.Status != setter.DNSOperationCompleted {
			e.deltaComplete = false
		}
		switch item.operation.Kind {
		case api.RecordDelete:
			e.deltaComplete = e.deltaComplete && item.old.ID != "" && item.old.IP == item.operation.Record.IP
		case api.RecordUpdate:
			e.deltaComplete = e.deltaComplete && item.paired && item.operation.Membership == api.MembershipManaged
		case api.RecordCreate:
			e.deltaComplete = e.deltaComplete && item.old.ID == "" && item.operation.Membership == api.MembershipManaged
		}
	}
	for _, record := range e.records {
		_, operated := seen[record.ID]
		unchanged := !operated && slices.Contains(o.result.Progress.Satisfied, record.IP)
		if !represented[record.ID] && !unchanged {
			e.deltaComplete = false
		}
	}
	return e
}

type dnsRemovalFacts struct {
	count      int
	countKnown bool
	omitOne    bool
	other      bool
}

// removalFacts describes one IP/status group. visibleUpdates names earlier
// per-record update lines actually emitted in this same original scope/block.
// Current, Before, content-level Updated and prior removals are never anchors.
// Callers must not share visibleUpdates across scopes, rounds or blocks.
func (e dnsEvidence) removalFacts(indices, visibleUpdates []int) dnsRemovalFacts {
	facts := dnsRemovalFacts{count: 0, countKnown: len(indices) > 0, omitOne: false, other: false}
	ids := make(map[api.ID]bool)
	var address netip.Addr
	var status setter.DNSOperationStatus
	earliest := len(e.operations)
	for _, index := range indices {
		if index < 0 || index >= len(e.operations) {
			facts.countKnown = false
			continue
		}
		item := e.operations[index]
		operation := item.operation
		if !item.identityKnown || operation.Kind != api.RecordDelete {
			facts.countKnown = false
		}
		if address.IsValid() && (address != operation.Record.IP || status != operation.Status) {
			facts.countKnown = false
		}
		address, status = operation.Record.IP, operation.Status
		ids[operation.Record.ID] = true
		earliest = min(earliest, index)
	}
	if !facts.countKnown {
		return facts
	}
	facts.count = len(ids)
	snapshotIDs := make(map[api.ID]bool)
	for _, record := range e.records {
		if record.IP == address {
			snapshotIDs[record.ID] = true
		}
	}
	facts.omitOne = facts.count == 1 && e.known && e.snapshotTrustworthy && len(snapshotIDs) == 1
	for id := range ids {
		facts.omitOne = facts.omitOne && snapshotIDs[id]
	}
	// Every qualifying visible update must be disjoint from every removal.
	for _, index := range visibleUpdates {
		if index < 0 || index >= earliest {
			continue
		}
		anchor := e.operations[index]
		if !anchor.paired || anchor.old.IP != address {
			continue
		}
		if ids[anchor.operation.Record.ID] {
			facts.other = false
			return facts
		}
		facts.other = true
	}
	return facts
}

// dnsDisclosure is the renderer's explicit account of admitted input shown in
// the block. It may combine old operands, removals, unchanged facts and fallback
// context. knownEmpty must be set by an explicit empty-input disclosure.
type dnsDisclosure struct {
	addresses  []netip.Addr
	knownEmpty bool
}

func (e dnsEvidence) coveredBy(disclosure dnsDisclosure) bool {
	if !e.known {
		return false
	}
	if len(e.records) == 0 {
		return disclosure.knownEmpty
	}
	for _, record := range e.records {
		if !record.IP.IsValid() || !slices.Contains(disclosure.addresses, record.IP) {
			return false
		}
	}
	return true
}
