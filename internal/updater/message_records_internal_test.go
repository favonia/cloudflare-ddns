package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestGenerateClearHeartbeatMessage(t *testing.T) {
	t.Parallel()

	t.Run("failed-with-followups", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK:    false,
			Lines: []string{"Could not confirm that A records for alpha.example were cleared"},
		}, generateClearHeartbeatMessage(ipnet.IP4, setterResponses{
			setter.ResponseFailed:  {"alpha.example"},
			setter.ResponseNoop:    {"beta.example"},
			setter.ResponseUpdated: {"gamma.example"},
		}))
	})

	t.Run("noop-and-updated", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK: true,
			Lines: []string{
				"Cleared A records for beta.example",
			},
		}, generateClearHeartbeatMessage(ipnet.IP4, setterResponses{
			setter.ResponseNoop:    {"alpha.example"},
			setter.ResponseUpdated: {"beta.example"},
		}))
	})
}

func TestGenerateUpdateHeartbeatMessage(t *testing.T) {
	t.Parallel()

	ip4Targets := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
	}

	t.Run("failed-with-followups", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK:    false,
			Lines: []string{"Could not confirm that A records for alpha.example were updated to 127.0.0.1, 127.0.0.2"},
		}, generateUpdateHeartbeatMessage(ipnet.IP4, ip4Targets, setterResponses{
			setter.ResponseFailed:  {"alpha.example"},
			setter.ResponseNoop:    {"beta.example"},
			setter.ResponseUpdated: {"gamma.example"},
		}))
	})

	t.Run("noop-and-updated", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK: true,
			Lines: []string{
				"Set A records for beta.example to 127.0.0.1, 127.0.0.2",
			},
		}, generateUpdateHeartbeatMessage(ipnet.IP4, ip4Targets, setterResponses{
			setter.ResponseNoop:    {"alpha.example"},
			setter.ResponseUpdated: {"beta.example"},
		}))
	})
}
