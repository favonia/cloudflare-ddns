package api

import "net/netip"

// RecordFact is an owned value describing a record identity and address.
type RecordFact struct {
	ID ID
	IP netip.Addr
}

// MutationStatus describes evidence from a reached synchronous operation.
type MutationStatus uint8

const (
	// MutationNoEffect means the mutating request was not made.
	MutationNoEffect MutationStatus = iota
	// MutationUnconfirmed means a request failed and may already have taken effect.
	MutationUnconfirmed
	// MutationCompleted means the operation completed successfully.
	MutationCompleted
)

// RecordMutationKind identifies the operation; delete denotes absence.
type RecordMutationKind uint8

// Record mutation kinds.
const (
	RecordCreate RecordMutationKind = iota
	RecordUpdate
	RecordDelete
)

// ManagedMembership describes returned metadata under the handle's selector.
type ManagedMembership uint8

// Managed membership evidence values.
const (
	MembershipUnknown ManagedMembership = iota
	MembershipManaged
	MembershipUnmanaged
)

// RecordMutation owns its values. IP is the requested address for create/update;
// delete carries only the supplied ID. An unconfirmed create has no known ID.
// Membership on completed create/update reports returned metadata independently
// of cache admission, which retains existing policy. On unconfirmed create/update,
// MembershipManaged guarantees any resulting record is managed under a match-all
// selector; selective selectors remain unknown. This does not confirm existence.
// Prerequisite failures and deletions carry MembershipUnknown.
type RecordMutation struct {
	Status     MutationStatus
	Kind       RecordMutationKind
	Record     RecordFact
	Membership ManagedMembership
}
