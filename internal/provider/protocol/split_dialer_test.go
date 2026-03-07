package protocol_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
)

func mustListen(ipNet ipnet.Type) net.Listener {
	switch ipNet {
	case ipnet.IP4:
		l, err := net.Listen("tcp4", "127.0.0.1:0") //nolint:noctx // net.Listen has no context-aware variant.
		if err != nil {
			panic(err)
		}
		return l
	case ipnet.IP6:
		l, err := net.Listen("tcp6", "[::1]:0") //nolint:noctx // net.Listen has no context-aware variant.
		if err != nil {
			panic(err)
		}
		return l
	default:
		return nil
	}
}

func newSplitServer(ipNet ipnet.Type, h http.HandlerFunc) *httptest.Server {
	s := &httptest.Server{ //nolint:exhaustruct
		Listener: mustListen(ipNet),
		Config:   &http.Server{Handler: h, ReadHeaderTimeout: time.Minute}, //nolint:exhaustruct
	}
	s.Start()
	return s
}

func TestSharedSplitClient(t *testing.T) {
	t.Parallel()

	server := map[ipnet.Type]*httptest.Server{
		ipnet.IP4: newSplitServer(ipnet.IP4, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, "ip4")
		})),
		ipnet.IP6: newSplitServer(ipnet.IP6, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, "ip6")
		})),
	}
	t.Cleanup(server[ipnet.IP4].Close)
	t.Cleanup(server[ipnet.IP6].Close)

	for name, tc := range map[string]struct {
		dialerNet ipnet.Type
		serverNet ipnet.Type
		ok        bool
		output    []byte
	}{
		"4":    {ipnet.IP4, ipnet.IP4, true, []byte("ip4")},
		"6":    {ipnet.IP6, ipnet.IP6, true, []byte("ip6")},
		"4to6": {ipnet.IP4, ipnet.IP6, false, []byte{}},
		"6to4": {ipnet.IP6, ipnet.IP4, false, []byte{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := protocol.SharedSplitClient(tc.dialerNet)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server[tc.serverNet].URL, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			output := []byte{}
			if err == nil {
				output, _ = io.ReadAll(resp.Body)
				defer resp.Body.Close()
			}

			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tc.output, output)
		})
	}
}
