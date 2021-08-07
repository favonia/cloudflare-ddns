package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

// mockID returns a hex string of length 32, suitable for all kinds of IDs
// used in the Cloudflare API.
func mockID(seed string, indexes ...int) string {
	for _, i := range indexes {
		seed = fmt.Sprintf("%s/%d", seed, i)
	}
	arr := sha512.Sum512([]byte(seed))
	return hex.EncodeToString(arr[:16])
}

const (
	mockToken     = "token123"
	mockAccount   = "account456"
	mockOwnerName = "Test Account"
)

func handleTokensVerify(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	switch {
	case !assert.Equal(t, http.MethodGet, r.Method):
		return
	case !assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"]):
		return
	}

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
			}`,
		mockID("result"))
}

func TestCloudflareAuthNewValid(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleTokensVerify(t, w, r)
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
		switch {
		case !assert.Equal(t, http.MethodGet, r.Method):
			return
		case !assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"]):
			return
		}

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

func mockZone(zoneName string, i int) *cloudflare.Zone {
	return &cloudflare.Zone{ //nolint:exhaustivestruct
		ID:   mockID(zoneName, i),
		Name: zoneName,
		Owner: cloudflare.Owner{ //nolint:exhaustivestruct
			ID:        mockID(mockOwnerName),
			Name:      mockOwnerName,
			OwnerType: "organization",
		},
		Status: "active",
		Type:   "full",
		Account: cloudflare.Account{ //nolint:exhaustivestruct
			ID:   mockID(mockOwnerName),
			Name: mockOwnerName,
		},
	}
}

func mockZonesResponse(zoneName string, numZones int) *cloudflare.ZonesResponse {
	if numZones > 50 {
		panic("mockZonesResponse got too many zone names.")
	}

	zones := make([]cloudflare.Zone, numZones)
	for i := 0; i < numZones; i++ {
		zones[i] = *mockZone(zoneName, i)
	}

	return &cloudflare.ZonesResponse{
		Result: zones,
		ResultInfo: cloudflare.ResultInfo{
			Page:       1,
			PerPage:    50,
			TotalPages: 1,
			Count:      numZones,
			Total:      numZones,
			Cursor:     "",
			Cursors:    cloudflare.ResultInfoCursors{}, //nolint:exhaustivestruct
		},
		Response: cloudflare.Response{
			Success:  true,
			Errors:   []cloudflare.ResponseInfo{},
			Messages: []cloudflare.ResponseInfo{},
		},
	}
}

func handleZones(t *testing.T, zoneName string, numZones int, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	switch {
	case !assert.Equal(t, http.MethodGet, r.Method):
		return
	case !assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"]):
		return
	case !assert.Equal(t, map[string][]string{
		"account.id": {mockAccount},
		"name":       {zoneName},
		"per_page":   {"50"},
		"status":     {"active"},
	}, map[string][]string(r.URL.Query())):
		return
	}

	w.Header().Set("content-type", "application/json")
	bytes, err := json.Marshal(mockZonesResponse(zoneName, numZones))
	if !assert.NoError(t, err) {
		return
	}

	if _, err := w.Write(bytes); assert.NoError(t, err) {
		return
	}
}

//nolint:paralleltest // caching cannot be easily tested
func TestCloudflareActiveZonesOneZone(t *testing.T) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleTokensVerify(t, w, r)
	})

	const mockZoneName = "test.org"
	var (
		numMockZones = 1
		accessCount  = 1
	)

	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		if accessCount <= 0 {
			return
		}
		accessCount--

		handleZones(t, mockZoneName, numMockZones, w, r)
	})

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.NotNil(t, h)
	require.True(t, ok)

	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, mockZoneName)
	require.Equal(t, []string{mockID(mockZoneName, 0)}, zones)
	require.True(t, ok)

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, mockZoneName)
	require.Equal(t, []string{mockID(mockZoneName, 0)}, zones)
	require.True(t, ok)

	h.FlushCache()

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, mockZoneName)
	require.Nil(t, zones)
	require.False(t, ok)

	accessCount = 1
	numMockZones = 2

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, mockZoneName)
	require.Equal(t, []string{mockID(mockZoneName, 0), mockID(mockZoneName, 1)}, zones)
	require.True(t, ok)

	numMockZones = 0 // this should not affect the result due to the caching

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, mockZoneName)
	require.Equal(t, []string{mockID(mockZoneName, 0), mockID(mockZoneName, 1)}, zones)
	require.True(t, ok)
}
