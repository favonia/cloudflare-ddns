package api

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type DomainSplitter interface {
	IsValid() bool
	ZoneNameASCII() string
	Next()
}

type Domain interface {
	DNSNameASCII() string
	Describe() string
	Split() DomainSplitter
}

//go:generate mockgen -destination=../mocks/mock_api.go -package=mocks . Handle

type Handle interface {
	ListRecords(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type) (map[string]netip.Addr, bool)
	DeleteRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type, id string) bool
	UpdateRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type, id string, ip netip.Addr) bool
	CreateRecord(ctx context.Context, ppfmt pp.PP, domain Domain, ipNet ipnet.Type,
		ip netip.Addr, ttl TTL, proxied bool) (string, bool)
	FlushCache()
}
