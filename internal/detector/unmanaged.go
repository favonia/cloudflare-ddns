package detector

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type unmanaged struct{}

func (p *unmanaged) IsManaged() bool {
	return false
}

func (p *unmanaged) String() string {
	return "unmanaged"
}

func (p *unmanaged) GetIP(_ context.Context, _ pp.Indent, _ ipnet.Type) (net.IP, bool) {
	return nil, false
}

func NewUnmanaged() Policy {
	return &unmanaged{}
}
