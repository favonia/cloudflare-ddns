// vim: nowrap
package monitor_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestNewHealthchecks(t *testing.T) {
	t.Parallel()

	rawBaseURL := "https://user:pass@host/path"
	parsedBaseURL, err := url.Parse(rawBaseURL)
	require.NoError(t, err)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthchecks(mockPP, rawBaseURL)
	require.Equal(t, monitor.Healthchecks{
		BaseURL: parsedBaseURL,
		Timeout: monitor.HealthchecksDefaultTimeout,
	}, m)
	require.True(t, ok)
}

func TestNewHealthchecksFail1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`),
	)
	_, ok := monitor.NewHealthchecks(mockPP, "this is not a valid URL")
	require.False(t, ok)
}

func TestNewHealthchecksFail2(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`),
	)
	_, ok := monitor.NewHealthchecks(mockPP, "ftp://example.org")
	require.False(t, ok)
}

func TestNewHealthchecksFail3(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
	_, ok := monitor.NewHealthchecks(mockPP, "://#?")
	require.False(t, ok)
}

func TestHealthchecksDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	m, ok := monitor.NewHealthchecks(mockPP, "https://user:pass@host/path")
	require.True(t, ok)

	count := 0
	for name := range m.Describe {
		count++
		require.Equal(t, "Healthchecks", name)
	}
	require.Equal(t, 1, count)
}

func TestHealthchecksEndPoints(t *testing.T) {
	t.Parallel()

	type action int
	const (
		ActionOK action = iota
		ActionNotOK
		ActionAbort
		ActionFail
	)

	for name, tc := range map[string]struct {
		endpoint      func(pp.PP, monitor.Monitor) bool
		url           string
		message       string
		actions       []action
		defaultAction action
		pinged        int
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "hello"))
			},
			"/", "hello",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `default (root)`),
				)
			},
		},
		"success/not-ok": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "aloha"))
			},
			"/", "aloha",
			[]action{ActionAbort, ActionAbort, ActionNotOK},
			ActionAbort, 0,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to ping the %s endpoint of Healthchecks; got response code: %d %s", `default (root)`, 400, "invalid url format"),
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "stop now"))
			},
			"/", "stop now",
			nil, ActionAbort, 0,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks: %v", `default (root)`, gomock.Any()),
				)
			},
		},
		"failure": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(false, "something's wrong"))
			},
			"/fail", "something's wrong",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `"/fail"`),
				)
			},
		},
		"start": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Start(context.Background(), ppfmt, "starting now!")
			},
			"/start", "starting now!",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `"/start"`),
				)
			},
		},
		"exits": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Exit(context.Background(), ppfmt, "bye!")
			},
			"/0", "bye!",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `"/0"`),
				)
			},
		},
		"log": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Log(context.Background(), ppfmt, monitor.NewMessagef(true, "message"))
			},
			"/log", "message",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `"/log"`),
				)
			},
		},
		"log/not-ok": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Log(context.Background(), ppfmt, monitor.NewMessagef(false, "oops!"))
			},
			"/fail", "oops!",
			[]action{ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Pinged the %s endpoint of Healthchecks", `"/fail"`),
				)
			},
		},
		"log/empty": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Log(context.Background(), ppfmt, monitor.NewMessage())
			},
			"/log", "message",
			[]action{},
			ActionAbort, 0,
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			visited := 0
			pinged := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodPost, r.Method) ||
					!assert.Equal(t, tc.url, r.URL.EscapedPath()) {
					panic(http.ErrAbortHandler)
				}

				if reqBody, err := io.ReadAll(r.Body); !assert.NoError(t, err) ||
					!assert.Equal(t, tc.message, string(reqBody)) {
					panic(http.ErrAbortHandler)
				}

				visited++
				action := tc.defaultAction
				if visited <= len(tc.actions) {
					action = tc.actions[visited-1]
				}
				switch action {
				case ActionOK:
					pinged++
					if _, err := io.WriteString(w, "OK"); !assert.NoError(t, err) {
						panic(http.ErrAbortHandler)
					}
				case ActionNotOK:
					w.WriteHeader(http.StatusBadRequest)
					if _, err := io.WriteString(w, "invalid url format"); !assert.NoError(t, err) {
						panic(http.ErrAbortHandler)
					}
				case ActionAbort:
					panic(http.ErrAbortHandler)
				case ActionFail:
					assert.Fail(t, "failing the test")
					panic(http.ErrAbortHandler)
				default:
					assert.Fail(t, "failing the test")
					panic(http.ErrAbortHandler)
				}
			}))

			m, ok := monitor.NewHealthchecks(mockPP, server.URL)
			require.True(t, ok)
			ok = tc.endpoint(mockPP, m)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
