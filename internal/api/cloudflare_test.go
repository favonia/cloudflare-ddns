package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

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

	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
	assert.Empty(t, r.URL.Query())

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

	ppmock := pp.NewMock()
	h, ok := auth.New(context.Background(), ppmock, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
	require.Empty(t, ppmock.Records)

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
	ppmock := pp.NewMock()
	h, ok := auth.New(context.Background(), ppmock, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Failed to prepare the Cloudflare authentication: invalid credentials: API Token must not be empty`), //nolint:lll
		},
		ppmock.Records)
}

func TestNewInvalid(t *testing.T) {
	t.Parallel()

	ts, mux, auth := newServerAuth()
	defer ts.Close()

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
		assert.Empty(t, r.URL.Query())

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w,
			`{
				"success": false,
				"errors": [{ "code": 1000, "message": "Invalid API Token" }],
				"messages": [],
				"result": null
			}`)
	})

	ppmock := pp.NewMock()
	h, ok := auth.New(context.Background(), ppmock, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Error, pp.EmojiUserError, `The Cloudflare API token is not valid: HTTP status 401: Invalid API Token (1000)`), //nolint:lll
			pp.NewRecord(0, pp.Error, pp.EmojiUserError, `Please double-check CF_API_TOKEN or CF_API_TOKEN_FILE`),
		},
		ppmock.Records)
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
		panic("mockZonesResponse got too many zone names")
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

	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
	assert.Equal(t, url.Values{
		"account.id": {mockAccount},
		"name":       {zoneName},
		"per_page":   {"50"},
		"status":     {"active"},
	}, r.URL.Query())

	w.Header().Set("content-type", "application/json")
	err := json.NewEncoder(w).Encode(mockZonesResponse(zoneName, numZones))
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
	*(h.numZones), *(h.accessCount) = numZones, accessCount
}

func (h *zonesHandler) isExhausted() bool {
	return *h.accessCount == 0
}

func TestActiveZonesRoot(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "")
	require.True(t, ok)
	require.Empty(t, zones)
	require.Empty(t, ppmock.Records)
}

func TestActiveZonesTwo(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{"test.org": 2}, 1)
	ppmock := pp.NewMock()
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())
	require.Empty(t, ppmock.Records)

	zh.set(nil, 0)
	ppmock.Clear()
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())
	require.Empty(t, ppmock.Records)

	h.FlushCache()

	ppmock.Clear()
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "test.org": error unmarshalling the JSON response: unexpected end of JSON input`), //nolint:lll
		},
		ppmock.Records)
}

func TestActiveZonesEmpty(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)

	zh.set(map[string]int{}, 1)
	ppmock := pp.NewMock()
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.True(t, ok)
	require.Empty(t, zones)
	require.True(t, zh.isExhausted())
	require.Empty(t, ppmock.Records)

	zh.set(nil, 0) // this should not affect the result due to the caching
	ppmock.Clear()
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.True(t, ok)
	require.Empty(t, zones)
	require.True(t, zh.isExhausted())
	require.Empty(t, ppmock.Records)

	h.FlushCache()

	ppmock.Clear()
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), ppmock, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "test.org": error unmarshalling the JSON response: unexpected end of JSON input`), //nolint:lll
		},
		ppmock.Records)
}

func TestZoneOfDomain(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zone        string
		domain      api.FQDN
		numZones    map[string]int
		accessCount int
		expected    string
		ok          bool
		ppRecords   []pp.Record
	}{
		"root": {"test.org", "test.org", map[string]int{"test.org": 1}, 1, mockID("test.org", 0), true, nil},
		"one":  {"test.org", "sub.test.org", map[string]int{"test.org": 1}, 2, mockID("test.org", 0), true, nil},
		"none": {
			"test.org", "sub.test.org",
			map[string]int{},
			3, "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to find the zone of "sub.test.org"`),
			},
		},
		"multiple": {
			"test.org", "sub.test.org",
			map[string]int{"test.org": 2},
			2, "", false,
			[]pp.Record{
				pp.NewRecord(0, pp.Warning, pp.EmojiImpossible, `Found multiple active zones named "test.org". Specifying CF_ACCOUNT_ID might help`), //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts, mux, h := newHandle(t)
			defer ts.Close()

			zh := newZonesHandler(t, mux)

			zh.set(tc.numZones, tc.accessCount)
			ppmock := pp.NewMock()
			zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), ppmock, tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			require.True(t, zh.isExhausted())
			require.Equal(t, tc.ppRecords, ppmock.Records)

			if tc.ok {
				zh.set(nil, 0)
				ppmock.Clear()
				zoneID, ok = h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), ppmock, tc.domain)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.expected, zoneID)
				require.True(t, zh.isExhausted())
				require.Equal(t, tc.ppRecords, ppmock.Records)
			}
		})
	}
}

