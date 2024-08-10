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

	"github.com/cloudflare/cloudflare-go"
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

func mockResultInfo(totalNum, pageSize int) cloudflare.ResultInfo {
	return cloudflare.ResultInfo{
		Page:       1,
		PerPage:    pageSize,
		TotalPages: (totalNum + pageSize - 1) / pageSize,
		Count:      totalNum,
		Total:      totalNum,
		Cursor:     "",
		Cursors:    cloudflare.ResultInfoCursors{}, //nolint:exhaustruct
	}
}

func mockResponse() cloudflare.Response {
	return cloudflare.Response{
		Success:  true,
		Errors:   []cloudflare.ResponseInfo{},
		Messages: []cloudflare.ResponseInfo{},
	}
}

const (
	mockToken      = "token123"
	mockAuthString = "Bearer " + mockToken
	mockAccountID  = "account456"
)

func TestSupportsRecords(t *testing.T) {
	t.Parallel()

	require.False(t, api.CloudflareAuth{}.SupportsRecords())                //nolint:exhaustruct
	require.True(t, api.CloudflareAuth{Token: mockToken}.SupportsRecords()) //nolint:exhaustruct
}

func TestSupportsWAFLists(t *testing.T) {
	t.Parallel()

	require.False(t, api.CloudflareAuth{}.SupportsWAFLists())                                         //nolint:exhaustruct
	require.False(t, api.CloudflareAuth{Token: mockToken}.SupportsWAFLists())                         //nolint:exhaustruct
	require.True(t, api.CloudflareAuth{Token: mockToken, AccountID: mockAccountID}.SupportsRecords()) //nolint:exhaustruct
}

func newServerAuth(t *testing.T, accountID string) (*http.ServeMux, api.CloudflareAuth) {
	t.Helper()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: accountID,
		BaseURL:   ts.URL,
	}

	return mux, auth
}

type httpHandler[T any] struct {
	mux          *http.ServeMux
	params       *T
	requestLimit *int
}

func (h httpHandler[T]) set(params T, requestLimit int) {
	*(h.params), *(h.requestLimit) = params, requestLimit
}

func (h httpHandler[T]) isExhausted() bool {
	return *h.requestLimit == 0
}

func handleVerifyToken(t *testing.T, w http.ResponseWriter, r *http.Request, responseCode int, response string) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
		!assert.Empty(t, r.URL.Query()) {
		panic(http.ErrAbortHandler)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(responseCode)
	fmt.Fprint(w, response)
}

func mockVerifyToken() string {
	return fmt.Sprintf(`{
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
}`, mockToken)
}

func newHandle(t *testing.T, ppfmt pp.PP, accountID string, httpStatus int, httpResponse string,
) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	mux, auth := newServerAuth(t, accountID)

	mux.HandleFunc("GET /user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerifyToken(t, w, r, httpStatus, httpResponse)
	})

	h, ok := auth.New(context.Background(), ppfmt, 8760*time.Hour) // a year
	return mux, h, ok
}

func newGoodHandle(t *testing.T, ppfmt pp.PP) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	return newHandle(t, ppfmt, mockAccountID, http.StatusOK, mockVerifyToken())
}

func TestNewValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	require.True(t, h.SanityCheck(context.Background(), mockPP))

	// Test again to test the caching
	require.True(t, h.SanityCheck(context.Background(), mockPP))
}

func TestNewEmptyToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, auth := newServerAuth(t, mockAccountID)

	auth.Token = ""
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", gomock.Any())
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}

func TestSanityCheckExpiring(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		httpStatus              int
		httpResponse            string
		okNew                   bool
		prepareMocksNew         func(*mocks.MockPP)
		okSanityCheck           bool
		prepareMocksSanityCheck func(*mocks.MockPP)
	}{
		"expiring": {
			http.StatusOK,
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
			nil,
			true,
			func(p *mocks.MockPP) {
				deadline, err := time.Parse(time.RFC3339, "3000-01-01T00:00:00Z")
				require.NoError(t, err)
				p.EXPECT().Warningf(pp.EmojiAlarm, "The token will expire at %s",
					deadline.In(time.Local).Format(time.RFC1123Z))
			},
		},
		"expired": {
			http.StatusOK,
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "expired"
  }
}`,
			true,
			nil,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "expired"),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
		"funny": {
			http.StatusOK,
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
			nil,
			true,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Warningf(pp.EmojiImpossible, "The Cloudflare API token is in an undocumented state: %s", "funny"),
					p.EXPECT().Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"), //nolint:lll
				)
			},
		},
		"disabled": {
			http.StatusOK,
			`{
  "success": true,
  "errors": [],
  "messages": [],
  "result": {
    "id": "11111111111111111111111111111111",
    "status": "disabled"
  }
}`,
			true,
			nil,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "disabled"),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
		"invalid-token": {
			http.StatusUnauthorized,
			`{
  "success": false,
  "errors": [{ "code": 1000, "message": "Invalid API Token" }],
  "messages": [],
  "result": null
}`,
			true,
			nil,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid: %v", gomock.Any()),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
		"invalid-format": {
			http.StatusUnauthorized,
			`{
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
			true,
			nil,
			false,
			func(p *mocks.MockPP) {
				gomock.InOrder(
					p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid: %v", gomock.Any()),
					p.EXPECT().Errorf(pp.EmojiUserError, "Please double-check the value of CF_API_TOKEN or CF_API_TOKEN_FILE"),
				)
			},
		},
		"invalid-json": {
			http.StatusOK,
			`{`,
			true,
			nil,
			true,
			func(p *mocks.MockPP) {
				p.EXPECT().Warningf(pp.EmojiWarning, "Failed to verify the Cloudflare API token; will retry later: %v", gomock.Any()) //nolint:lll
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)

			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocksNew != nil {
				tc.prepareMocksNew(mockPP)
			}
			_, h, ok := newHandle(t, mockPP, mockAccountID, tc.httpStatus, tc.httpResponse)
			require.Equal(t, tc.okNew, ok)
			require.NotNil(t, h)

			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocksSanityCheck != nil {
				tc.prepareMocksSanityCheck(mockPP)
			}
			require.Equal(t, tc.okSanityCheck, h.SanityCheck(context.Background(), mockPP))
		})
	}
}

func TestSanityCheckTimeout(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, auth := newServerAuth(t, mockAccountID)
	mux.HandleFunc("GET /user/tokens/verify", func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
		assert.Empty(t, r.URL.Query())

		time.Sleep(2 * time.Second)
		panic(http.ErrAbortHandler)
	})

	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
	ok = h.SanityCheck(context.Background(), mockPP)
	require.True(t, ok)
}
