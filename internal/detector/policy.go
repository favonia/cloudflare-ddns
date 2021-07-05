package detector

import (
	"fmt"
	"net"
)

type Policy interface {
	IsManaged() bool
	String() string
	GetIP4() (net.IP, error)
	GetIP6() (net.IP, error)
}

type Unmanaged struct{}

func (p *Unmanaged) IsManaged() bool {
	return false
}

func (p *Unmanaged) String() string {
	return "unmanaged"
}

func (p *Unmanaged) GetIP4() (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}

func (p *Unmanaged) GetIP6() (net.IP, error) {
	return nil, fmt.Errorf("😱 The impossible happened!")
}

type Local struct{}

func (p *Local) IsManaged() bool {
	return true
}

func (p *Local) String() string {
	return "local"
}

func (p *Local) GetIP4() (net.IP, error) {
	conn, err := net.Dial("udp4", "1.1.1.1:443")
	if err != nil {
		return nil, fmt.Errorf(`😩 Could not detect a local IPv4 address: %v`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To4(), nil
}

func (p *Local) GetIP6() (net.IP, error) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:443")
	if err != nil {
		return nil, fmt.Errorf(`😩 Could not detect a local IPv6 address: %v`, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.To16(), nil
}

type Cloudflare struct{}

func (p *Cloudflare) IsManaged() bool {
	return true
}

func (p *Cloudflare) String() string {
	return "cloudflare"
}

func (p *Cloudflare) GetIP4() (net.IP, error) {
	ip, err := getIPFromCloudflare("https://1.1.1.1/cdn-cgi/trace")
	if err == nil {
		return ip.To4(), nil
	} else {
		return nil, err
	}
}

func (p *Cloudflare) GetIP6() (net.IP, error) {
	ip, err := getIPFromCloudflare("https://[2606:4700:4700::1111]/cdn-cgi/trace")
	if err == nil {
		return ip.To16(), nil
	} else {
		return nil, err
	}
}
