package detector

import (
	"context"
	"fmt"
	"net"
)

type Local struct{}

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return "local"
}

func (p *Local) GetIP4(ctx context.Context) (net.IP, error) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		return nil, fmt.Errorf(`😩 Could not detect a local IPv4 address: %v`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), nil
}

func (p *Local) GetIP6(ctx context.Context) (net.IP, error) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		return nil, fmt.Errorf(`😩 Could not detect a local IPv6 address: %v`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), nil
}
