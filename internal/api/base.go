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

// A Handle represents a generic API to update DNS records. Currently, the only implementation is Cloudflare.
type Handle interface {
	// Perform basic checking. It returns false when we should give up
	// all future operations.
	SanityCheck(ctx context.Context, ppfmt pp.PP) bool

	// ListRecords lists all matching DNS records.
	//
	// The second return value indicates whether the list was cached.
	ListRecords(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type) (map[string]netip.Addr, bool, bool) //nolint:lll

	// DeleteRecord deletes one DNS record.
	DeleteRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type, id string) bool

	// UpdateRecord updates one DNS record.
	UpdateRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type, id string, ip netip.Addr) bool

	// CreateRecord creates one DNS record.
	CreateRecord(ctx context.Context, ppfmt pp.PP, domain domain.Domain, ipNet ipnet.Type,
		ip netip.Addr, ttl TTL, proxied bool, recordComment string) (string, bool)

	// FlushCache flushes the API cache. Flushing should automatically happen when other operations encounter errors.
	FlushCache()
}

// An Auth contains authentication information.
type Auth interface {
	// New uses the authentication information to create a Handle.
	New(ctx context.Context, ppfmt pp.PP, cacheExpiration time.Duration) (Handle, bool)
}
