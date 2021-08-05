package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP(context.Context, pp.Indent, ipnet.Type) (net.IP, bool)
}
