package detector

import (
	"context"
	"log"
	"net"
)

type Local struct{}

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return "local"
}

func (p *Local) GetIP4(ctx context.Context) (net.IP, bool) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		log.Printf(`😩 Could not detect a local IPv4 address: %v`, err)
		return nil, false //nolint:nlreturn
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), true
}

func (p *Local) GetIP6(ctx context.Context) (net.IP, bool) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		log.Printf(`😩 Could not detect a local IPv6 address: %v`, err)
		return nil, false //nolint:nlreturn
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), true
}
