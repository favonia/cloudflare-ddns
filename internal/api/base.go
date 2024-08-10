// Package api implements protocols to update DNS records.
package api

import (
	"context"
	"net/netip"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -typed -destination=../mocks/mock_api.go -package=mocks . Handle

// Record bundles an ID and an IP address, representing a DNS record.
type Record struct {
	ID string
	IP netip.Addr
}

// WAFListItem bundles an ID and an IP range, representing an item in a WAF list.
type WAFListItem struct {
	ID     string
	Prefix netip.Prefix
}

// A Handle represents a generic API to update DNS records and WAF lists.
// Currently, the only implementation is Cloudflare.
type Handle interface {
	// Perform basic checking (e.g., the validity of tokens).
	// It returns false when we should give up all future operations.
	SanityCheck(ctx context.Context, ppfmt pp.PP) bool

	// ListRecords lists all matching DNS records.
	//
	// The second return value indicates whether the list was cached.
	ListRecords(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type) ([]Record, bool, bool)

	// DeleteRecord deletes one DNS record.
	DeleteRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type, id string) bool

	// UpdateRecord updates one DNS record.
	UpdateRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type, id string, ip netip.Addr) bool

	// CreateRecord creates one DNS record. It returns the ID of the new record.
	CreateRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type,
		ip netip.Addr, ttl TTL, proxied bool, recordComment string) (string, bool)

	// EnsureWAFList creates an empty WAF list with IP ranges if it does not already exist yet.
	// The first return value is the ID of the list.
	// The second return value indicates whether the list already exists.
	EnsureWAFList(ctx context.Context, ppfmt pp.PP, listName string, description string) (string, bool, bool)

	// DeleteWAFList deletes a WAF list with IP ranges.
	DeleteWAFList(ctx context.Context, ppfmt pp.PP, listName string) bool

	// ListWAFListItems retrieves a WAF list with IP rages.
	//
	// The second return value indicates whether the list was cached.
	ListWAFListItems(ctx context.Context, ppfmt pp.PP, listName string) ([]WAFListItem, bool, bool)

	// DeleteWAFListItems deletes IP ranges from a WAF list.
	DeleteWAFListItems(ctx context.Context, ppfmt pp.PP, listName string, ids []string) bool

	// CreateWAFListItems adds IP ranges to a WAF list.
	CreateWAFListItems(ctx context.Context, ppfmt pp.PP, listName string, items []netip.Prefix, comment string) bool
}

// An Auth contains authentication information.
type Auth interface {
	// New uses the authentication information to create a Handle.
	New(ctx context.Context, ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool)

	// Check whether DNS records are supported.
	SupportsRecords() bool

	// Check whether WAF lists are supported.
	SupportsWAFLists() bool
}
