package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

//nolint:paralleltest // paralleltest should not be used because environment vars are global
func TestReadAndAppendHealthchecksURL(t *testing.T) {
	key := keyPrefix + "HEALTHCHECKS"

	type mon = monitor.Monitor

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      mon
		newField      mon
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", nil, nil, true, nil,
		},
		"empty": {
			true, "", nil, nil, true, nil,
		},
		"example": {
			true, "https://hi.org/1234",
			nil,
			monitor.NewComposed(monitor.Healthchecks{
				BaseURL: urlMustParse(t, "https://hi.org/1234"),
				Timeout: monitor.HealthchecksDefaultTimeout,
			}),
			true,
			nil,
		},
		"password": {
			true, "https://me:pass@hi.org/1234",
			nil,
			monitor.NewComposed(monitor.Healthchecks{
				BaseURL: urlMustParse(t, "https://me:pass@hi.org/1234"),
				Timeout: monitor.HealthchecksDefaultTimeout,
			}),
			true,
			nil,
		},
		"fragment": {
			true, "https://hi.org/1234#fragment",
			nil,
			monitor.NewComposed(monitor.Healthchecks{
				BaseURL: urlMustParse(t, "https://hi.org/1234#fragment"),
				Timeout: monitor.HealthchecksDefaultTimeout,
			}),
			true,
			nil,
		},
		"query": {
			true, "https://hi.org/1234?hello=123",
			nil,
			nil,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
					m.EXPECT().Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`), //nolint:lll
				)
			},
		},
		"illformed/not-url": {
			true, "\001",
			nil,
			nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `Failed to parse the Healthchecks URL (redacted)`)
			},
		},
		"illformed/not-abs": {
			true, "/1234?hello=123",
			nil,
			nil, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
					m.EXPECT().Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`), //nolint:lll
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAndAppendHealthchecksURL(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}

//nolint:paralleltest // paralleltest should not be used because environment vars are global
func TestReadAndAppendUptimeKumaURL(t *testing.T) {
	key := keyPrefix + "UPTIMEKUMA"

	type mon = monitor.Monitor

	for name, tc := range map[string]struct {
		set           bool
		val           string
		oldField      mon
		newField      mon
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"unset": {
			false, "", nil, nil, true, nil,
		},
		"empty": {
			true, "", nil, nil, true, nil,
		},
		"example": {
			true, "https://hi.org/1234",
			nil,
			monitor.NewComposed(monitor.UptimeKuma{
				BaseURL: urlMustParse(t, "https://hi.org/1234"),
				Timeout: monitor.UptimeKumaDefaultTimeout,
			}),
			true,
			nil,
		},
		"password": {
			true, "https://me:pass@hi.org/1234",
			nil,
			monitor.NewComposed(monitor.UptimeKuma{
				BaseURL: urlMustParse(t, "https://me:pass@hi.org/1234"),
				Timeout: monitor.UptimeKumaDefaultTimeout,
			}),
			true,
			nil,
		},
		"fragment": {
			true, "https://hi.org/1234#fragment",
			nil,
			monitor.NewComposed(monitor.UptimeKuma{
				BaseURL: urlMustParse(t, "https://hi.org/1234#fragment"),
				Timeout: monitor.UptimeKumaDefaultTimeout,
			}),
			true,
			nil,
		},
		"query": {
			true, "https://hi.org/1234?hello=123",
			nil,
			monitor.NewComposed(monitor.UptimeKuma{
				BaseURL: urlMustParse(t, "https://hi.org/1234"),
				Timeout: monitor.UptimeKumaDefaultTimeout,
			}),
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError,
					`The Uptime Kuma URL (redacted) contains an unexpected query %s=... and it will be ignored`,
					"hello")
			},
		},
		"illformed/not-url": {
			true, "\001",
			nil,
			nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `Failed to parse the Uptime Kuma URL (redacted)`)
			},
		},
		"illformed/not-abs": {
			true, "/1234?hello=123",
			nil,
			nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, `The Uptime Kuma URL (redacted) does not look like a valid URL`)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			set(t, key, tc.set, tc.val)
			field := tc.oldField
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			ok := config.ReadAndAppendUptimeKumaURL(mockPP, key, &field)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.newField, field)
		})
	}
}
