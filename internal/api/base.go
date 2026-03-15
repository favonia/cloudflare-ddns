// Package api implements protocols to update DNS records and WAF lists.
package api

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate go tool mockgen -typed -destination=../mocks/mock_api.go -package=mocks . Handle

// ID is a new type representing identifiers to avoid programming mistakes.
type ID string

func (id ID) String() string { return string(id) }

// WAFList represents a WAF list to update.
type WAFList struct {
	AccountID ID
	Name      string
}

// Describe formats WAFList as a string.
func (l WAFList) Describe() string { return fmt.Sprintf("%s/%s", string(l.AccountID), l.Name) }

// CompareWAFList compares two WAF lists first by account ID and then by list name.
func CompareWAFList(l1, l2 WAFList) int {
	return cmp.Or(
		cmp.Compare(l1.AccountID, l2.AccountID),
		cmp.Compare(l1.Name, l2.Name),
	)
}

// RecordParams bundles parameters of a DNS record.
type RecordParams struct {
	TTL     TTL
	Proxied bool
	Comment string
	Tags    []string
}

// Record represents a DNS record.
type Record struct {
	ID           ID
	IP           netip.Addr
	RecordParams //nolint:embeddedstructfieldcheck // parameters go last
}

// WAFListItem represents one WAF list item: ID, IP range, and original comment.
type WAFListItem struct {
	ID      ID
	Prefix  netip.Prefix
	Comment string
}

// WAFListCreateItem represents one WAF list item to create.
type WAFListCreateItem struct {
	Prefix  netip.Prefix
	Comment string
}

// WAFListCleanupCode summarizes final shutdown cleanup for one WAF list.
type WAFListCleanupCode int

const (
	// WAFListCleanupNoop means the managed WAF content was already gone.
	WAFListCleanupNoop WAFListCleanupCode = iota

	// WAFListCleanupUpdated means the managed WAF content was removed synchronously.
	WAFListCleanupUpdated

	// WAFListCleanupUpdating means WAF cleanup was started asynchronously.
	WAFListCleanupUpdating

	// WAFListCleanupFailed means shutdown cleanup did not finish successfully.
	WAFListCleanupFailed
)

// DeletionMode tells the deletion updater whether a careful re-reading of lists
// must be enforced if an error happens.
type DeletionMode bool

const (
	// RegularDelitionMode enables re-reading when an error occurs.
	RegularDelitionMode DeletionMode = false
	// FinalDeletionMode disables re-reading when an error occurs.
	FinalDeletionMode DeletionMode = true
)

// HandleOptions bundles handle-scoped settings that affect cache correctness
// and other per-handle behavior.
type HandleOptions struct {
	CacheExpiration time.Duration
	HandleOwnershipPolicy
}

// A Handle represents a generic API to update DNS records and WAF lists.
// Currently, the only implementation is Cloudflare.
type Handle interface {
	// ListRecords lists managed DNS records matching the given domain/IP-family scope.
	// The managed-record selector is bound into the handle options because
	// implementations may cache filtered records by domain/IP-family scope.
	//
	// The second return value indicates whether the list was cached.
	ListRecords(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain,
		configuredParams RecordParams,
	) ([]Record, bool, bool)

	// UpdateRecord reconciles one managed DNS record to the desired state.
	//
	// Implementations must apply the desired DNS content and metadata in
	// desiredParams for this record:
	// - content/IP: ip
	// - ttl/proxied/comment/tags: desiredParams
	UpdateRecord(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain,
		id ID, ip netip.Addr, desiredParams RecordParams,
	) bool

	// CreateRecord creates one managed DNS record with the given desired metadata.
	// It returns the ID of the new record.
	CreateRecord(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain,
		ip netip.Addr, desiredParams RecordParams) (ID, bool)

	// DeleteRecord deletes one managed DNS record by ID.
	//
	// mode controls cache invalidation behavior for failure handling.
	DeleteRecord(ctx context.Context, ppfmt pp.PP, ipFamily ipnet.Family, domain domain.Domain, id ID, mode DeletionMode) bool

	// ListWAFListItems returns managed WAF list items with their IP ranges.
	// It creates the list if it does not exist.
	//
	// The managed-item selector is bound into the handle options because
	// implementations may cache filtered items by WAF-list scope.
	// configuredItemComment is the configured comment target for newly created
	// managed list items; implementations may use it for advisory mismatch hints.
	//
	// The second return value indicates whether the list already exists.
	// The third return value indicates whether the list content was cached.
	ListWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, configuredDescription, configuredItemComment string,
	) ([]WAFListItem, bool, bool, bool)

	// FinalCleanWAFList removes managed WAF content during shutdown.
	// Implementations choose whole-list or managed-item cleanup from the handle's
	// bound ownership policy and the in-scope family set for the run.
	//
	// The handle should not be reused for any further update operations after
	// calling this method.
	FinalCleanWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
		configuredDescription string, managedFamilies map[ipnet.Family]bool,
	) WAFListCleanupCode

	// DeleteWAFListItems deletes managed WAF list items by item IDs.
	DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, configuredDescription string, ids []ID) bool

	// CreateWAFListItems creates managed WAF list items with the given prefixes
	// and per-item comments.
	CreateWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, configuredDescription string,
		items []WAFListCreateItem) bool
}

// An Auth contains authentication information.
type Auth interface {
	// New uses the authentication information to create a Handle.
	New(ppfmt pp.PP, options HandleOptions) (Handle, bool)

	// CheckUsability performs an early credential usability check.
	//
	// It returns false only for conclusive credential failures. Ambiguous
	// failures such as timeouts still return true after logging a warning, so
	// temporary outages do not block startup.
	CheckUsability(ctx context.Context, ppfmt pp.PP) bool
}
