package heartbeat_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// This catches accidentally restoring a private per-ping pool. A real local
// server is needed to observe connection sharing with ordinary HTTP traffic;
// the injected in-memory client bypasses that production construction path.
//
//nolint:paralleltest // Server.Close clears http.DefaultTransport idle connections.
func TestHeartbeatSharesDefaultPool(t *testing.T) {
	for _, name := range []string{"healthchecks", "uptime-kuma"} {
		t.Run(name, func(t *testing.T) {
			connections := make(chan string, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				connections <- r.RemoteAddr
				if name == "healthchecks" {
					_, _ = io.WriteString(w, "OK")
				} else {
					_, _ = io.WriteString(w, `{"ok":true}`)
				}
			}))
			t.Cleanup(server.Close)

			// Seed the ordinary pool, then use fresh heartbeat values for both pings.
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, resp.Body.Close())
			require.NoError(t, err)
			connection := <-connections

			for attempt := range 2 {
				ppfmt := pp.NewSilent()
				var h heartbeat.BasicHeartbeat
				var ok bool
				if name == "healthchecks" {
					h, ok = heartbeat.NewHealthchecks(ppfmt, server.URL)
				} else {
					h, ok = heartbeat.NewUptimeKuma(ppfmt, server.URL)
				}
				require.True(t, ok)
				require.True(t, h.Ping(t.Context(), ppfmt, heartbeat.NewMessage()))
				require.Equal(t, connection, <-connections, "ping %d", attempt)
			}
		})
	}
}
