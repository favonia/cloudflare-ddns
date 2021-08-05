package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

// mockID returns a hex string of length 32, suitable for all kinds of IDs
// used in the Cloudflare API.
func mockID(seed string) string {
	arr := sha512.Sum512([]byte(seed)) //nolint:gosec
	return hex.EncodeToString(arr[:16])
}

const (
	mockToken   = "token123"
	mockAccount = "account456"
)

func TestCloudflareAuthNewSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])

		w.Header().Set("content-type", "application/json")
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
			}`, mockID("result"))
	})

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.NotNil(t, h)
	require.True(t, ok)
}

func TestCloudflareAuthNewEmpty(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	auth := api.CloudflareAuth{
		Token:     "",
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.Nil(t, h)
	require.False(t, ok)
}

func TestCloudflareAuthNewInvalid(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])

		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("content-type", "application/json")
		fmt.Fprintf(w,
			`{
			  "success": false,
			  "errors": [{ "code": 1000, "message": "Invalid API Token" }],
			  "messages": [],
			  "result": null
			}`)
	})

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.Nil(t, h)
	require.False(t, ok)
}
