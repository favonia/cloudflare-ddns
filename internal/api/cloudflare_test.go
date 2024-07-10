package api_test

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
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

func newServerAuth(t *testing.T, emptyAccountID bool) (*http.ServeMux, *api.CloudflareAuth) {
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

	return mux, &auth
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

func newHandle(t *testing.T, emptyAccountID bool) (*http.ServeMux, api.Handle) {
	t.Helper()
	mockCtrl := gomock.NewController(t)

	mux, auth := newServerAuth(t, emptyAccountID)

	mux.HandleFunc("/user/tokens/verify", func(w http.ResponseWriter, r *http.Request) {
		handleSanityCheck(t, w, r)
	})

	mockPP := mocks.NewMockPP(mockCtrl)
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)

	return mux, h
}

func TestNewValid(t *testing.T) {
	t.Parallel()

	newHandle(t, false)
}

func TestNewEmpty(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, auth := newServerAuth(t, false)

	auth.Token = ""
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Errorf(pp.EmojiUserError, "Failed to prepare the Cloudflare authentication: %v", gomock.Any())
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.False(t, ok)
	require.Nil(t, h)
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
			require.False(t, ok)
			require.Nil(t, h)
		})
	}
}

func TestNewTimeout(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	mux, auth := newServerAuth(t, false)
	mux.HandleFunc("/user/tokens/verify", func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
		assert.Empty(t, r.URL.Query())

		panic(http.ErrAbortHandler)
	})

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Errorf(pp.EmojiError, "The Cloudflare API token could not be verified: %v", gomock.Any())
	h, ok := auth.New(context.Background(), mockPP, time.Second)
	require.True(t, ok)
	require.NotNil(t, h)
}

func mockZone(name string, i int, status string) *cloudflare.Zone {
	return &cloudflare.Zone{ //nolint:exhaustruct
		ID:     mockID(name, i),
		Name:   name,
		Status: status,
	}
}

const (
	zonePageSize      = 50
	dnsRecordPageSize = 100
)

func mockZonesResponse(zoneName string, zoneStatuses []string) *cloudflare.ZonesResponse {
	numZones := len(zoneStatuses)

	if numZones > zonePageSize {
		panic("mockZonesResponse got too many zone names")
	}

	zones := make([]cloudflare.Zone, len(zoneStatuses))
	for i, status := range zoneStatuses {
		zones[i] = *mockZone(zoneName, i, status)
	}

	return &cloudflare.ZonesResponse{
		Result: zones,
		ResultInfo: cloudflare.ResultInfo{
			Page:       1,
			PerPage:    zonePageSize,
			TotalPages: (numZones + zonePageSize - 1) / zonePageSize,
			Count:      numZones,
			Total:      numZones,
			Cursor:     "",
			Cursors:    cloudflare.ResultInfoCursors{}, //nolint:exhaustruct
		},
		Response: cloudflare.Response{
			Success:  true,
			Errors:   []cloudflare.ResponseInfo{},
			Messages: []cloudflare.ResponseInfo{},
		},
	}
}

func handleZones(
	t *testing.T, zoneName string, zoneStatuses []string, emptyAccountID bool, w http.ResponseWriter, r *http.Request,
) {
	t.Helper()

	require.Equal(t, http.MethodGet, r.Method)
	require.Equal(t, []string{mockAuthString}, r.Header["Authorization"])
	if emptyAccountID {
		require.Equal(t, url.Values{
			"name":     {zoneName},
			"per_page": {strconv.Itoa(zonePageSize)},
		}, r.URL.Query())
	} else {
		require.Equal(t, url.Values{
			"account.id": {mockAccount},
			"name":       {zoneName},
			"per_page":   {strconv.Itoa(zonePageSize)},
		}, r.URL.Query())
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockZonesResponse(zoneName, zoneStatuses))
	require.NoError(t, err)
}

type zonesHandler struct {
	mux          *http.ServeMux
	zoneStatuses *map[string][]string
	accessCount  *int
}