func TestZoneOfDomainInvalid(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), ppmock, "sub.test.org")
	require.False(t, ok)
	require.Equal(t, "", zoneID)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func mockDNSRecord(id string, ipNet ipnet.Type, domain api.FQDN, ip net.IP) *cloudflare.DNSRecord {
	return &cloudflare.DNSRecord{ //nolint:exhaustivestruct
		ID:      id,
		Type:    ipNet.RecordType(),
		Name:    domain.ToASCII(),
		Content: ip.String(),
	}
}

//nolint:unparam // domain always = "test.org" for now
func mockDNSListResponse(ipNet ipnet.Type, domain api.FQDN, ips map[string]net.IP) *cloudflare.DNSListResponse {
	if len(ips) > 100 {
		panic("mockDNSResponse got too many IPs")
	}

	rs := make([]cloudflare.DNSRecord, 0, len(ips))
	for id, ip := range ips {
		rs = append(rs, *mockDNSRecord(id, ipNet, domain, ip))
	}

	return &cloudflare.DNSListResponse{
		Result: rs,
		ResultInfo: cloudflare.ResultInfo{
			Page:       1,
			PerPage:    100,
			TotalPages: (len(ips) + 99) / 100,
			Count:      len(ips),
			Total:      len(ips),
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

func TestListRecords(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	var (
		ipNet       ipnet.Type
		ips         map[string]net.IP
		accessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if accessCount <= 0 {
				return
			}
			accessCount--

			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
			assert.Equal(t, url.Values{
				"name":     {"sub.test.org"},
				"page":     {"1"},
				"per_page": {"100"},
				"type":     {ipNet.RecordType()},
			}, r.URL.Query())

			w.Header().Set("content-type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSListResponse(ipNet, "test.org", ips))
			assert.NoError(t, err)
		})

	expected := map[string]net.IP{"record1": net.ParseIP("::1"), "record2": net.ParseIP("::2")}
	ipNet, ips, accessCount = ipnet.IP6, expected, 1
	ppmock := pp.NewMock()
	ips, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.True(t, ok)
	require.Equal(t, expected, ips)
	require.Equal(t, 0, accessCount)
	require.Empty(t, ppmock.Records)

	// testing the caching
	ppmock.Clear()
	ips, ok = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.True(t, ok)
	require.Equal(t, expected, ips)
	require.Empty(t, ppmock.Records)
}

func TestListRecordsInvalidDomain(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	ppmock := pp.NewMock()
	ips, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP4)
	require.False(t, ok)
	require.Nil(t, ips)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to retrieve records of "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)

	ppmock.Clear()
	ips, ok = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.False(t, ok)
	require.Nil(t, ips)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to retrieve records of "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func TestListRecordsInvalidZone(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	ips, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP4)
	require.False(t, ok)
	require.Nil(t, ips)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)

	ppmock.Clear()
	ips, ok = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.False(t, ok)
	require.Nil(t, ips)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func envelopDNSRecordResponse(record *cloudflare.DNSRecord) *cloudflare.DNSRecordResponse {
	return &cloudflare.DNSRecordResponse{
		Result: *record,
		ResultInfo: cloudflare.ResultInfo{
			Page:       1,
			PerPage:    100,
			TotalPages: 1,
			Count:      1,
			Total:      1,
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

func mockDNSRecordResponse(id string, ipNet ipnet.Type, domain api.FQDN, ip net.IP) *cloudflare.DNSRecordResponse {
	return envelopDNSRecordResponse(mockDNSRecord(id, ipNet, domain, ip))
}

func TestDeleteRecordsValid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	var (
		listAccessCount   int
		deleteAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if listAccessCount <= 0 {
				return
			}
			listAccessCount--

			w.Header().Set("content-type", "application/json")
			_ = json.NewEncoder(w).Encode(mockDNSListResponse(ipnet.IP6, "test.org",
				map[string]net.IP{"record1": net.ParseIP("::1")}))
		})

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records/record1", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if deleteAccessCount <= 0 {
				return
			}
			deleteAccessCount--

			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
			assert.Empty(t, r.URL.Query())

			w.Header().Set("content-type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSRecordResponse("record1", ipnet.IP6, "test.org", net.ParseIP("::1")))
			assert.NoError(t, err)
		})

	deleteAccessCount = 1
	ppmock := pp.NewMock()
	ok := h.DeleteRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1")
	require.True(t, ok)
	require.Empty(t, ppmock.Records)

	listAccessCount, deleteAccessCount = 1, 1
	ppmock.Clear()
	_, _ = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	_ = h.DeleteRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1")
	rs, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.True(t, ok)
	require.Empty(t, rs)
	require.Empty(t, ppmock.Records)
}

func TestDeleteRecordsInvalid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	ppmock := pp.NewMock()
	ok := h.DeleteRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1")
	require.False(t, ok)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to delete a stale AAAA record of "sub.test.org" (ID: record1): error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func TestDeleteRecordsZoneInvalid(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	ok := h.DeleteRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1")
	require.False(t, ok)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

//nolint:funlen
func TestUpdateRecordValid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	var (
		listAccessCount   int
		updateAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			if listAccessCount <= 0 {
				return
			}
			listAccessCount--

			w.Header().Set("content-type", "application/json")
			_ = json.NewEncoder(w).Encode(mockDNSListResponse(ipnet.IP6, "test.org",
				map[string]net.IP{"record1": net.ParseIP("::1")}))
		})

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records/record1", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			if updateAccessCount <= 0 {
				return
			}
			updateAccessCount--

			assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
			assert.Empty(t, r.URL.Query())

			var record cloudflare.DNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, "sub.test.org", record.Name)
			assert.Equal(t, ipnet.IP6.RecordType(), record.Type)
			assert.Equal(t, "::2", record.Content)

			w.Header().Set("content-type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSRecordResponse("record1", ipnet.IP6, "sub.test.org", net.ParseIP("::2")))
			assert.NoError(t, err)
		})

	updateAccessCount = 1
	ppmock := pp.NewMock()
	ok := h.UpdateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1", net.ParseIP("::2"))
	require.True(t, ok)
	require.Empty(t, ppmock.Records)

	listAccessCount, updateAccessCount = 1, 1
	ppmock.Clear()
	_, _ = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	_ = h.UpdateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1", net.ParseIP("::2"))
	rs, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.True(t, ok)
	require.Equal(t, map[string]net.IP{"record1": net.ParseIP("::2")}, rs)
	require.Empty(t, ppmock.Records)
}

