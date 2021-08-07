package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

// mockID returns a hex string of length 32, suitable for all kinds of IDs
// used in the Cloudflare API.
func mockID(seed string, i int) string {
	seed = fmt.Sprintf("%s/%d", seed, i)
	arr := sha512.Sum512([]byte(seed))
	return hex.EncodeToString(arr[:16])
}

func mockIDs(seed string, is ...int) []string {
	ids := make([]string, 0, len(is))
	for _, i := range is {
		ids = append(ids, mockID(seed, i))
	}
	return ids
}

const (
	mockToken   = "token123"
	mockAccount = "account456"
)

func newServerAuth() (*httptest.Server, *http.ServeMux, *api.CloudflareAuth) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	auth := api.CloudflareAuth{
		Token:     mockToken,
		AccountID: mockAccount,
		BaseURL:   ts.URL,
	}

	return ts, mux, &auth
}

func handleTokensVerify(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	switch {
	case !assert.Equal(t, http.MethodGet, r.Method):
		return
	case !assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"]):
		return
	case !assert.Equal(t, url.Values{}, r.URL.Query()):
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
		mockID("result", 0))
}

func newHandle(t *testing.T) (*httptest.Server, *http.ServeMux, api.Handle) {
	t.Helper()

	ts, mux, auth := newServerAuth()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleTokensVerify(t, w, r)
	})

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)

	return ts, mux, h
}

func TestNewValid(t *testing.T) {
	t.Parallel()

	ts, _, _ := newHandle(t)
	defer ts.Close()
}

func TestNewEmpty(t *testing.T) {
	t.Parallel()

	ts, _, auth := newServerAuth()
	defer ts.Close()

	auth.Token = ""
	h, ok := auth.New(context.Background(), 3, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}

func TestNewInvalid(t *testing.T) {
	t.Parallel()

	ts, mux, auth := newServerAuth()
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

	h, ok := auth.New(context.Background(), 3, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
}

func mockZone(zoneName string, i int) *cloudflare.Zone {
	return &cloudflare.Zone{ //nolint:exhaustivestruct
		ID:     mockID(zoneName, i),
		Name:   zoneName,
		Status: "active",
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
			TotalPages: (numZones + 49) / 50,
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
	case !assert.Equal(t, url.Values{
		"account.id": {mockAccount},
		"name":       {zoneName},
		"per_page":   {"50"},
		"status":     {"active"},
	}, r.URL.Query()):
		return
	}

	bytes, err := json.Marshal(mockZonesResponse(zoneName, numZones))
	if !assert.NoError(t, err) {
		return
	}

	w.Header().Set("content-type", "application/json")
	_, err = w.Write(bytes)
	assert.NoError(t, err)
}

type zonesHandler struct {
	mux         *http.ServeMux
	numZones    *map[string]int
	accessCount *int
}

func newZonesHandler(t *testing.T, mux *http.ServeMux) *zonesHandler {
	t.Helper()

	var (
		numZones    map[string]int
		accessCount int
	)

	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		if accessCount <= 0 {
			return
		}
		accessCount--

		zoneName := r.URL.Query().Get("name")
		handleZones(t, zoneName, numZones[zoneName], w, r)
	})

	return &zonesHandler{
		mux:         mux,
		numZones:    &numZones,
		accessCount: &accessCount,
	}
}

func (h *zonesHandler) set(numZones map[string]int, accessCount int) {
	*(h.numZones) = numZones
	*(h.accessCount) = accessCount
}

func (h *zonesHandler) isExhausted() bool {
	return *h.accessCount == 0
}

func TestActiveZonesRoot(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "")
	require.True(t, ok)
	require.Equal(t, []string{}, zones)
}

func TestActiveZonesTwo(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{"test.org": 2}, 1)
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0)
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())

	h.FlushCache()

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
}

func TestActiveZonesEmpty(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{}, 1)
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, []string{}, zones)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0) // this should not affect the result due to the caching
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, []string{}, zones)
	require.True(t, zh.isExhausted())

	h.FlushCache()

	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), 3, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
}

func TestZoneOfDomainRoot(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{"test.org": 1}, 1)
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0)
	zoneID, ok = h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "test.org")
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
}

func TestZoneOfDomainOneLevel(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{"test.org": 1}, 2)
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "sub.test.org")
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0)
	zoneID, ok = h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "sub.test.org")
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
}

func TestZoneOfDomainNone(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{}, 3)
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "sub.test.org")
	require.False(t, ok)
	require.Equal(t, "", zoneID)
	require.True(t, zh.isExhausted())
}

func TestZoneOfDomainMultiple(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{"test.org": 2}, 2)
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "sub.test.org")
	require.False(t, ok)
	require.Equal(t, "", zoneID)
	require.True(t, zh.isExhausted())
}

func TestZoneOfDomainInvalid(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), 3, "sub.test.org")
	require.False(t, ok)
	require.Equal(t, "", zoneID)
}
