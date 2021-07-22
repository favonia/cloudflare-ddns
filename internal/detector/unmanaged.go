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

func (p *Unmanaged) GetIP(_ context.Context) (net.IP, bool) {
	return nil, false
}
