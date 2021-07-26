package api

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Handle interface {
	ListRecords(ctx context.Context, indent pp.Indent,
		domain FQDN, ipNet ipnet.Type) (map[string]net.IP, bool)
	DeleteRecord(ctx context.Context, indent pp.Indent,
		domain FQDN, ipNet ipnet.Type, id string) bool
	UpdateRecord(ctx context.Context, indent pp.Indent,
		domain FQDN, ipNet ipnet.Type, id string, ip net.IP) bool
	CreateRecord(ctx context.Context, indent pp.Indent,
		domain FQDN, ipNet ipnet.Type, ip net.IP, ttl int, proxied bool) (string, bool)
	FlushCache()
}
