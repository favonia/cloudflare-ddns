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
	mockToken               = "token123"
	mockAuthString          = "Bearer " + mockToken
	mockAccountID           = "account456"
	mockTokenVerifyResponse = `{"result":{"id":"token123","status":"active"},"success":true,"errors":[],"messages":[{"code":10000,"message":"This API Token is valid and active","type":null}]}` //nolint:gosec,lll // no real credentials here
	mockAccountResponse     = `{"result":{"id":"account567","name":"account-name"},"success":true,"errors":[],"messages":[]}`                                                                    //nolint:lll
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

func newHandle(t *testing.T, ppfmt pp.PP, accountID string,
	tokenStatus int, tokenResponse string,
	accountStatus int, accountResponse string,
) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	mux, auth := newServerAuth(t, accountID)

	mux.HandleFunc("GET /user/tokens/verify", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(tokenStatus)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, tokenResponse)
	})

	mux.HandleFunc("GET /accounts/"+accountID, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(accountStatus)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, accountResponse)
	})

	h, ok := auth.New(ppfmt, 8760*time.Hour) // a year
	return mux, h, ok
}

func newGoodHandle(t *testing.T, ppfmt pp.PP) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	return newHandle(t, ppfmt, mockAccountID,
		http.StatusOK, mockTokenVerifyResponse,
		http.StatusOK, mockAccountResponse,
	)
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
		accountID               string
		tokenStatus             int
		tokenResponse           string
		accountStatus           int
		accountResponse         string
		certainSanityCheck      bool
		okSanityCheck           bool
		prepareMocksSanityCheck func(*mocks.MockPP)
	}{
		"token-expiring": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, `{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"active","expires_on":"3000-01-01T00:00:00Z"}}`, //nolint:lll
			http.StatusOK, mockAccountResponse,
			true, true,
			func(p *mocks.MockPP) {
				_, err := time.Parse(time.RFC3339, "3000-01-01T00:00:00Z")
				require.NoError(t, err)
				p.EXPECT().Warningf(pp.EmojiAlarm, "The Cloudflare API token will expire at %s (%v left)",
					gomock.Any(), gomock.Any())
			},
		},
		"token-expired": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, `{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"expired"}}`, //nolint:lll
			http.StatusOK, "",
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "expired")
			},
		},
		"token-funny": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, `{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"funny"}}`, //nolint:lll
			http.StatusOK, mockAccountResponse,
			true, true,
			func(p *mocks.MockPP) {
				p.EXPECT().Warningf(pp.EmojiImpossible,
					"The Cloudflare API token is in an undocumented state %q; please report this at %s",
					"funny", pp.IssueReportingURL)
			},
		},
		"token-disabled": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, `{"success":true,"errors":[],"messages":[],"result":{"id":"11111111111111111111111111111111","status":"disabled"}}`, //nolint:lll
			http.StatusOK, "",
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is %s", "disabled")
			},
		},
		"token-invalid": {
			true,
			nil,
			mockAccountID,
			http.StatusUnauthorized, `{"success":false,"errors":[{"code":1000,"message":"Invalid API Token"}],"messages":[],"result":null}`, //nolint:lll
			http.StatusOK, "",
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid; please check the value of CF_API_TOKEN or CF_API_TOKEN_FILE") //nolint:lll
			},
		},
		"token-illformed": {
			true,
			nil,
			mockAccountID,
			http.StatusBadRequest, `{"success":false,"errors":[{"code":6003,"message": "Invalid request headers","error_chain":[{"code":6111,"message": "Invalid format for Authorization header" }]}],"messages":[],"result":null}`, //nolint:lll
			http.StatusOK, "",
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare API token is invalid; please check the value of CF_API_TOKEN or CF_API_TOKEN_FILE") //nolint:lll
			},
		},
		"token-jsone-invalid": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, `{`,
			http.StatusOK, "",
			false, true,
			nil,
		},
		"account-empty": {
			true,
			nil,
			"",
			http.StatusOK, mockTokenVerifyResponse,
			http.StatusOK, "",
			true, true,
			nil,
		},
		"account-ambiguous": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, mockTokenVerifyResponse,
			http.StatusForbidden, `{"success":false,"errors":[{"code":9109,"message":"Unauthorized to access requested resource"}],"messages":[],"result":null}`, //nolint:lll
			false, true,
			nil,
		},
		"account-illformed/1": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, mockTokenVerifyResponse,
			http.StatusBadRequest, `{"result":null,"success":false,"errors":[{"code":7003,"message":"Could not route to the black hole, perhaps your object identifier is invalid?"}],"messages":[]}`, //nolint:lll
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare account ID is invalid; please check the value of CF_ACCOUNT_ID") //nolint:lll
			},
		},
		"account-illformed/2": {
			true,
			nil,
			mockAccountID,
			http.StatusOK, mockTokenVerifyResponse,
			http.StatusNotFound, `{"result":null,"success":false,"errors":[{"code":7003,"message":"Could not route to the white hole, perhaps your object identifier is invalid?"}],"messages":[]}`, //nolint:lll
			true, false,
			func(p *mocks.MockPP) {
				p.EXPECT().Errorf(pp.EmojiUserError, "The Cloudflare account ID is invalid; please check the value of CF_ACCOUNT_ID") //nolint:lll
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
			_, h, ok := newHandle(t, mockPP, tc.accountID,
				tc.tokenStatus, tc.tokenResponse,
				tc.accountStatus, tc.accountResponse,
			)
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

func TestSanityCheckTokenTimeout(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, auth := newServerAuth(t, mockAccountID)
	mux.HandleFunc("GET /user/tokens/verify", func(http.ResponseWriter, *http.Request) {
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

func TestSanityCheckAccountTimeout(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, auth := newServerAuth(t, mockAccountID)
	mux.HandleFunc("GET /user/tokens/verify", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTokenVerifyResponse)
	})
	mux.HandleFunc("GET /accounts/"+mockAccountID, func(http.ResponseWriter, *http.Request) {
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
