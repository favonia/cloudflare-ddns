package detector

import (
	"context"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Policy interface {
	name() string
	GetIP(context.Context, pp.PP, ipnet.Type) netip.Addr
}

func Name(p Policy) string {
	if p == nil {
		return "unmanaged"
	}

	return p.name()
}
