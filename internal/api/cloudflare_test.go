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

type httpHandler struct{ requestLimit *int }

func (h httpHandler) setRequestLimit(requestLimit int) { *(h.requestLimit) = requestLimit }
func (h httpHandler) isExhausted() bool                { return *h.requestLimit == 0 }

func checkRequestLimit(t *testing.T, requestLimit *int) bool {
	t.Helper()

	if *requestLimit <= 0 {
		return false
	}
	*requestLimit--

	return true
}

func checkToken(t *testing.T, r *http.Request) bool {
	t.Helper()
	return assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
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
	return fmt.Sprintf(`{"result":{"id":%q,"status":"active"},"success":true,"errors":[],"messages":[{"code":10000,"message":"This API Token is valid and active","type":null}]}`, mockToken) //nolint:lll
}

func newHandle(t *testing.T, ppfmt pp.PP, accountID string, httpStatus int, httpResponse string,
) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	mux, auth := newServerAuth(t, accountID)

	mux.HandleFunc("GET /user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerifyToken(t, w, r, httpStatus, httpResponse)
	})

	h, ok := auth.New(ppfmt, 8760*time.Hour) // a year
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

	ok, certain := h.SanityCheck(context.Background(), mockPP)
	require.True(t, ok)
	require.True(t, certain)

	// Test again to test the caching
	ok, certain = h.SanityCheck(context.Background(), mockPP)
	require.True(t, ok)
	require.True(t, certain)
}

func TestNewEmptyToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, auth := newServerAuth(t, mockAccountID)

	auth.Token = ""
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", gomock.Any())
	h, ok := auth.New(mockPP, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}

func TestSanityCheck(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		okNew                   bool
		prepareMocksNew         func(*mocks.MockPP)
		httpStatus              int
		httpResponse            string
		certainSanityCheck      bool
		okSanityCheck           bool
		prepareMocksSanityCheck func(*mocks.MockPP)
	}{
		"expiring": {
			true,
			nil,
			http.StatusOK,
			`{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"active","expires_on":"3000-01-01T00:00:00Z"}}`, //nolint:lll
			true, true,
			func(p *mocks.MockPP) {
				_, err := time.Parse(time.RFC3339, "3000-01-01T00:00:00Z")
				require.NoError(t, err)
				p.EXPECT().Warningf(pp.EmojiAlarm, "The Cloudflare API token will expire at %s (%v left)",
					gomock.Any(), gomock.Any())
			},
		},
		"expired": {
			true,
			nil,
			http.StatusOK,
			`{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"expired"}}`,
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "expired")
			},
		},
		"funny": {
			true,
			nil,
			http.StatusOK,
			`{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"funny"}}`,
			false, true,
			func(p *mocks.MockPP) {
				p.EXPECT().Warningf(pp.EmojiImpossible,
					"The Cloudflare API token is in an undocumented state %q; please report this at %s",
					"funny", pp.IssueReportingURL)
			},
		},
		"disabled": {
			true,
			nil,
			http.StatusOK,
			`{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"disabled"}}`,
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "disabled")
			},
		},
		"invalid-token": {
			true,
			nil,
			http.StatusUnauthorized,
			`{"success":false,"errors":[{"code":1000,"message":"Invalid API Token"}],"messages":[],"result":null}`,
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid; please check the value of CF_API_TOKEN or CF_API_TOKEN_FILE") //nolint:lll
			},
		},
		"invalid-format": {
			true,
			nil,
			http.StatusBadRequest,
			`{"success":false,"errors":[{"code":6003,"message": "Invalid request headers","error_chain":[{"code":6111,"message": "Invalid format for Authorization header" }]}],"messages":[],"result":null}`, //nolint:lll
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid; please check the value of CF_API_TOKEN or CF_API_TOKEN_FILE") //nolint:lll
			},
		},
		"invalid-json": {
			true,
			nil,
			http.StatusOK,
			`{`,
			false, true,
			nil,
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

			ok, certain := h.SanityCheck(context.Background(), mockPP)
			require.Equal(t, tc.okSanityCheck, ok)
			require.Equal(t, tc.certainSanityCheck, certain)
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

	h, ok := auth.New(mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
	ok, certain := h.SanityCheck(context.Background(), mockPP)
	require.True(t, ok)
	require.False(t, certain)
}
