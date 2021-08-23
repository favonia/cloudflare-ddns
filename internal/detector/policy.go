package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Policy interface {
	name() string
	GetIP(context.Context, pp.PP, ipnet.Type) net.IP
}

func Name(p Policy) string {
	if p == nil {
		return "unmanaged"
	}

	return p.name()
}