func TestUpdateRecordInvalid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	ppmock := pp.NewMock()
	ok := h.UpdateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1", net.ParseIP("::1"))
	require.False(t, ok)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to update a stale AAAA record of "sub.test.org" (ID: record1): error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func TestUpdateRecordInvalidZone(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	ok := h.UpdateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, "record1", net.ParseIP("::1"))
	require.False(t, ok)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

//nolint:funlen
func TestCreateRecordValid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	var (
		listAccessCount   int
		createAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				if listAccessCount <= 0 {
					return
				}
				listAccessCount--

				w.Header().Set("content-type", "application/json")
				_ = json.NewEncoder(w).Encode(mockDNSListResponse(ipnet.IP6, "test.org",
					map[string]net.IP{"record1": net.ParseIP("::1")}))
			case http.MethodPost:
				if createAccessCount <= 0 {
					return
				}
				createAccessCount--

				assert.Equal(t, []string{fmt.Sprintf("Bearer %s", mockToken)}, r.Header["Authorization"])
				assert.Empty(t, r.URL.Query())

				var record cloudflare.DNSRecord
				if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, "sub.test.org", record.Name)
				assert.Equal(t, ipnet.IP6.RecordType(), record.Type)
				assert.Equal(t, "::1", record.Content)
				assert.Equal(t, 100, record.TTL)
				assert.Equal(t, false, *record.Proxied)
				record.ID = "record1"

				w.Header().Set("content-type", "application/json")
				err := json.NewEncoder(w).Encode(envelopDNSRecordResponse(&record))
				assert.NoError(t, err)
			}
		})

	createAccessCount = 1
	ppmock := pp.NewMock()
	actualID, ok := h.CreateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, net.ParseIP("::1"), 100, false) //nolint:lll
	require.True(t, ok)
	require.Equal(t, "record1", actualID)
	require.Empty(t, ppmock.Records)

	listAccessCount, createAccessCount = 1, 1
	ppmock.Clear()
	_, _ = h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	_, _ = h.CreateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, net.ParseIP("::1"), 100, false)
	rs, ok := h.ListRecords(context.Background(), ppmock, "sub.test.org", ipnet.IP6)
	require.True(t, ok)
	require.Equal(t, map[string]net.IP{"record1": net.ParseIP("::1")}, rs)
	require.Empty(t, ppmock.Records)
}

func TestCreateRecordInvalid(t *testing.T) {
	t.Parallel()

	ts, mux, h := newHandle(t)
	defer ts.Close()

	zh := newZonesHandler(t, mux)
	zh.set(map[string]int{"test.org": 1}, 2)

	ppmock := pp.NewMock()
	actualID, ok := h.CreateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, net.ParseIP("::1"), 100, false) //nolint:lll
	require.False(t, ok)
	require.Equal(t, "", actualID)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to add a new AAAA record of "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}

func TestCreateRecordInvalidZone(t *testing.T) {
	t.Parallel()

	ts, _, h := newHandle(t)
	defer ts.Close()

	ppmock := pp.NewMock()
	actualID, ok := h.CreateRecord(context.Background(), ppmock, "sub.test.org", ipnet.IP6, net.ParseIP("::1"), 100, false) //nolint:lll
	require.False(t, ok)
	require.Equal(t, "", actualID)
	require.Equal(t,
		[]pp.Record{
			pp.NewRecord(0, pp.Warning, pp.EmojiError, `Failed to check the existence of a zone named "sub.test.org": error unmarshalling the JSON response error body: invalid character 'p' after top-level value`), //nolint:lll
		},
		ppmock.Records)
}
