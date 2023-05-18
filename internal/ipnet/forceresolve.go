package ipnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"

	"github.com/hashicorp/go-retryablehttp"
)

var errNoIP = errors.New("no IP addresses were found")

func ResolveHostname(ctx context.Context, ipNet Type, host string) (netip.Addr, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, ipNet.IPNetwork(), host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("couldn't resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return netip.Addr{}, errNoIP
	}

	// The current Go runtime just try the list in sequence without randomization.
	// Therefore, it is probably okay to just return the first IP address.
	ip := ips[0]

	// A temporary fix before https://go-review.googlesource.com/c/go/+/415580 fully lands
	if ipNet == IP4 && ip.Is4In6() {
		ip = ip.Unmap()
	}

	return ip, nil
}

func ForceResolveURLHost(ctx context.Context, ipNet Type, u *url.URL) bool {
	hostname := u.Hostname()

	// If it's already an IP address, skip the test.
	// This does not take into account URLs such as https://0/
	if _, err := netip.ParseAddr(hostname); err != nil {
		// Resolve the host name.
		ip, err := ResolveHostname(ctx, ipNet, hostname)
		if err != nil {
			return false
		}
		hostname = ip.String()
	}

	// If the port is empty, we have a few choices due to issue #14836:
	//
	// 1. Remove the trailing ':' after calling net.JoinHostPort
	// 2. Reimplement net.JoinHostPort but handle the empty port specially
	// 3. Look up the port number by the scheme
	//
	// The following code chooses method 2. The standard library removes
	// the trailing ':' when net.NewRquestWithContext is called.
	host := net.JoinHostPort(hostname, u.Port())
	if host[len(host)-1] == ':' {
		host = host[:len(host)-1]
	}

	// Replace the URL host with the resolved one.
	u.Host = host
	return true
}

func ForceResolveRetryableRequest(ctx context.Context, ipNet Type, r *retryablehttp.Request) bool {
	originalHost := r.Host
	if originalHost == "" {
		originalHost = r.URL.Host
	}

	if !ForceResolveURLHost(ctx, ipNet, r.URL) {
		return false
	}
	r.Host = originalHost
	return true
}
