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

func TestNewHealthchecks(t *testing.T) {
	t.Parallel()

	rawURL := "https://user:pass@host/path"
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthchecks(mockPP, rawURL)
	require.Equal(t, &monitor.Healthchecks{
		BaseURL: parsedURL,
		Timeout: monitor.HealthchecksDefaultTimeout,
	}, m)
	require.True(t, ok)
}

func TestNewHealthchecksFail1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`), //nolint:lll
	)
	_, ok := monitor.NewHealthchecks(mockPP, "this is not a valid URL")
	require.False(t, ok)
}

func TestNewHealthchecksFail2(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `The Healthchecks URL (redacted) does not look like a valid URL`),
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc"`), //nolint:lll
	)
	_, ok := monitor.NewHealthchecks(mockPP, "ftp://example.org")
	require.False(t, ok)
}

func TestNewHealthchecksFail3(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks URL (redacted)")
	_, ok := monitor.NewHealthchecks(mockPP, "://#?")
	require.False(t, ok)
}

func TestDescripbe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthchecks(mockPP, "https://user:pass@host/path")
	require.True(t, ok)
	m.Describe(func(service, params string) {
		require.Equal(t, "Healthchecks", service)
	})
}

//nolint:funlen
func TestEndPoints(t *testing.T) {
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
		message       string
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
			"/", "hello",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `default (root)`), //nolint:lll
				)
			},
		},
		"success/notok": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt, "aloha")
			},
			"/", "aloha",
			[]action{ActionAbort, ActionAbort, ActionNotOk},
			ActionAbort,
			false, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to ping the %s endpoint of Healthchecks; got response code: %d %s", `default (root)`, 400, "invalid url format"), //nolint:lll
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt, "stop now")
			},
			"/", "stop now",
			nil,
			ActionAbort,
			false, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks: %v", `default (root)`, gomock.Any()), //nolint:lll
				)
			},
		},
		"start": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Start(context.Background(), ppfmt, "starting now!")
			},
			"/start", "starting now!",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `"/start"`), //nolint:lll
				)
			},
		},
		"failure": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Failure(context.Background(), ppfmt, "something's wrong")
			},
			"/fail", "something's wrong",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `"/fail"`), //nolint:lll
				)
			},
		},
		"log": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Log(context.Background(), ppfmt, "message")
			},
			"/log", "message",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `"/log"`), //nolint:lll
				)
			},
		},
		"exitstatus/0": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 0, "bye!")
			},
			"/0", "bye!",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `"/0"`),
				)
			},
		},
		"exitstatus/1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 1, "did exit(1)")
			},
			"/1", "did exit(1)",
			[]action{ActionAbort, ActionAbort, ActionOk},
			ActionAbort,
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks", `"/1"`),
				)
			},
		},
		"exitstatus/-1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, -1, "feeling negative")
			},
			"", "feeling negative",
			nil, ActionAbort,
			false, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserWarning, "The Healthchecks URL (redacted) uses HTTP; please consider using HTTPS"),
					m.EXPECT().Errorf(pp.EmojiImpossible, "Exit code (%d) not within the range 0-255", -1),
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
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, tc.url, r.URL.EscapedPath())

				reqBody, err := io.ReadAll(r.Body)
				require.Nil(t, err)
				require.Equal(t, tc.message, string(reqBody))

				visited++
				action := tc.defaultAction
				if visited <= len(tc.actions) {
					action = tc.actions[visited-1]
				}
				switch action {
				case ActionOk:
					pinged = true
					_, err := io.WriteString(w, "OK")
					require.NoError(t, err)
				case ActionNotOk:
					w.WriteHeader(http.StatusBadRequest)
					_, err := io.WriteString(w, "invalid url format")
					require.NoError(t, err)
				case ActionAbort:
					panic(http.ErrAbortHandler)
				case ActionFail:
					require.FailNow(t, "failing the test")
				default:
					require.FailNow(t, "failing the test")
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
