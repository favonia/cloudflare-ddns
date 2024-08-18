// Package api implements protocols to update DNS records.
package api

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -typed -destination=../mocks/mock_api.go -package=mocks . Handle

// ID is a new type representing identifiers to avoid programming mistakes.
type ID string

// Describe converts an ID to a string.
func (id ID) Describe() string { return string(id) }

// WAFList represents a WAF list to update.
type WAFList struct {
	AccountID ID
	ListName  string
}

// Describe formats WAFList as a string.
func (l WAFList) Describe() string { return fmt.Sprintf("%s/%s", string(l.AccountID), l.ListName) }

// Record bundles an ID and an IP address, representing a DNS record.
type Record struct {
	ID ID
	IP netip.Addr
}

// WAFListItem bundles an ID and an IP range, representing an item in a WAF list.
type WAFListItem struct {
	ID     ID
	Prefix netip.Prefix
}

// A Handle represents a generic API to update DNS records and WAF lists.
// Currently, the only implementation is Cloudflare.
type Handle interface {
	// ListRecords lists all matching DNS records.
	//
	// The second return value indicates whether the list was cached.
	ListRecords(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain) ([]Record, bool, bool)

	// DeleteRecord deletes one DNS record.
	DeleteRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID) bool

	// UpdateRecord updates one DNS record.
	UpdateRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain, id ID, ip netip.Addr) bool

	// CreateRecord creates one DNS record. It returns the ID of the new record.
	CreateRecord(ctx context.Context, ppfmt pp.PP, ipNet ipnet.Type, domain domain.Domain,
		ip netip.Addr, ttl TTL, proxied bool, recordComment string) (ID, bool)

	// EnsureWAFList creates an empty WAF list with IP ranges if it does not already exist yet.
	// The first return value is the ID of the list.
	// The second return value indicates whether the list already exists.
	EnsureWAFList(ctx context.Context, ppfmt pp.PP, list WAFList, description string) (ID, bool, bool)

	// DeleteWAFList deletes a WAF list with IP ranges.
	DeleteWAFList(ctx context.Context, ppfmt pp.PP, list WAFList) bool

	// ListWAFListItems retrieves a WAF list with IP rages.
	//
	// The second return value indicates whether the list was cached.
	ListWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList) ([]WAFListItem, bool, bool)

	// DeleteWAFListItems deletes IP ranges from a WAF list.
	DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, ids []ID) bool

	// CreateWAFListItems adds IP ranges to a WAF list.
	CreateWAFListItems(ctx context.Context, ppfmt pp.PP, list WAFList, items []netip.Prefix, comment string) bool
}

// An Auth contains authentication information.
type Auth interface {
	// New uses the authentication information to create a Handle.
	New(ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool)
}
