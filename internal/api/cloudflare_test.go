package api_test

import (
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
func mockID(seed string, suffix int) api.ID {
	seed = fmt.Sprintf("%s/%d", seed, suffix)
	arr := sha512.Sum512([]byte(seed))
	return api.ID(hex.EncodeToString(arr[:16]))
}

func mockIDs(seed string, suffixes ...int) []api.ID {
	ids := make([]api.ID, len(suffixes))
	for i, suffix := range suffixes {
		ids[i] = mockID(seed, suffix)
	}
	return ids
}

func mockIDsAsStrings(seed string, suffixes ...int) []string {
	ids := make([]string, len(suffixes))
	for i, suffix := range suffixes {
		ids[i] = string(mockID(seed, suffix))
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
	mockAccountID  = api.ID("account456")
)

func newServerAuth(t *testing.T) (*http.ServeMux, api.CloudflareAuth) {
	t.Helper()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	auth := api.CloudflareAuth{
		Token:   mockToken,
		BaseURL: ts.URL,
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

func newHandle(t *testing.T, ppfmt pp.PP) (*http.ServeMux, api.Handle, bool) {
	t.Helper()

	mux, auth := newServerAuth(t)

	h, ok := auth.New(ppfmt, 8760*time.Hour) // a year
	return mux, h, ok
}

type cloudflareFixture struct {
	t        *testing.T
	mockCtrl *gomock.Controller

	mux      *http.ServeMux
	handle   api.Handle
	cfHandle api.CloudflareHandle
}

func newCloudflareFixture(t *testing.T) *cloudflareFixture {
	t.Helper()

	f := &cloudflareFixture{
		t:        t,
		mockCtrl: gomock.NewController(t),
	}

	mux, h, ok := newHandle(t, mocks.NewMockPP(f.mockCtrl))
	require.True(t, ok)
	ch, ok := h.(api.CloudflareHandle)
	require.True(t, ok)

	f.mux = mux
	f.handle = h
	f.cfHandle = ch
	return f
}

func (f *cloudflareFixture) newPP() *mocks.MockPP {
	f.t.Helper()
	return mocks.NewMockPP(f.mockCtrl)
}

func prepareMockPP(mockPP *mocks.MockPP, prepare func(*mocks.MockPP)) {
	if prepare != nil {
		prepare(mockPP)
	}
}

func assertHandlersExhausted(t *testing.T, handlers ...httpHandler) {
	t.Helper()
	for _, h := range handlers {
		require.True(t, h.isExhausted())
	}
}

func TestNewValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, _, ok := newHandle(t, mockPP)
	require.True(t, ok)
}

func TestNewEmptyToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, auth := newServerAuth(t)

	auth.Token = ""
	mockPP.EXPECT().Noticef(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", gomock.Any())
	h, ok := auth.New(mockPP, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}
