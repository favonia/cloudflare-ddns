package protocol

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
)

// This file contains mechanisms to limit connections to only IPv4 or IPv6.

var errForbiddenIPFamily = errors.New("forbidden IP family")

func filterIP6Only(_ context.Context, network, _ string, _ syscall.RawConn) error {
	switch network {
	case "tcp4", "udp4", "ip4":
		return errForbiddenIPFamily
	}
	return nil
}

func filterIP4Only(_ context.Context, network, _ string, _ syscall.RawConn) error {
	switch network {
	case "tcp6", "udp6", "ip6":
		return errForbiddenIPFamily
	}
	return nil
}

func newControlledDialer(control func(context.Context, string, string, syscall.RawConn) error) *net.Dialer {
	return &net.Dialer{ //nolint:exhaustruct
		Timeout:        30 * time.Second,
		KeepAlive:      30 * time.Second,
		ControlContext: control,
	}
}

func newControlledTransport(control func(context.Context, string, string, syscall.RawConn) error) http.RoundTripper {
	return &http.Transport{ //nolint:exhaustruct
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           newControlledDialer(control).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func newControlledClient(control func(context.Context, string, string, syscall.RawConn) error) *http.Client {
	return &http.Client{Transport: newControlledTransport(control)} //nolint:exhaustruct
}

//nolint:gochecknoglobals
var sharedSplitClient = map[ipnet.Type]*http.Client{
	ipnet.IP4: newControlledClient(filterIP4Only),
	ipnet.IP6: newControlledClient(filterIP6Only),
}

// SharedSplitClient returns the shared [http.Client] that allows only the traffic of specified IP family.
func SharedSplitClient(ipNet ipnet.Type) *http.Client {
	return sharedSplitClient[ipNet]
}

// SharedRetryableSplitClient returns a [retryablehttp.Client] with the shared underlying [http.Client]
// that allows only the traffic of specified IP family.
func SharedRetryableSplitClient(ipNet ipnet.Type) *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.HTTPClient = SharedSplitClient(ipNet)
	c.Logger = nil
	return c
}

// CloseIdleConnections closes all idle connections after making detecting the IP addresses.
func CloseIdleConnections() {
	for _, client := range ipnet.Bindings(sharedSplitClient) {
		client.CloseIdleConnections()
	}
}
