package updater

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
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
			setter.ResponseFailed:   {"alpha.example"},
			setter.ResponseUpdating: {"beta.example"},
			setter.ResponseUpdated:  {"gamma.example"},
		}))
	})

	t.Run("updating-and-updated", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK: true,
			Lines: []string{
				"Clearing A records for alpha.example",
				"Cleared A records for beta.example",
			},
		}, generateClearHeartbeatMessage(ipnet.IP4, setterResponses{
			setter.ResponseUpdating: {"alpha.example"},
			setter.ResponseUpdated:  {"beta.example"},
		}))
	})
}

func TestGenerateClearNotifierMessage(t *testing.T) {
	t.Parallel()

	t.Run("failed-with-followups", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, notifier.Message{
			"Could not confirm that A records for alpha.example were cleared; clearing A records for beta.example; cleared A records for gamma.example.",
		}, generateClearNotifierMessage(ipnet.IP4, setterResponses{
			setter.ResponseFailed:   {"alpha.example"},
			setter.ResponseUpdating: {"beta.example"},
			setter.ResponseUpdated:  {"gamma.example"},
		}))
	})

	t.Run("empty-responses", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, generateClearNotifierMessage(ipnet.IP4, emptySetterResponses()))
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
			setter.ResponseFailed:   {"alpha.example"},
			setter.ResponseUpdating: {"beta.example"},
			setter.ResponseUpdated:  {"gamma.example"},
		}))
	})

	t.Run("updating-and-updated", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, heartbeat.Message{
			OK: true,
			Lines: []string{
				"Setting A records for alpha.example to 127.0.0.1, 127.0.0.2",
				"Set A records for beta.example to 127.0.0.1, 127.0.0.2",
			},
		}, generateUpdateHeartbeatMessage(ipnet.IP4, ip4Targets, setterResponses{
			setter.ResponseUpdating: {"alpha.example"},
			setter.ResponseUpdated:  {"beta.example"},
		}))
	})
}

func TestGenerateUpdateNotifierMessage(t *testing.T) {
	t.Parallel()

	ip4Targets := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
	}

	t.Run("registered-domain-descriptions", func(t *testing.T) {
		t.Parallel()

		responses := emptySetterResponses()
		responses.register(domain.FQDN("alpha.example"), setter.ResponseUpdated)
		responses.register(domain.FQDN("beta.example"), setter.ResponseUpdating)

		require.Equal(t, notifier.Message{
			"Updating A records for beta.example to 127.0.0.1 and 127.0.0.2; updated A records for alpha.example to 127.0.0.1 and 127.0.0.2.",
		}, generateUpdateNotifierMessage(ipnet.IP4, ip4Targets, responses))
	})

	t.Run("failed-with-followups", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, notifier.Message{
			"Could not confirm that A records for alpha.example were updated to 127.0.0.1 and 127.0.0.2; updating A records for beta.example to 127.0.0.1 and 127.0.0.2; updated A records for gamma.example to 127.0.0.1 and 127.0.0.2.",
		}, generateUpdateNotifierMessage(ipnet.IP4, ip4Targets, setterResponses{
			setter.ResponseFailed:   {"alpha.example"},
			setter.ResponseUpdating: {"beta.example"},
			setter.ResponseUpdated:  {"gamma.example"},
		}))
	})

	t.Run("empty-responses", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, generateUpdateNotifierMessage(ipnet.IP4, ip4Targets, emptySetterResponses()))
	})
}
