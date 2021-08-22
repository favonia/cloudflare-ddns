package api

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//go:generate mockgen -destination=../mocks/mock_api.go -package=mocks . Handle

type Handle interface {
	ListRecords(ctx context.Context, ppfmt pp.PP, domain FQDN, ipNet ipnet.Type) (map[string]net.IP, bool)
	DeleteRecord(ctx context.Context, ppfmt pp.PP, domain FQDN, ipNet ipnet.Type, id string) bool
	UpdateRecord(ctx context.Context, ppfmt pp.PP, domain FQDN, ipNet ipnet.Type, id string, ip net.IP) bool
	CreateRecord(ctx context.Context, ppfmt pp.PP, domain FQDN, ipNet ipnet.Type,
		ip net.IP, ttl TTL, proxied bool) (string, bool)
	FlushCache()
}
