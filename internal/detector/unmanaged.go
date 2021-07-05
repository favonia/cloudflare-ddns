package detector

import (
	"fmt"
	"net"
	"time"
)

type Unmanaged struct{}

func (p *Unmanaged) IsManaged() bool {
	return false
}

func (p *Unmanaged) String() string {
	return "unmanaged"
}

func (p *Unmanaged) GetIP4(timeout time.Duration) (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}

func (p *Unmanaged) GetIP6(timeout time.Duration) (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}
