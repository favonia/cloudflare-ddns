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
)

func TestShoutrrrDescripbe(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	m, ok := notifier.NewShoutrrr(mockPP, []string{"generic://localhost/"})
	require.True(t, ok)
	m.Describe(func(service, params string) {
		require.Equal(t, "generic", service)
	})
}

func TestShoutrrrSend(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		url           string
		message       string
		pinged        bool
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"success": {
			"/", "hello",
			true, true,
			func(m *mocks.MockPP) {
				gomock.InOrder()
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

			pinged := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, tc.url, r.URL.EscapedPath())

				reqBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, tc.message, string(reqBody))

				pinged = true
			}))

			s, ok := notifier.NewShoutrrr(mockPP, []string{"generic+" + server.URL + tc.url})
			require.True(t, ok)
			ok = s.Send(context.Background(), mockPP, tc.message)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.pinged, pinged)
		})
	}
}
