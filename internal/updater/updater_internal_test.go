package updater

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestSetIPsSkipsManagedDomainWithoutTargets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(ctrl)
	s := mocks.NewMockSetter(ctrl)
	missing := domain.FQDN("missing.example")
	present := domain.FQDN("present.example")
	ip := netip.MustParseAddr("192.0.2.1")
	params := api.RecordParams{TTL: api.TTLAuto} //nolint:exhaustruct
	conf := &config.UpdateConfig{                //nolint:exhaustruct
		Domains:       map[ipnet.Family][]domain.Domain{ipnet.IP4: {missing, present}},
		Proxied:       map[domain.Domain]bool{},
		TTL:           api.TTLAuto,
		UpdateTimeout: time.Second,
	}

	gomock.InOrder(
		ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
			"No target set was provided for managed domain %s; this should not happen. Please report it at %s",
			missing.Describe(), pp.IssueReportingURL),
		s.EXPECT().SetIPs(gomock.Any(), ppfmt, ipnet.IP4, present, []netip.Addr{ip}, params).
			Return(setter.ResponseUpdated),
	)

	msg := setIPs(context.Background(), ppfmt, conf, s, ipnet.IP4, dnsTargetsByDomain{
		present: {ip},
	})

	require.Equal(t, Message{
		HeartbeatMessage: heartbeat.Message{
			OK:    false,
			Lines: []string{"Could not update A records for missing.example because no target set was provided"},
		},
		NotifierMessage: notifier.Message{
			"Could not update A records for missing.example because no target set was provided.",
			"Updated A records for present.example to 192.0.2.1.",
		},
	}, msg)
}
