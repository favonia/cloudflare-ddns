package notifier_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/notifier"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestShoutrrrDescripbe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := notifier.NewShoutrrr(mockPP, []string{"generic://localhost/"})
	require.True(t, ok)
	m.Describe(func(service, _params string) {
		require.Equal(t, "generic", service)
	})
}

func TestShoutrrrSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		path          string
		service       func(serverURL string) string
		message       string
		pinged        bool
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			"/greeting",
			func(serverURL string) string { return "generic+" + serverURL + "/greeting" },
			"hello",
			true, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Infof(pp.EmojiNotification, "Sent shoutrrr message")
			},
		},
		"ill-formed url": {
			"",
			func(_serverURL string) string { return "generic+https://0.0.0.0" },
			"hello",
			false, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiError, "Failed to send some shoutrrr message: %v", gomock.Any())
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

			pinged := false
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, tc.path, r.URL.EscapedPath())

				reqBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, tc.message, string(reqBody))

				pinged = true
			}))

			s, ok := notifier.NewShoutrrr(mockPP, []string{tc.service(server.URL)})
			require.True(t, ok)
			ok = s.Send(context.Background(), mockPP, tc.message)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
