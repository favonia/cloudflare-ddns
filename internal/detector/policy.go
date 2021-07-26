package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP(context.Context, pp.Indent, ipnet.Type) (net.IP, bool)
}
