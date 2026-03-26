// vim: nowrap

package heartbeat_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestNewHealthchecks(t *testing.T) {
	t.Parallel()

	rawBaseURL := "https://user:pass@host/path"
	parsedBaseURL, err := url.Parse(rawBaseURL)
	require.NoError(t, err)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := heartbeat.NewHealthchecks(mockPP, rawBaseURL)
	require.Equal(t, heartbeat.Healthchecks{
		BaseURL: parsedBaseURL,
		Timeout: heartbeat.HealthchecksDefaultTimeout,
	}, m)
	require.True(t, ok)
}

func TestNewHealthchecksFail1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) is not a valid URL`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `Expected a URL like "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`),
	)
	_, ok := heartbeat.NewHealthchecks(mockPP, "this is not a valid URL")
	require.False(t, ok)
}

func TestNewHealthchecksFail2(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `The Healthchecks URL (redacted) is not a valid URL`),
		mockPP.EXPECT().Noticef(pp.EmojiUserError, `Expected a URL like "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`),
	)
	_, ok := heartbeat.NewHealthchecks(mockPP, "ftp://example.org")
	require.False(t, ok)
}

func TestNewHealthchecksFail3(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
	_, ok := heartbeat.NewHealthchecks(mockPP, "://#?")
	require.False(t, ok)
}

func TestHealthchecksDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	m, ok := heartbeat.NewHealthchecks(mockPP, "https://user:pass@host/path")
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
		endpoint      func(pp.PP, heartbeat.Heartbeat) bool
		url           string
		message       string
		actions       []action
		defaultAction action
		pinged        int
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Ping(context.Background(), ppfmt, heartbeat.NewMessagef(true, "hello"))
			},
			"/", "hello",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "a ping"),
				)
			},
		},
		"success/not-ok": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Ping(context.Background(), ppfmt, heartbeat.NewMessagef(true, "aloha"))
			},
			"/", "aloha",
			[]action{ActionAbort, ActionAbort, ActionNotOK},
			ActionAbort, 0,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Noticef(pp.EmojiError,
						"The %s to Healthchecks returned an unexpected response (%s): got %d %s",
						"ping", "base URL", http.StatusBadRequest, "invalid url format"),
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Ping(context.Background(), ppfmt, heartbeat.NewMessagef(true, "stop now"))
			},
			"/", "stop now",
			nil, ActionAbort, 0,
			false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to send %s to Healthchecks (%s): %v", "a ping", "base URL", gomock.Any()),
				)
			},
		},
		"failure": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Ping(context.Background(), ppfmt, heartbeat.NewMessagef(false, "something's wrong"))
			},
			"/fail", "something's wrong",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "a failure ping"),
				)
			},
		},
		"start": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Start(context.Background(), ppfmt, "starting now!")
			},
			"/start", "starting now!",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "a start ping"),
				)
			},
		},
		"exits": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Exit(context.Background(), ppfmt, "bye!")
			},
			"/0", "bye!",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "an exit ping"),
				)
			},
		},
		"log": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Log(context.Background(), ppfmt, heartbeat.NewMessagef(true, "message"))
			},
			"/log", "message",
			[]action{ActionAbort, ActionAbort, ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "a log ping"),
				)
			},
		},
		"log/not-ok": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Log(context.Background(), ppfmt, heartbeat.NewMessagef(false, "oops!"))
			},
			"/fail", "oops!",
			[]action{ActionOK},
			ActionAbort, 1,
			true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiPing, "Successfully sent %s to Healthchecks", "a failure ping"),
				)
			},
		},
		"log/empty": {
			func(ppfmt pp.PP, m heartbeat.Heartbeat) bool {
				return m.Log(context.Background(), ppfmt, heartbeat.NewMessage())
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

			m, ok := heartbeat.NewHealthchecks(mockPP, server.URL)
			require.True(t, ok)
			ok = tc.endpoint(mockPP, m)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}

func TestHealthchecksPingRequestCreationFailure(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiImpossible, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
		func(gotEmoji pp.Emoji, format string, args ...any) {
			assert.Equal(t, pp.EmojiImpossible, gotEmoji)
			assert.Contains(t, fmt.Sprintf(format, args...), "Failed to create the request for a ping to Healthchecks (base URL):")
		},
	)

	// A space in the host makes net/http reject the URL during request creation before any network I/O happens.
	ok := (heartbeat.Healthchecks{
		BaseURL: &url.URL{Scheme: "http", Host: "bad host", Path: "/"}, //nolint:exhaustruct // Unused URL fields are irrelevant to this request-construction failure fixture.
		Timeout: heartbeat.HealthchecksDefaultTimeout,
	}).Ping(context.Background(), mockPP, heartbeat.NewMessagef(true, "hello"))
	require.False(t, ok)
}

func TestHealthchecksPingResponseReadFailure(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
		mockPP.EXPECT().Noticef(pp.EmojiError, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(
			func(gotEmoji pp.Emoji, format string, args ...any) {
				assert.Equal(t, pp.EmojiError, gotEmoji)
				assert.Contains(t, fmt.Sprintf(format, args...), "Failed to read the response from Healthchecks for a ping (base URL):")
			},
		),
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/", r.URL.EscapedPath())

		reqBody, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}
		assert.Equal(t, "hello", string(reqBody))

		hijacker, ok := w.(http.Hijacker)
		if !assert.True(t, ok) {
			panic(http.ErrAbortHandler)
		}

		conn, bufrw, err := hijacker.Hijack()
		if !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}
		defer conn.Close()

		_, err = bufrw.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nO")
		if !assert.NoError(t, err) {
			panic(http.ErrAbortHandler)
		}
		if !assert.NoError(t, bufrw.Flush()) {
			panic(http.ErrAbortHandler)
		}
	}))
	defer server.Close()

	h, ok := heartbeat.NewHealthchecks(mockPP, server.URL)
	require.True(t, ok)
	require.False(t, h.Ping(context.Background(), mockPP, heartbeat.NewMessagef(true, "hello")))
}