func newZonesHandler(t *testing.T, mux *http.ServeMux, emptyAccountID bool) *zonesHandler {
	t.Helper()

	var (
		zoneStatuses map[string][]string
		accessCount  int
	)

	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		if accessCount <= 0 {
			panic(http.ErrAbortHandler)
		}
		accessCount--

		zoneName := r.URL.Query().Get("name")
		handleZones(t, zoneName, zoneStatuses[zoneName], emptyAccountID, w, r)
	})

	return &zonesHandler{
		mux:          mux,
		zoneStatuses: &zoneStatuses,
		accessCount:  &accessCount,
	}
}

func (h *zonesHandler) set(zoneStatuses map[string][]string, accessCount int) {
	*(h.zoneStatuses), *(h.accessCount) = zoneStatuses, accessCount
}

func (h *zonesHandler) isExhausted() bool {
	return *h.accessCount == 0
}

func TestActiveZonesRoot(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "")
	require.True(t, ok)
	require.Empty(t, zones)
}

func TestActiveZonesTwo(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)

	zh.set(map[string][]string{"test.org": {"active", "active"}}, 1)
	mockPP := mocks.NewMockPP(mockCtrl)
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0)
	mockPP = mocks.NewMockPP(mockCtrl)
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), zones)
	require.True(t, zh.isExhausted())

	h.FlushCache()

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"test.org",
		gomock.Any(),
	)
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
}

func TestActiveZonesEmpty(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)

	zh.set(map[string][]string{}, 1)
	mockPP := mocks.NewMockPP(mockCtrl)
	zones, ok := h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.True(t, ok)
	require.Empty(t, zones)
	require.True(t, zh.isExhausted())

	zh.set(nil, 0) // this should not affect the result due to the caching
	mockPP = mocks.NewMockPP(mockCtrl)
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.True(t, ok)
	require.Empty(t, zones)
	require.True(t, zh.isExhausted())

	h.FlushCache()

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"test.org",
		gomock.Any(),
	)
	zones, ok = h.(*api.CloudflareHandle).ActiveZones(context.Background(), mockPP, "test.org")
	require.False(t, ok)
	require.Nil(t, zones)
	require.True(t, zh.isExhausted())
}

//nolint:funlen
func TestZoneOfDomain(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		emptyAccountID bool
		zone           string
		domain         domain.Domain
		zoneStatuses   map[string][]string
		accessCount    int
		expected       string
		ok             bool
		prepareMockPP  func(*mocks.MockPP)
	}{
		"root":     {false, "test.org", domain.FQDN("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil},     //nolint:lll
		"wildcard": {false, "test.org", domain.Wildcard("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil}, //nolint:lll
		"one":      {false, "test.org", domain.FQDN("sub.test.org"), map[string][]string{"test.org": {"active"}}, 2, mockID("test.org", 0), true, nil}, //nolint:lll
		"none": {
			false, "test.org", domain.FQDN("sub.test.org"),
			map[string][]string{},
			3, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "sub.test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; in most cases, you can leave it blank"), //nolint:lll
				)
			},
		},
		"none/wildcard": {
			false, "test.org", domain.Wildcard("test.org"),
			map[string][]string{},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "*.test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; in most cases, you can leave it blank"), //nolint:lll
				)
			},
		},
		"multiple": {
			false, "test.org", domain.FQDN("sub.test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Found multiple active zones named %q. Specifying CF_ACCOUNT_ID might help",
						"test.org",
					),
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Please consider reporting this rare situation at https://github.com/favonia/cloudflare-ddns/issues/new",
					),
				)
			},
		},
		"multiple/wildcard": {
			false, "test.org", domain.Wildcard("test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			1, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Found multiple active zones named %q. Specifying CF_ACCOUNT_ID might help",
						"test.org",
					),
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Please consider reporting this rare situation at https://github.com/favonia/cloudflare-ddns/issues/new",
					),
				)
			},
		},
		"deleted": {
			false, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"deleted"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", "test.org", "deleted"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; in most cases, you can leave it blank"), //nolint:lll
				)
			},
		},
		"deleted/empty-account": {
			true, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"deleted"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", "test.org", "deleted"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "test.org"),
				)
			},
		},
		"pending": {
			false, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"pending"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", "test.org", "pending"), //nolint:lll
				)
			},
		},
		"initializing": {
			false, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"initializing"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", "test.org", "initializing"), //nolint:lll
				)
			},
		},
		"undocumented": {
			false, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"some-undocumented-status"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiImpossible, "Zone %q is in an undocumented status %q", "test.org", "some-undocumented-status"), //nolint:lll
					m.EXPECT().Warningf(pp.EmojiImpossible, "Please report the bug at https://github.com/favonia/cloudflare-ddns/issues/new"),  //nolint:lll
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mux, h := newHandle(t, tc.emptyAccountID)

			zh := newZonesHandler(t, mux, tc.emptyAccountID)

			zh.set(tc.zoneStatuses, tc.accessCount)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			require.True(t, zh.isExhausted())

			if tc.ok {
				zh.set(nil, 0)
				mockPP = mocks.NewMockPP(mockCtrl) // there shouldn't be any messages
				zoneID, ok = h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, tc.domain)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.expected, zoneID)
				require.True(t, zh.isExhausted())
			}
		})
	}
}

func TestZoneOfDomainInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	zoneID, ok := h.(*api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.Equal(t, "", zoneID)
}

func mockDNSRecord(id string, ipNet ipnet.Type, name string, ip string) *cloudflare.DNSRecord {
	return &cloudflare.DNSRecord{ //nolint:exhaustruct
		ID:      id,
		Type:    ipNet.RecordType(),
		Name:    name,
		Content: ip,
	}
}

func mockDNSListResponse(ipNet ipnet.Type, name string, ips map[string]string) *cloudflare.DNSListResponse {
	if len(ips) > 100 {
		panic("mockDNSResponse got too many IPs")
	}

	rs := make([]cloudflare.DNSRecord, 0, len(ips))
	for id, ip := range ips {
		rs = append(rs, *mockDNSRecord(id, ipNet, name, ip))
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
			Cursors:    cloudflare.ResultInfoCursors{}, //nolint:exhaustruct
		},
		Response: cloudflare.Response{
			Success:  true,
			Errors:   []cloudflare.ResponseInfo{},
			Messages: []cloudflare.ResponseInfo{},
		},
	}
}

func mockDNSListResponseFromAddr(ipNet ipnet.Type, name string, ips map[string]netip.Addr) *cloudflare.DNSListResponse {
	if len(ips) > 100 {
		panic("mockDNSResponse got too many IPs")
	}

	strings := make(map[string]string)

	for id, ip := range ips {
		strings[id] = ip.String()
	}

	return mockDNSListResponse(ipNet, name, strings)
}

//nolint:dupl
func TestListRecords(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		ipNet       ipnet.Type
		ips         map[string]netip.Addr
		accessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if accessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			accessCount--

			if !assert.Equal(t, http.MethodGet, r.Method) ||
				!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
				!assert.Equal(t, url.Values{
					"name":     {"sub.test.org"},
					"page":     {"1"},
					"per_page": {strconv.Itoa(dnsRecordPageSize)},
					"type":     {ipNet.RecordType()},
				}, r.URL.Query()) {
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSListResponseFromAddr(ipNet, "test.org", ips),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	expected := map[string]netip.Addr{"record1": mustIP("::1"), "record2": mustIP("::2")}
	ipNet, ips, accessCount = ipnet.IP6, expected, 1
	mockPP := mocks.NewMockPP(mockCtrl)
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, ips)
	require.Equal(t, 0, accessCount)

	// testing the caching
	mockPP = mocks.NewMockPP(mockCtrl)
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, expected, ips)
}

