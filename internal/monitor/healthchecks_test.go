package monitor_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/monitor"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestNewHealthChecks(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthChecks(mockPP, "https://user:pass@host/path", monitor.SetHealthChecksMaxRetries(100))
	require.True(t, ok)
	h, ok := m.(*monitor.HealthChecks)
	require.True(t, ok)
	require.Equal(t, "https://user:pass@host/path", h.BaseURL)
}

func TestNewHealthChecksFail1(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	gomock.InOrder(
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `The URL %q does not look like a valid Healthchecks URL.`, url.PathEscape("this is not a valid URL")), //nolint: lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `A valid example is "https://hc-ping.com/01234567-0123-0123-0123-0123456789abc".`),                    //nolint: lll
	)
	_, ok := monitor.NewHealthChecks(mockPP, "this is not a valid URL")
	require.False(t, ok)
}

func TestNewHealthChecksFail2(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to parse the Healthchecks URL %q: %v", "://#?", gomock.Any())
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

func TestDescripbeBaseURL(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := monitor.NewHealthChecks(mockPP, "https://user:pass@host/path")
	require.True(t, ok)
	require.Equal(t, "https://user:xxxxx@host/path", m.DescribeBaseURL())
}

//nolint: funlen
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged %q.", gomock.Any()),                            //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()),                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()),                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to ping %q; got response code: %d %s", gomock.Any(), 400, "invalid url format"), //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q in %d time(s).", gomock.Any(), 3), //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged %q.", gomock.Any()),                            //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged %q.", gomock.Any()),                            //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged %q.", gomock.Any()),                            //nolint: lll
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
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Warningf(pp.EmojiError, "Failed to send HTTP(S) request to %q: %v", gomock.Any(), gomock.Any()), //nolint: lll
					m.EXPECT().Infof(pp.EmojiRepeatOnce, "Trying again . . ."),                                                 //nolint: lll
					m.EXPECT().Infof(pp.EmojiNotification, "Successfully pinged %q.", gomock.Any()),                            //nolint: lll
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
				m.EXPECT().Errorf(pp.EmojiImpossible, "Exit code (%i) not within the range 0-255.", -1)
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
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, tc.url, r.URL.EscapedPath())

				visited++
				assert.LessOrEqual(t, visited, len(tc.actions))
				switch tc.actions[visited-1] {
				case ActionOk:
					pinged = true
					_, err := io.WriteString(w, "OK")
					assert.NoError(t, err)
				case ActionNotOk:
					w.WriteHeader(http.StatusBadRequest)
					_, err := io.WriteString(w, "invalid url format")
					assert.NoError(t, err)
				case ActionAbort:
					panic(http.ErrAbortHandler)
				case ActionFail:
					assert.FailNow(t, "failing the test")
				default:
					assert.FailNow(t, "failing the test")
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

func TestEndPointsIllFormed(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiImpossible, "Failed to prepare HTTP(S) request to %q: %v", "blah", gomock.Any())

	m := &monitor.HealthChecks{
		BaseURL:         "://#?",
		RedactedBaseURL: "blah",
		Timeout:         monitor.HealthChecksDefaultTimeout,
		MaxRetries:      monitor.HealthChecksDefaultMaxRetries,
	}

	ok := m.Success(context.Background(), mockPP)
	require.False(t, ok)
}
