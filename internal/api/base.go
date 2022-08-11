package api

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// A DomainSplitter enumerates all possible zones from a domain.
type DomainSplitter interface {
	// IsValid checks whether the current splitting point is still valid
	IsValid() bool
	// ZoneNameASCII gives the suffix (the zone), when it is still valid
	ZoneNameASCII() string
	// Next moves to the next possible splitting point, which might end up being invalid
	Next()
}

// A Domain represents a domain name to update.
type Domain interface {
	// DNSNameASCII gives a name suitable for accessing the Cloudflare API
	DNSNameASCII() string
	// Describe gives the most human-readable domain name that is still unambiguous
	Describe() string
	// Split gives a DomainSplitter that can be used to find zones
	Split() DomainSplitter
}

//go:generate mockgen -destination=../mocks/mock_api.go -package=mocks . Handle

// Handle represents a generic API to update DNS records. Currently, the only implementation is Cloudflare.
type Handle interface {
	ListRecords(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type) (map[string]netip.Addr, bool)
	DeleteRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type, id string) bool
	UpdateRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type, id string, ip netip.Addr) bool
	CreateRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type,
		ip netip.Addr, ttl TTL, proxied bool) (string, bool)
	FlushCache()
}