//nolint:funlen
func TestListRecordsInvalidIPAddress(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		ipNet       ipnet.Type
		ips         map[string]netip.Addr
		accessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if accessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			accessCount--

			if !assert.Equal(t, http.MethodGet, r.Method) ||
				!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
				!assert.Equal(t, url.Values{
					"name":     {"sub.test.org"},
					"page":     {"1"},
					"per_page": {strconv.Itoa(dnsRecordPageSize)},
					"type":     {ipNet.RecordType()},
				}, r.URL.Query()) {
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(mockDNSListResponse(ipNet, "test.org",
				map[string]string{"record1": "::1", "record2": "NOT AN IP"},
			)); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	ipNet, accessCount = ipnet.IP6, 1
	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiImpossible,
		"Failed to parse the IP address in records of %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
	require.Equal(t, 0, accessCount)

	// testing the (no) caching
	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to retrieve records of %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
	require.Equal(t, 0, accessCount)
}

//nolint:dupl
func TestListRecordsWildcard(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 1)

	var (
		ipNet       ipnet.Type
		ips         map[string]netip.Addr
		accessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if accessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			accessCount--

			if !assert.Equal(t, http.MethodGet, r.Method) ||
				!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
				!assert.Equal(t, url.Values{
					"name":     {"*.test.org"},
					"page":     {"1"},
					"per_page": {strconv.Itoa(dnsRecordPageSize)},
					"type":     {ipNet.RecordType()},
				}, r.URL.Query()) {
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSListResponseFromAddr(ipNet, "*.test.org", ips),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	expected := map[string]netip.Addr{"record1": mustIP("::1"), "record2": mustIP("::2")}
	ipNet, ips, accessCount = ipnet.IP6, expected, 1
	mockPP := mocks.NewMockPP(mockCtrl)
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.Wildcard("test.org"), ipnet.IP6)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, ips)
	require.Equal(t, 0, accessCount)

	// testing the caching
	mockPP = mocks.NewMockPP(mockCtrl)
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.Wildcard("test.org"), ipnet.IP6)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, expected, ips)
}

func TestListRecordsInvalidDomain(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to retrieve records of %q: %v", "sub.test.org", gomock.Any())
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP4)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to retrieve records of %q: %v", "sub.test.org", gomock.Any())
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
}

func TestListRecordsInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP4)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
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
			Cursors:    cloudflare.ResultInfoCursors{}, //nolint:exhaustruct
		},
		Response: cloudflare.Response{
			Success:  true,
			Errors:   []cloudflare.ResponseInfo{},
			Messages: []cloudflare.ResponseInfo{},
		},
	}
}

func mockDNSRecordResponse(id string, ipNet ipnet.Type, name string, ip string) *cloudflare.DNSRecordResponse {
	return envelopDNSRecordResponse(mockDNSRecord(id, ipNet, name, ip))
}

func TestDeleteRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		listAccessCount   int
		deleteAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, _ *http.Request) {
			if listAccessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			listAccessCount--

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSListResponseFromAddr(ipnet.IP6, "test.org",
					map[string]netip.Addr{"record1": mustIP("::1")}),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records/record1", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if deleteAccessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			deleteAccessCount--

			if !assert.Equal(t, http.MethodDelete, r.Method) ||
				!assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
				!assert.Empty(t, r.URL.Query()) {
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSRecordResponse("record1", ipnet.IP6, "test.org", "::1"),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	deleteAccessCount = 1
	mockPP := mocks.NewMockPP(mockCtrl)
	ok := h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	require.True(t, ok)

	listAccessCount, deleteAccessCount = 1, 1
	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	_ = h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.True(t, cached)
	require.Empty(t, rs)
}

func TestDeleteRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record1",
		gomock.Any(),
	)
	ok := h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	require.False(t, ok)
}

func TestDeleteRecordZoneInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok := h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	require.False(t, ok)
}

