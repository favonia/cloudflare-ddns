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

const httpUnsafeMsg string = "The Uptime Kuma URL (redacted) uses HTTP; please consider using HTTPS"

func TestNewUptimeKuma(t *testing.T) {
	t.Parallel()

	rawBaseURL := "https://user:pass@host/path"
	parsedBaseURL, err := url.Parse(rawBaseURL)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		input         string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"bare": {"https://user:pass@host/path", true, nil},
		"full": {"https://user:pass@host/path?status=up&msg=OK&ping=", true, nil},
		"unexpected": {
			"https://user:pass@host/path?random=", true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The Uptime Kuma URL (redacted) contains an unexpected query %s=... and it will be ignored", "random")
			},
		},
		"ill-formed-query": {
			"https://user:pass@host/path?status=up;msg=OK;ping=", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The Uptime Kuma URL (redacted) does not look like a valid URL")
			},
		},
		"ftp": {
			"ftp://user:pass@host/", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiUserError, "The Uptime Kuma URL (redacted) does not look like a valid URL")
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
			m, ok := monitor.NewUptimeKuma(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, monitor.UptimeKuma{
					BaseURL: parsedBaseURL,
					Timeout: monitor.UptimeKumaDefaultTimeout,
				}, m)
			} else {
				require.Zero(t, m)
			}
		})
	}
}

func TestUptimeKumaDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	m, ok := monitor.NewUptimeKuma(mockPP, "https://user:pass@host/path")
	require.True(t, ok)

	count := 0
	for name := range m.Describe {
		count++
		require.Equal(t, "Uptime Kuma", name)
	}
	require.Equal(t, 1, count)
}

func TestUptimeKumaEndPoints(t *testing.T) {
	t.Parallel()

	type action int
	const (
		ActionOK action = iota
		ActionNotOK
		ActionGarbage
		ActionAbort
		ActionFail
	)

	successPP := func(m *mocks.MockPP) {
		gomock.InOrder(
			m.EXPECT().Noticef(pp.EmojiUserWarning, httpUnsafeMsg),
			m.EXPECT().Infof(pp.EmojiPing, "Pinged Uptime Kuma"),
		)
	}

	for name, tc := range map[string]struct {
		endpoint      func(pp.PP, monitor.BasicMonitor) bool
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
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "hello"))
			},
			"/", "up", "OK", "",
			[]action{ActionOK},
			ActionAbort, true,
			true,
			successPP,
		},
		"success/not-ok": {
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "aloha"))
			},
			"/", "up", "OK", "",
			[]action{ActionNotOK},
			ActionAbort, false,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, httpUnsafeMsg),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to ping Uptime Kuma: %s", "bad"),
				)
			},
		},
		"success/garbage-response": {
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "aloha"))
			},
			"/", "up", "OK", "",
			[]action{ActionGarbage},
			ActionAbort, false,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, httpUnsafeMsg),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to parse the response from Uptime Kuma: %v", gomock.Any()),
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(true, "stop now"))
			},
			"/", "up", "OK", "",
			nil, ActionAbort, false,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, httpUnsafeMsg),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to send HTTP(S) request to Uptime Kuma: %v", gomock.Any()),
				)
			},
		},
		"failure": {
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(false, "something's wrong"))
			},
			"/", "down", "something's wrong", "",
			[]action{ActionOK},
			ActionAbort, true,
			true,
			successPP,
		},
		"failure/empty": {
			func(ppfmt pp.PP, m monitor.BasicMonitor) bool {
				return m.Ping(context.Background(), ppfmt, monitor.NewMessagef(false, ""))
			},
			"/", "down", "Failing", "",
			[]action{ActionOK},
			ActionAbort, true,
			true,
			successPP,
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
			pinged := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodGet, r.Method) ||
					!assert.Equal(t, tc.url, r.URL.EscapedPath()) {
					panic(http.ErrAbortHandler)
				}

				q, err := url.ParseQuery(r.URL.RawQuery)
				if !assert.NoError(t, err) ||
					!assert.Equal(t, url.Values{
						"status": {tc.status},
						"msg":    {tc.msg},
						"ping":   {tc.ping},
					}, q) {
					panic(http.ErrAbortHandler)
				}

				if reqBody, err := io.ReadAll(r.Body); !assert.NoError(t, err) ||
					!assert.Empty(t, string(reqBody)) {
					panic(http.ErrAbortHandler)
				}

				visited++
				action := tc.defaultAction
				if visited <= len(tc.actions) {
					action = tc.actions[visited-1]
				}
				switch action {
				case ActionOK:
					pinged = true
					if _, err := io.WriteString(w, `{"ok":true}`); !assert.NoError(t, err) {
						panic(http.ErrAbortHandler)
					}
				case ActionNotOK:
					if _, err := io.WriteString(w, `{"ok":false,"msg":"bad"}`); !assert.NoError(t, err) {
						panic(http.ErrAbortHandler)
					}
				case ActionGarbage:
					if _, err := io.WriteString(w, `This is [ { not a valid JSON`); !assert.NoError(t, err) {
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

			m, ok := monitor.NewUptimeKuma(mockPP, server.URL)
			require.True(t, ok)
			ok = tc.endpoint(mockPP, m)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
