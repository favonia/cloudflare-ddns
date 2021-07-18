package detector

import (
	"context"
	"fmt"
	"net"
)

type Unmanaged struct{}

func (p *Unmanaged) IsManaged() bool {
	return false
}

func (p *Unmanaged) String() string {
	return "unmanaged"
}

func (p *Unmanaged) GetIP4(ctx context.Context) (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}

func (p *Unmanaged) GetIP6(ctx context.Context) (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}
