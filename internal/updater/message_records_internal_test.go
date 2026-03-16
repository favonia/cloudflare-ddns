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

func TestGenerateUpdateHeartbeatMessage(t *testing.T) {
	t.Parallel()

	ip4Targets := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
	}

	cases := []struct {
		name string
		ips  []netip.Addr
		resp setterResponses
		want heartbeat.Message
	}{
		{
			name: "clearing-updating-and-updated",
			ips:  nil,
			resp: setterResponses{
				setter.ResponseUpdating: {"alpha.example"},
				setter.ResponseUpdated:  {"beta.example"},
			},
			want: heartbeat.Message{
				OK: true,
				Lines: []string{
					"Clearing A of alpha.example",
					"Cleared A of beta.example",
				},
			},
		},
		{
			name: "clearing-failed",
			ips:  nil,
			resp: setterResponses{
				setter.ResponseFailed: {"alpha.example", "beta.example"},
			},
			want: heartbeat.Message{
				OK:    false,
				Lines: []string{"Could not confirm clearing A of alpha.example, beta.example"},
			},
		},
		{
			name: "non-clearing-updating-only",
			ips:  ip4Targets,
			resp: setterResponses{
				setter.ResponseUpdating: {"alpha.example", "beta.example"},
			},
			want: heartbeat.Message{
				OK:    true,
				Lines: []string{"Setting A (127.0.0.1, 127.0.0.2) of alpha.example, beta.example"},
			},
		},
		{
			name: "non-clearing-updated-only",
			ips:  ip4Targets,
			resp: setterResponses{
				setter.ResponseUpdated: {"alpha.example", "beta.example"},
			},
			want: heartbeat.Message{
				OK:    true,
				Lines: []string{"Set A (127.0.0.1, 127.0.0.2) of alpha.example, beta.example"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, generateUpdateHeartbeatMessage(ipnet.IP4, tc.ips, tc.resp))
		})
	}
}

func TestGenerateUpdateNotifierMessage(t *testing.T) {
	t.Parallel()

	ip4Targets := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
		netip.MustParseAddr("127.0.0.2"),
	}

	cases := []struct {
		name string
		ips  []netip.Addr
		resp setterResponses
		want notifier.Message
	}{
		{
			name: "updating-only",
			ips:  ip4Targets,
			resp: setterResponses{
				setter.ResponseUpdating: {"alpha.example", "beta.example"},
			},
			want: notifier.Message{
				"Updating A records of alpha.example and beta.example with 127.0.0.1 and 127.0.0.2.",
			},
		},
		{
			name: "updated-only",
			ips:  ip4Targets,
			resp: setterResponses{
				setter.ResponseUpdated: {"alpha.example", "beta.example"},
			},
			want: notifier.Message{
				"Updated A records of alpha.example and beta.example with 127.0.0.1 and 127.0.0.2.",
			},
		},
		{
			name: "clearing-updating-only",
			ips:  nil,
			resp: setterResponses{
				setter.ResponseUpdating: {"alpha.example", "beta.example"},
			},
			want: notifier.Message{
				"Clearing A records of alpha.example and beta.example.",
			},
		},
		{
			name: "clearing-updated-only",
			ips:  nil,
			resp: setterResponses{
				setter.ResponseUpdated: {"alpha.example", "beta.example"},
			},
			want: notifier.Message{
				"Cleared A records of alpha.example and beta.example.",
			},
		},
		{
			name: "registered-domain-descriptions",
			ips:  ip4Targets,
			resp: func() setterResponses {
				responses := emptySetterResponses()
				responses.register(domain.FQDN("alpha.example"), setter.ResponseUpdated)
				responses.register(domain.FQDN("beta.example"), setter.ResponseUpdating)
				return responses
			}(),
			want: notifier.Message{
				"Updating A records of beta.example with 127.0.0.1 and 127.0.0.2; updated those of alpha.example.",
			},
		},
		{
			name: "clearing-failed-with-followups",
			ips:  nil,
			resp: setterResponses{
				setter.ResponseFailed:   {"alpha.example"},
				setter.ResponseUpdating: {"beta.example"},
				setter.ResponseUpdated:  {"gamma.example"},
			},
			want: notifier.Message{
				"Could not confirm clearing A records of alpha.example; updating those of beta.example; updated those of gamma.example.",
			},
		},
		{
			name: "empty-responses",
			ips:  ip4Targets,
			resp: emptySetterResponses(),
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, generateUpdateNotifierMessage(ipnet.IP4, tc.ips, tc.resp))
		})
	}
}
