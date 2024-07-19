package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func mustIP(ip string) netip.Addr {
	return netip.MustParseAddr(ip)
}

// mockID returns a hex string of length 32, suitable for all kinds of IDs
// used in the Cloudflare API.
func mockID(seed string, suffix int) string {
	seed = fmt.Sprintf("%s/%d", seed, suffix)
	arr := sha512.Sum512([]byte(seed))
	return hex.EncodeToString(arr[:16])
}

func mockIDs(seed string, suffixes ...int) []string {
	ids := make([]string, len(suffixes))
	for i, suffix := range suffixes {
		ids[i] = mockID(seed, suffix)
	}
	return ids
}

const (
	mockToken      = "token123"
	mockAuthString = "Bearer " + mockToken
	mockAccount    = "account456"
)

func newServerAuth(t *testing.T, emptyAccountID bool) (*http.ServeMux, api.CloudflareAuth) {
	t.Helper()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	if emptyAccountID {
		auth.AccountID = ""
	}

	return mux, auth
}

func handleSanityCheck(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	require.Equal(t, http.MethodGet, r.Method)
	require.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
	require.Empty(t, r.URL.Query())

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w,
		`{
			"result": { "id": "%s", "status": "active" },
			"success": true,
			"errors": [],
			"messages": [
				{
					"code": 10000,
					"message": "This API Token is valid and active",
					"type": null
				}
			]
		}`,
		mockID("result", 0))
}

func newHandle(t *testing.T, emptyAccountID bool, mockPP *mocks.MockPP) (*http.ServeMux, api.Handle) {
	t.Helper()

	mux, auth := newServerAuth(t, emptyAccountID)

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleSanityCheck(t, w, r)
	})

	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)

	return mux, h
}

func TestNewValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h := newHandle(t, false, mockPP)

	require.True(t, h.SanityCheck(context.Background(), mockPP))

	// Test again to test the caching
	require.True(t, h.SanityCheck(context.Background(), mockPP))
}

func TestNewEmpty(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, auth := newServerAuth(t, false)

	auth.Token = ""
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", gomock.Any())
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}

func TestSanityCheckExpiring(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		resp          string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"expiring": {
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "active",
    "expires_on": "3000-01-01T00:00:00Z"
  }
}`,
			true,
			func(p *mocks.MockPP) {
				deadline, err := time.Parse(time.RFC3339, "3000-01-01T00:00:00Z")
				require.NoError(t, err)
				p.EXPECT().Warningf(pp.EmojiAlarm, "The token will expire at %s",
					deadline.In(time.Local).Format(time.RFC1123Z))
			},
		},
		"expired": {
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "expired"
  }
}`,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "expired"),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
		"funny": {
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "funny"
  }
}`,
			true,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Warningf(pp.EmojiImpossible, "The Cloudflare API token is in an undocumented state: %s", "funny"),
					p.EXPECT().Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"), //nolint:lll
				)
			},
		},
		"disabled": {
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "disabled"
  }
}`,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "disabled"),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mux, auth := newServerAuth(t, false)
			mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodGet, r.Method) ||
					!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
					!assert.Empty(t, r.URL.Query()) {
					panic(http.ErrAbortHandler)
				}

				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.resp)
			})

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			h, ok := auth.New(context.Background(), mockPP, time.Second)
			require.True(t, ok)
			require.NotNil(t, h)
			require.Equal(t, tc.ok, h.SanityCheck(context.Background(), mockPP))
		})
	}
}

func TestNewInvalid(t *testing.T) {
	t.Parallel()

	for name, resp := range map[string]string{
		"invalid-token": `{
  "success": false,
  "errors": [{ "code": 1000, "message": "Invalid API Token" }],
  "messages": [],
  "result": null
}`,
		"invalid-format": `{
  "success": false,
  "errors": [
    {
      "code": 6003,
      "message": "Invalid request headers",
      "error_chain": [
        { "code": 6111, "message": "Invalid format for Authorization header" }
      ]
    }
  ],
  "messages": [],
  "result": null
}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mux, auth := newServerAuth(t, false)
			mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
				if !assert.Equal(t, http.MethodGet, r.Method) ||
					!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
					!assert.Empty(t, r.URL.Query()) {
					panic(http.ErrAbortHandler)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, resp)
			})

			mockPP := mocks.NewMockPP(mockCtrl)
			gomock.InOrder(
				mockPP.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid: %v", gomock.Any()),
				mockPP.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
			)
			h, ok := auth.New(context.Background(), mockPP, time.Second)
			require.True(t, ok)
			require.NotNil(t, h)
			require.False(t, h.SanityCheck(context.Background(), mockPP))
		})
	}
}

func TestSanityCheckInvalidJSON(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, auth := newServerAuth(t, false)
	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
		assert.Empty(t, r.URL.Query())

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{")
	})

	mockPP.EXPECT().Warningf(pp.EmojiWarning, "Failed to verify the Cloudflare API token; will retry later: %v", gomock.Any()) //nolint:lll
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
	require.True(t, h.SanityCheck(context.Background(), mockPP))
}

func TestSanityCheckTimeout(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, auth := newServerAuth(t, false)
	mux.HandleFunc("/user/tokens/verify", func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
		assert.Empty(t, r.URL.Query())

		time.Sleep(2 * time.Second)
		panic(http.ErrAbortHandler)
	})

	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
	require.True(t, h.SanityCheck(context.Background(), mockPP))
}
