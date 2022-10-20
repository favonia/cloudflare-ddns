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

func TestSetHealthChecksMaxRetries(t *testing.T) {
	t.Parallel()

	m := &monitor.HealthChecks{} //nolint:exhaustruct

	monitor.SetHealthChecksMaxRetries(42)(m)

	require.Equal(t, &monitor.HealthChecks{MaxRetries: 42}, m) //nolint:exhaustruct
}

func TestSetHealthChecksMaxRetriesPanic(t *testing.T) {
	t.Parallel()

	require.Panics(t,
		func() {
			monitor.SetHealthChecksMaxRetries(0)
		},
	)
	require.Panics(t,
		func() {
			monitor.SetHealthChecksMaxRetries(-1)
		},
	)
}

func TestNewHealthChecks(t *testing.T) {
	t.Parallel()

	rawURL := "https://user:pass@host/path"
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthChecks(mockPP, rawURL, monitor.SetHealthChecksMaxRetries(100))
	require.Equal(t, &monitor.HealthChecks{
		BaseURL:    parsedURL,
		Timeout:    monitor.HealthChecksDefaultTimeout,
		MaxRetries: 100,
	}, m)
	require.True(t, ok)
}

func TestNewHealthChecksFail1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `The Healthchecks.io URL (redacted) does not look like a valid URL.`),
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`), //nolint:lll
	)
	_, ok := monitor.NewHealthChecks(mockPP, "this is not a valid URL")
	require.False(t, ok)
}

func TestNewHealthChecksFail2(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks.io URL (redacted)")
	_, ok := monitor.NewHealthChecks(mockPP, "://#?")
	require.False(t, ok)
}

func TestDescripbeService(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthChecks(mockPP, "https://user:pass@host/path")
	require.True(t, ok)
	require.Equal(t, "Healthchecks.io", m.DescribeService())
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
		actions       []action
		pinged        bool
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt)
			},
			"/",
			[]action{ActionAbort, ActionAbort, ActionOk},
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                     //nolint:lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io", `default (root)`),                             //nolint:lll
				)
			},
		},
		"success/notok": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt)
			},
			"/",
			[]action{ActionAbort, ActionAbort, ActionNotOk},
			false, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()),                 //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()),                 //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to ping the %s endpoint of Healthchecks.io; got response code: %d %s", `default (root)`, 400, "invalid url format"), //nolint:lll
				)
			},
		},
		"success/abort/all": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Success(context.Background(), ppfmt)
			},
			"/",
			[]action{ActionAbort, ActionAbort, ActionAbort},
			false, false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `default (root)`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                                     //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io in %d time(s)", `default (root)`, 3),  //nolint:lll
				)
			},
		},
		"start": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Start(context.Background(), ppfmt)
			},
			"/start",
			[]action{ActionAbort, ActionAbort, ActionOk},
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/start"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                               //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/start"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                               //nolint:lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io", `"/start"`),                             //nolint:lll
				)
			},
		},
		"failure": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.Failure(context.Background(), ppfmt)
			},
			"/fail",
			[]action{ActionAbort, ActionAbort, ActionOk},
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/fail"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                              //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/fail"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                              //nolint:lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io", `"/fail"`),                             //nolint:lll
				)
			},
		},
		"exitstatus/0": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 0)
			},
			"/0",
			[]action{ActionAbort, ActionAbort, ActionOk},
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/0"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                           //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/0"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                           //nolint:lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io", `"/0"`),                             //nolint:lll
				)
			},
		},
		"exitstatus/1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, 1)
			},
			"/1",
			[]action{ActionAbort, ActionAbort, ActionOk},
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/1"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                           //nolint:lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to the %s endpoint of Healthchecks.io: %v", `"/1"`, gomock.Any()), //nolint:lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                           //nolint:lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged the %s endpoint of Healthchecks.io", `"/1"`),                             //nolint:lll
				)
			},
		},
		"exitstatus/-1": {
			func(ppfmt pp.PP, m monitor.Monitor) bool {
				return m.ExitStatus(context.Background(), ppfmt, -1)
			},
			"",
			nil,
			false, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiImpossible, "Exit code (%i) not within the range 0-255", -1)
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

				visited++
				require.LessOrEqual(t, visited, len(tc.actions))
				switch tc.actions[visited-1] {
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

			m, ok := monitor.NewHealthChecks(mockPP, server.URL, monitor.SetHealthChecksMaxRetries(3))
			require.True(t, ok)
			ok = tc.endpoint(mockPP, m)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
