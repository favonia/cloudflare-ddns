// Package api implements protocols to update DNS records and WAF lists.
package api

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"regexp"
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
}

// Record represents a DNS record.
type Record struct {
	ID           ID
	IP           netip.Addr
	RecordParams //nolint:embeddedstructfieldcheck // parameters go last
}

// WAFListItem represents one WAF list item: ID, IP range, and original comment.
type WAFListItem struct {
	ID
	netip.Prefix

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
	CacheExpiration                   time.Duration
	ManagedRecordsCommentRegex        *regexp.Regexp
	ManagedWAFListItemsCommentRegex   *regexp.Regexp
	AllowWholeWAFListDeleteOnShutdown bool
}

// A Handle represents a generic API to update DNS records and WAF lists.
// Currently, the only implementation is Cloudflare.
type Handle interface {
	// ListRecords lists managed DNS records matching the given domain/IP-family scope.
	// The managed-record selector is bound into the handle options because
	// implementations may cache filtered records by domain/IP-family scope.
	//
	// The second return value indicates whether the list was cached.
	ListRecords(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain,
		expectedParams RecordParams,
	) ([]Record, bool, bool)

	// UpdateRecord updates one DNS record.
	UpdateRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain,
		id ID, ip netip.Addr, currentParams, expectedParams RecordParams,
	) bool

	// CreateRecord creates one DNS record. It returns the ID of the new record.
	CreateRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain,
		ip netip.Addr, params RecordParams) (ID, bool)

	// DeleteRecord deletes one DNS record, assuming we will not update or create any DNS records.
	DeleteRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, mode DeletionMode) bool

	// ListWAFListItems returns managed WAF list items with their IP ranges.
	// It creates the list if it does not exist.
	//
	// The managed-item selector is bound into the handle options because
	// implementations may cache filtered items by WAF-list scope.
	// expectedItemComment is the configured comment target for newly created
	// managed list items; implementations may use it for advisory mismatch hints.
	//
	// The second return value indicates whether the list already exists.
	// The third return value indicates whether the list content was cached.
	ListWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, expectedDescription, expectedItemComment string,
	) ([]WAFListItem, bool, bool, bool)

	// FinalCleanWAFList removes managed WAF content during shutdown.
	// Implementations choose whole-list or managed-item cleanup from the handle's
	// bound ownership policy.
	//
	// The handle should not be reused for any further update operations after
	// calling this method.
	FinalCleanWAFList(ctx context.Context, ppfmt pp.PP, list WAFList,
		expectedDescription string,
	) WAFListCleanupCode

	// DeleteWAFListItems deletes IP ranges from a WAF list.
	DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, expectedDescription string,
		expectedItemComment string, ids []ID) bool

	// CreateWAFListItems adds IP ranges to a WAF list.
	CreateWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, expectedDescription string,
		items []netip.Prefix, comment string) bool
}

// An Auth contains authentication information.
type Auth interface {
	// New uses the authentication information to create a Handle.
	New(ppfmt pp.PP, options HandleOptions) (Handle, bool)
}