//nolint:funlen
func TestUpdateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		listAccessCount   int
		updateAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if !assert.Equal(t, http.MethodGet, r.Method) {
				panic(http.ErrAbortHandler)
			}
			if listAccessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			listAccessCount--

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSListResponse(ipnet.IP6, "test.org",
					map[string]string{"record1": "::1"}),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records/record1", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if !assert.Equal(t, http.MethodPatch, r.Method) {
				panic(http.ErrAbortHandler)
			}
			if updateAccessCount <= 0 {
				panic(http.ErrAbortHandler)
			}
			updateAccessCount--

			if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
				!assert.Empty(t, r.URL.Query()) {
				panic(http.ErrAbortHandler)
			}

			var record cloudflare.DNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}

			if !assert.Equal(t, "::2", record.Content) {
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(
				mockDNSRecordResponse("record1", ipnet.IP6, "sub.test.org", "::2"),
			); !assert.NoError(t, err) {
				panic(http.ErrAbortHandler)
			}
		})

	updateAccessCount = 1
	mockPP := mocks.NewMockPP(mockCtrl)
	ok := h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::2"))
	require.True(t, ok)

	listAccessCount, updateAccessCount = 1, 1
	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	_ = h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::2"))
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, map[string]netip.Addr{"record1": mustIP("::2")}, rs)
}

func TestUpdateRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record1",
		gomock.Any(),
	)
	ok := h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::1"))
	require.False(t, ok)
}

func TestUpdateRecordInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok := h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::1"))
	require.False(t, ok)
}

//nolint:funlen
func TestCreateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		listAccessCount   int
		createAccessCount int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				if listAccessCount <= 0 {
					panic(http.ErrAbortHandler)
				}
				listAccessCount--

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(
					mockDNSListResponse(ipnet.IP6, "test.org",
						map[string]string{"record1": "::1"}),
				); !assert.NoError(t, err) {
					panic(http.ErrAbortHandler)
				}
			case http.MethodPost:
				if createAccessCount <= 0 {
					panic(http.ErrAbortHandler)
				}
				createAccessCount--

				if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) ||
					!assert.Empty(t, r.URL.Query()) {
					panic(http.ErrAbortHandler)
				}

				var record cloudflare.DNSRecord
				if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
					panic(http.ErrAbortHandler)
				}

				if !assert.Equal(t, "sub.test.org", record.Name) ||
					!assert.Equal(t, ipnet.IP6.RecordType(), record.Type) ||
					!assert.Equal(t, "::1", record.Content) ||
					!assert.Equal(t, 100, record.TTL) ||
					!assert.False(t, *record.Proxied) ||
					!assert.Equal(t, "hello", record.Comment) {
					panic(http.ErrAbortHandler)
				}
				record.ID = "record1"

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(
					envelopDNSRecordResponse(&record),
				); !assert.NoError(t, err) {
					panic(http.ErrAbortHandler)
				}
			}
		})

	createAccessCount = 1
	mockPP := mocks.NewMockPP(mockCtrl)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, mustIP("::1"), 100, false, "hello") //nolint:lll
	require.True(t, ok)
	require.Equal(t, "record1", actualID)

	listAccessCount, createAccessCount = 1, 1
	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	h.CreateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, mustIP("::1"), 100, false, "hello") //nolint:lll
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, map[string]netip.Addr{"record1": mustIP("::1")}, rs)
}

func TestCreateRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	mux, h := newHandle(t, false)

	zh := newZonesHandler(t, mux, false)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to add a new %s record of %q: %v",
		"AAAA",
		"sub.test.org",
		gomock.Any(),
	)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, mustIP("::1"), 100, false, "hello") //nolint:lll
	require.False(t, ok)
	require.Equal(t, "", actualID)
}

func TestCreateRecordInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)

	_, h := newHandle(t, false)

	mockPP := mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, mustIP("::1"), 100, false, "hello") //nolint:lll
	require.False(t, ok)
	require.Equal(t, "", actualID)
}
