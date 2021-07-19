package detector

import (
	"context"
	"net"
)

type Unmanaged struct{}

func (p *Unmanaged) IsManaged() bool {
	return false
}

func (p *Unmanaged) String() string {
	return "unmanaged"
}

func (p *Unmanaged) GetIP4(ctx context.Context) (net.IP, bool) {
	return nil, false
}

func (p *Unmanaged) GetIP6(ctx context.Context) (net.IP, bool) {
	return nil, false
}
