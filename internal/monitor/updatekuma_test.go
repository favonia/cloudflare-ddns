package monitor_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestUptimeKumaDescripbe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewUptimeKuma(mockPP, "https://user:pass@host/path")
	require.True(t, ok)
	m.Describe(func(service, params string) {
		require.Equal(t, "Uptime Kuma", service)
	})
}

//nolint:funlen
func TestUptimeKumaEndPoints(t *testing.T) {
	t.Parallel()

	type action int
	const (
		ActionOk action = iota
		ActionNotOk
		ActionAbort
		ActionFail
	)

	for name, tc := range map[string]struct {
		endpoint      func(pp.PP, monitor.Monitor) bool
		url           string
		status        string
		msg           string
		ping          string
		actions       []action
		defaultAction action
		pinged        bool
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt, "hello")
			},
			"/", "up", "hello", "",
			[]action{ActionOk},
			ActionAbort, true,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged Uptime Kuma"),
				)
			},
		},
		"success/notok": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt, "aloha")
			},
			"/", "up", "aloha", "",
			[]action{ActionNotOk},
			ActionAbort, false,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to ping Uptime Kuma: %q", "bad"),
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt, "stop now")
			},
			"/", "up", "stop now", "",
			nil, ActionAbort, false,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to Uptime Kuma: %v", gomock.Any()),
				)
			},
		},
		"start": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Start(context.Background(), ppfmt, "starting now!")
			},
			"/", "", "", "",
			[]action{},
			ActionAbort, false,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS")
			},
		},
		"failure": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Failure(context.Background(), ppfmt, "something's wrong")
			},
			"/", "down", "something's wrong", "",
			[]action{ActionOk},
			ActionAbort, true,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged Uptime Kuma"),
				)
			},
		},
		"log": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Log(context.Background(), ppfmt, "message")
			},
			"/", "", "", "",
			[]action{},
			ActionAbort, false,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS")
			},
		},
		"exitstatus/0": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 0, "bye!")
			},
			"/", "", "", "",
			[]action{},
			ActionAbort, false,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS")
			},
		},
		"exitstatus/1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 1, "did exit(1)")
			},
			"/", "down", "did exit(1)", "",
			[]action{ActionOk},
			ActionAbort, true,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged Uptime Kuma"),
				)
			},
		},
		"exitstatus/-1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, -1, "feeling negative")
			},
			"/", "down", "feeling negative", "",
			[]action{ActionOk},
			ActionAbort, true,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged Uptime Kuma"),
				)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			visited := 0
			pinged := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, tc.url, r.URL.EscapedPath())

				q, err := url.ParseQuery(r.URL.RawQuery)
				require.Nil(t, err)
				require.Equal(t, url.Values{
					"status": {tc.status},
					"msg":    {tc.msg},
					"ping":   {tc.ping},
				}, q)

				reqBody, err := io.ReadAll(r.Body)
				require.Nil(t, err)
				require.Empty(t, string(reqBody))

				visited++
				action := tc.defaultAction
				if visited <= len(tc.actions) {
					action = tc.actions[visited-1]
				}
				switch action {
				case ActionOk:
					pinged = true
					_, err := io.WriteString(w, `{"ok":true}`)
					require.NoError(t, err)
				case ActionNotOk:
					_, err := io.WriteString(w, `{"ok":false,"msg":"bad"}`)
					require.NoError(t, err)
				case ActionAbort:
					panic(http.ErrAbortHandler)
				case ActionFail:
					require.FailNow(t, "failing the test")
				default:
					require.FailNow(t, "failing the test")
				}
			}))

			m, ok := monitor.NewUptimeKuma(mockPP, server.URL)
			require.True(t, ok)
			ok = tc.endpoint(mockPP, m)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
