package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

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
	h, ok := auth.New(mockPP, defaultHandleOptions())
	require.False(t, ok)
	require.Nil(t, h)
}

func TestVerifyPassed(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"success": true,
			"errors": [],
			"messages": [],
			"result": {
				"id": "ed17574386854bf78a67040be0a770b0",
				"status": "active",
				"not_before": "2018-07-01T05:20:00Z",
				"expires_on": "2020-01-01T00:00:00Z"
			}
		}`)
	})

	require.True(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestVerifyFailedInvalidToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{
			"success": false,
			"errors": [{"code":1000,"message":"Invalid API Token"}],
			"messages": [],
			"result": null
		}`)
	})

	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "The Cloudflare API token appears to be invalid: %v", gomock.Any()),
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			"Please double-check the value of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE"),
	)

	require.False(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestVerifyFailedInvalidAuthorizationHeader(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{
			"success": false,
			"errors": [{
				"code": 6003,
				"message": "Invalid request headers",
				"error_chain": [{"code": 6111, "message": "Invalid format for Authorization header"}]
			}],
			"messages": [],
			"result": null
		}`)
	})

	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "The Cloudflare API token appears to be invalid: %v", gomock.Any()),
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			"Please double-check the value of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE"),
	)

	require.False(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestVerifyUnexpectedAuthorizationFailureIsUncertain(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{
			"success": false,
			"errors": [{"code":9109,"message":"Unauthorized to access requested resource"}],
			"messages": [],
			"result": null
		}`)
	})

	mockPP.EXPECT().Noticef(pp.EmojiWarning,
		"Unexpected authorization failure while verifying the Cloudflare API token: %v; startup will continue",
		gomock.Any())

	require.True(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestVerifyFailedExpiredToken(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"success": true,
			"errors": [],
			"messages": [],
			"result": {
				"id": "ed17574386854bf78a67040be0a770b0",
				"status": "expired",
				"not_before": "2018-07-01T05:20:00Z",
				"expires_on": "2020-01-01T00:00:00Z"
			}
		}`)
	})

	gomock.InOrder(
		mockPP.EXPECT().Noticef(pp.EmojiUserError, "The Cloudflare API token is %s", "expired"),
		mockPP.EXPECT().Noticef(pp.EmojiUserError,
			"Please double-check the value of CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE"),
	)

	require.False(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestCheckUsabilityTimeout(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(_ http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		<-r.Context().Done()
	})

	mockPP.EXPECT().Noticef(pp.EmojiWarning,
		"Cloudflare API token verification timed out after %v; the updater will continue",
		time.Second)

	require.True(t, auth.CheckUsability(context.Background(), mockPP))
}

func TestCheckUsabilityUnexpectedVerifyFailureIsUncertain(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	serveMux, auth := newServerAuth(t)
	serveMux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, http.MethodGet, r.Method) || !checkToken(t, r) {
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprint(w, `{`)
	})

	mockPP.EXPECT().Noticef(pp.EmojiWarning,
		"Cloudflare API token verification failed: %v; the updater will continue",
		gomock.Any())

	require.True(t, auth.CheckUsability(context.Background(), mockPP))
}
