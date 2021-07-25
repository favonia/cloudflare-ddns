package detector

import (
	"context"
	"fmt"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
)

type Local struct {
	Net ipnet.Type
}

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return "local"
}

func (p *Local) getIP4(_ context.Context) (net.IP, bool) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		fmt.Printf("😩 Could not detect a local IPv4 address: %v\n", err)
		return nil, false
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), true
}

func (p *Local) getIP6(_ context.Context) (net.IP, bool) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		fmt.Printf("😩 Could not detect a local IPv6 address: %v\n", err)
		return nil, false
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), true
}

func (p *Local) GetIP(ctx context.Context) (net.IP, bool) {
	switch p.Net {
	case ipnet.IP4:
		return p.getIP4(ctx)
	case ipnet.IP6:
		return p.getIP6(ctx)
	default:
		return nil, false
	}
}
