package config_test

// vim: nowrap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // environment vars are global
func TestSetupReportersHeartbeat(t *testing.T) {
	for name, tc := range map[string]struct {
		setHealthchecks bool
		healthchecks    string
		setUptimeKuma   bool
		uptimeKuma      string
		ok              bool
		prepareMockPP   func(*mocks.MockPP)
		check           func(*testing.T, heartbeat.Heartbeat, notifier.Notifier)
	}{
		"empty": {
			setHealthchecks: false,
			healthchecks:    "",
			setUptimeKuma:   false,
			uptimeKuma:      "",
			ok:              true,
			prepareMockPP:   nil,
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"healthchecks": {
			setHealthchecks: true,
			healthchecks:    "https://hi.org/1234",
			setUptimeKuma:   false,
			uptimeKuma:      "",
			ok:              true,
			prepareMockPP:   nil,
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t,
					heartbeat.NewComposed(heartbeat.Healthchecks{
						BaseURL: urlMustParse(t, "https://hi.org/1234"),
						Timeout: heartbeat.HealthchecksDefaultTimeout,
					}),
					hb,
				)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"uptime-kuma": {
			setHealthchecks: false,
			healthchecks:    "",
			setUptimeKuma:   true,
			uptimeKuma:      "https://hi.org/1234",
			ok:              true,
			prepareMockPP:   nil,
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t,
					heartbeat.NewComposed(heartbeat.UptimeKuma{
						BaseURL: urlMustParse(t, "https://hi.org/1234"),
						Timeout: heartbeat.UptimeKumaDefaultTimeout,
					}),
					hb,
				)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"both": {
			setHealthchecks: true,
			healthchecks:    "https://healthchecks.example/1234",
			setUptimeKuma:   true,
			uptimeKuma:      "https://uptime.example/1234",
			ok:              true,
			prepareMockPP:   nil,
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t,
					heartbeat.NewComposed(
						heartbeat.Healthchecks{
							BaseURL: urlMustParse(t, "https://healthchecks.example/1234"),
							Timeout: heartbeat.HealthchecksDefaultTimeout,
						},
						heartbeat.UptimeKuma{
							BaseURL: urlMustParse(t, "https://uptime.example/1234"),
							Timeout: heartbeat.UptimeKumaDefaultTimeout,
						},
					),
					hb,
				)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"invalid-healthchecks": {
			setHealthchecks: true,
			healthchecks:    "\001",
			setUptimeKuma:   false,
			uptimeKuma:      "",
			ok:              false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `Failed to parse the Healthchecks URL (redacted)`)
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
		"invalid-uptime-kuma": {
			setHealthchecks: true,
			healthchecks:    "https://hi.org/1234",
			setUptimeKuma:   true,
			uptimeKuma:      "/1234?hello=123",
			ok:              false,
			prepareMockPP: func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `The Uptime Kuma URL (redacted) does not look like a valid URL`)
			},
			check: func(t *testing.T, hb heartbeat.Heartbeat, nt notifier.Notifier) {
				t.Helper()
				require.Equal(t, heartbeat.NewComposed(), hb)
				require.Equal(t, notifier.NewComposed(), nt)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			unset(t, "HEALTHCHECKS", "UPTIMEKUMA", "SHOUTRRR")
			if tc.setHealthchecks {
				store(t, "HEALTHCHECKS", tc.healthchecks)
			}
			if tc.setUptimeKuma {
				store(t, "UPTIMEKUMA", tc.uptimeKuma)
			}

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			hb, nt, ok := config.SetupReporters(mockPP)
			require.Equal(t, tc.ok, ok)
			tc.check(t, hb, nt)
		})
	}
}
