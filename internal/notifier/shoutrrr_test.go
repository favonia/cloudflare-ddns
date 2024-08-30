package notifier_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/message"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestDescribeShoutrrrService(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input        string
		output       string
		prepareMocks func(*mocks.MockPP)
	}{
		"ifttt": {"ifttt", "IFTTT", nil},
		"zulip": {"zulip", "Zulip Chat", nil},
		"empty": {
			"", "",
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible,
					"Unknown shoutrrr service name %q; please report it at %s",
					"", pp.IssueReportingURL)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			output := notifier.DescribeShoutrrrService(mockPP, tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestShoutrrrDescribe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := notifier.NewShoutrrr(mockPP, []string{"generic://localhost/"})
	require.True(t, ok)
	for name := range m.Describe {
		require.Equal(t, "Generic", name)
	}
}

func TestShoutrrrSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path          string
		pinged        int
		service       func(serverURL string) string
		message       message.NotifierMessage
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			"/greeting", 1,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			message.NewNotifierMessagef("hello"),
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiNotify, "Notified %s via shoutrrr", "Generic")
			},
		},
		"ill-formed url": {
			"", 0,
			func(_serverURL string) string { return "generic+https://0.0.0.0" },
			message.NewNotifierMessagef("hello"),
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to notify shoutrrr service(s): %v", gomock.Any())
			},
		},
		"empty": {
			"/greeting", 0,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			message.NewNotifierMessage(),
			true,
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			pinged := 0
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodPost, r.Method) ||
					!assert.Equal(t, tc.path, r.URL.EscapedPath()) {
					panic(http.ErrAbortHandler)
				}

				if reqBody, err := io.ReadAll(r.Body); !assert.NoError(t, err) ||
					!assert.Equal(t, tc.message.Format(), string(reqBody)) {
					panic(http.ErrAbortHandler)
				}

				pinged++
			}))

			s, ok := notifier.NewShoutrrr(mockPP, []string{tc.service(server.URL)})
			require.True(t, ok)
			ok = s.Send(context.Background(), mockPP, tc.message)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
