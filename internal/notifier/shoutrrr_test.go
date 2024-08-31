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
	m, ok := notifier.NewShoutrrr(mockPP, []string{
		"generic://localhost/",
		"gotify://host:80/path/tokentoken",
		"ifttt://hey/?events=1",
	})
	require.True(t, ok)

	count := 0
outer:
	for name := range m.Describe {
		count++
		switch count {
		case 1:
			require.Equal(t, "Generic", name)
		case 2:
			require.Equal(t, "Gotify", name)
			break outer
		default:
		}
	}
	require.Equal(t, 2, count)
}

func TestShoutrrrSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path          string
		pinged        int
		service       func(serverURL string) string
		message       notifier.Message
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			"/greeting", 1,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			notifier.NewMessagef("hello"),
			true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiNotify, "Notified %s via shoutrrr", "Generic")
			},
		},
		"ill-formed url": {
			"", 0,
			func(_serverURL string) string { return "generic+https://0.0.0.0" },
			notifier.NewMessagef("hello"),
			false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to notify shoutrrr service(s): %v", gomock.Any())
			},
		},
		"empty": {
			"/greeting", 0,
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			notifier.NewMessage(),
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
