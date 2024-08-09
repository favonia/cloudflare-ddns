package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"testing"

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

func mockZonesResponse(zoneName string, zoneStatuses []string) cloudflare.ZonesResponse {
	numZones := len(zoneStatuses)

	if numZones > zonePageSize {
		panic("mockZonesResponse got too many zone names")
	}

	zones := make([]cloudflare.Zone, numZones)
	for i, status := range zoneStatuses {
		zones[i] = *mockZone(zoneName, i, status)
	}

	return cloudflare.ZonesResponse{
		Result:     zones,
		ResultInfo: mockResultInfo(numZones, zonePageSize),
		Response:   mockResponse(),
	}
}

func handleZones(
	t *testing.T, zoneName string, zoneStatuses []string, accountID string, w http.ResponseWriter, r *http.Request,
) {
	t.Helper()

	if !assert.Equal(t, []string{mockAuthString}, r.Header["Authorization"]) {
		panic(http.ErrAbortHandler)
	}
	if accountID == "" {
		if !assert.Equal(t, url.Values{
			"name":     {zoneName},
			"per_page": {strconv.Itoa(zonePageSize)},
		}, r.URL.Query()) {
			panic(http.ErrAbortHandler)
		}
	} else {
		if !assert.Equal(t, url.Values{
			"account.id": {mockAccountID},
			"name":       {zoneName},
			"per_page":   {strconv.Itoa(zonePageSize)},
		}, r.URL.Query()) {
			panic(http.ErrAbortHandler)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(mockZonesResponse(zoneName, zoneStatuses))
	assert.NoError(t, err)
}

func handleExceedingRequestLimit(t *testing.T, w http.ResponseWriter, _ *http.Request) {
	t.Helper()
	w.WriteHeader(http.StatusBadRequest)
}

type zonesHandler = httpHandler[map[string][]string]

func newZonesHandler(t *testing.T, mux *http.ServeMux, accountID string) zonesHandler {
	t.Helper()

	var (
		zoneStatuses map[string][]string
		requestLimit int
	)

	mux.HandleFunc("GET /zones", func(w http.ResponseWriter, r *http.Request) {
		if requestLimit <= 0 {
			handleExceedingRequestLimit(t, w, r)
			return
		}
		requestLimit--

		zoneName := r.URL.Query().Get("name")
		handleZones(t, zoneName, zoneStatuses[zoneName], accountID, w, r)
	})

	return zonesHandler{
		mux:          mux,
		params:       &zoneStatuses,
		requestLimit: &requestLimit,
	}
}

func TestListZonesTwo(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zones       map[string][]string
		numAccesses int
		input       string
		ok          bool
		output      []string
	}{
		"root": {
			nil,
			0,
			"",
			true,
			[]string{},
		},
		"two": {
			map[string][]string{"test.org": {"active", "active"}},
			1,
			"test.org",
			true,
			mockIDs("test.org", 0, 1),
		},
		"empty": {
			map[string][]string{},
			1,
			"test.org",
			true,
			[]string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mux, h, ok := newGoodHandle(t, mockPP)
			require.True(t, ok)
			zh := newZonesHandler(t, mux, mockAccountID)

			zh.set(tc.zones, tc.numAccesses)
			mockPP = mocks.NewMockPP(mockCtrl)
			output, ok := h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.True(t, zh.isExhausted())

			zh.set(nil, 0)
			mockPP = mocks.NewMockPP(mockCtrl)
			output, ok = h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.True(t, zh.isExhausted())

			if tc.numAccesses > 0 {
				h.(api.CloudflareHandle).FlushCache() //nolint:forcetypeassert

				mockPP = mocks.NewMockPP(mockCtrl)
				mockPP.EXPECT().Warningf(
					pp.EmojiError,
					"Failed to check the existence of a zone named %q: %v",
					"test.org",
					gomock.Any(),
				)
				output, ok = h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
				require.False(t, ok)
				require.Nil(t, output)
				require.True(t, zh.isExhausted())
			}
		})
	}
}

//nolint:funlen
func TestZoneOfDomain(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		accountID     string
		zone          string
		domain        domain.Domain
		zoneStatuses  map[string][]string
		requestLimit  int
		expected      string
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"root":     {mockAccountID, "test.org", domain.FQDN("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil},     //nolint:lll
		"wildcard": {mockAccountID, "test.org", domain.Wildcard("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil}, //nolint:lll
		"one":      {mockAccountID, "test.org", domain.FQDN("sub.test.org"), map[string][]string{"test.org": {"active"}}, 2, mockID("test.org", 0), true, nil}, //nolint:lll
		"none": {
			mockAccountID, "test.org", domain.FQDN("sub.test.org"),
			map[string][]string{},
			3, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "sub.test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; you can usually leave it blank unless you are updating WAF lists"), //nolint:lll
				)
			},
		},
		"none/wildcard": {
			mockAccountID, "test.org", domain.Wildcard("test.org"),
			map[string][]string{},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "*.test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; you can usually leave it blank unless you are updating WAF lists"), //nolint:lll
				)
			},
		},
		"multiple": {
			mockAccountID, "test.org", domain.FQDN("sub.test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Found multiple active zones named %q",
						"test.org",
					),
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Please report this rare situation at https://github.com/favonia/cloudflare-ddns/issues/new",
					),
				)
			},
		},
		"multiple/wildcard": {
			mockAccountID, "test.org", domain.Wildcard("test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			1, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Found multiple active zones named %q",
						"test.org",
					),
					m.EXPECT().Warningf(
						pp.EmojiImpossible,
						"Please report this rare situation at https://github.com/favonia/cloudflare-ddns/issues/new",
					),
				)
			},
		},
		"deleted": {
			mockAccountID, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"deleted"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", "test.org", "deleted"),
					m.EXPECT().Warningf(pp.EmojiError, "Failed to find the zone of %q", "test.org"),
					m.EXPECT().Infof(pp.EmojiHint, "Double-check the value of CF_ACCOUNT_ID; you can usually leave it blank unless you are updating WAF lists"), //nolint:lll
				)
			},
		},
		"deleted/empty-account": {
			"", "test.org", domain.FQDN("test.org"),
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
			mockAccountID, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"pending"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", "test.org", "pending"), //nolint:lll
				)
			},
		},
		"initializing": {
			mockAccountID, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"initializing"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", "test.org", "initializing"), //nolint:lll
				)
			},
		},
		"undocumented": {
			mockAccountID, "test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"some-undocumented-status"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Warningf(pp.EmojiImpossible, "Zone %q is in an undocumented status %q; please report this at https://github.com/favonia/cloudflare-ddns/issues/new", "test.org", "some-undocumented-status") //nolint:lll
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mux, h, ok := newHandle(t, mockPP, tc.accountID, http.StatusOK, mockVerifyToken())
			require.True(t, ok)
			zh := newZonesHandler(t, mux, tc.accountID)

			zh.set(tc.zoneStatuses, tc.requestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			zoneID, ok := h.(api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			require.True(t, zh.isExhausted())

			if tc.ok {
				zh.set(nil, 0)
				mockPP = mocks.NewMockPP(mockCtrl) // there should be no messages
				zoneID, ok = h.(api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, tc.domain)
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
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	zoneID, ok := h.(api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, domain.FQDN("sub.test.org"))
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
	if len(ips) > dnsRecordPageSize {
		panic("mockDNSResponse got too many IPs")
	}

	rs := make([]cloudflare.DNSRecord, 0, len(ips))
	for id, ip := range ips {
		rs = append(rs, *mockDNSRecord(id, ipNet, name, ip))
	}

	return &cloudflare.DNSListResponse{
		Result:     rs,
		ResultInfo: mockResultInfo(len(ips), dnsRecordPageSize),
		Response:   mockResponse(),
	}
}

func mockDNSListResponseFromAddr(ipNet ipnet.Type, name string, ips map[string]netip.Addr) *cloudflare.DNSListResponse {
	if len(ips) > dnsRecordPageSize {
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
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		ipNet        ipnet.Type
		ips          map[string]netip.Addr
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

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
	ipNet, ips, requestLimit = ipnet.IP6, expected, 1
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, ips)
	require.Equal(t, 0, requestLimit)

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
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	var (
		ipNet        ipnet.Type
		ips          map[string]netip.Addr
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

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

	ipNet, requestLimit = ipnet.IP6, 1
	mockPP.EXPECT().Warningf(
		pp.EmojiImpossible,
		"Failed to parse the IP address in an %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record2",
		gomock.Any(),
	)
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
	require.Equal(t, 0, requestLimit)

	// testing the (no) caching
	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(
		pp.EmojiError,
		"Failed to retrieve %s records of %q: %v",
		"AAAA",
		"sub.test.org",
		gomock.Any(),
	)
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
	require.Equal(t, 0, requestLimit)
}

//nolint:dupl
func TestListRecordsWildcard(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 1)

	var (
		ipNet        ipnet.Type
		ips          map[string]netip.Addr
		requestLimit int
	)

	mux.HandleFunc(fmt.Sprintf("/zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if requestLimit <= 0 {
				handleExceedingRequestLimit(t, w, r)
				return
			}
			requestLimit--

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
	ipNet, ips, requestLimit = ipnet.IP6, expected, 1
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.Wildcard("test.org"), ipnet.IP6)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, ips)
	require.Equal(t, 0, requestLimit)

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
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to retrieve %s records of %q: %v", "A", "sub.test.org", gomock.Any())
	ips, cached, ok := h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP4)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to retrieve %s records of %q: %v", "AAAA", "sub.test.org", gomock.Any()) //nolint:lll
	ips, cached, ok = h.ListRecords(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6)
	require.False(t, ok)
	require.False(t, cached)
	require.Nil(t, ips)
}

func TestListRecordsInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

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
		Result:     *record,
		ResultInfo: mockResultInfo(1, dnsRecordPageSize),
		Response:   mockResponse(),
	}
}

func mockDNSRecordResponse(id string, ipNet ipnet.Type, name string, ip string) *cloudflare.DNSRecordResponse {
	return envelopDNSRecordResponse(mockDNSRecord(id, ipNet, name, ip))
}

func TestDeleteRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
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
	ok = h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
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
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record1",
		gomock.Any(),
	)
	ok = h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	require.False(t, ok)
}

func TestDeleteRecordZoneInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok = h.DeleteRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1")
	require.False(t, ok)
}

//nolint:funlen
func TestUpdateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
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
	ok = h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::2"))
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
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
	zh.set(map[string][]string{"test.org": {"active"}}, 2)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record1",
		gomock.Any(),
	)
	ok = h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::1"))
	require.False(t, ok)
}

func TestUpdateRecordInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok = h.UpdateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, "record1", mustIP("::1"))
	require.False(t, ok)
}

//nolint:funlen
func TestCreateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, mockAccountID)
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
	mockPP := mocks.NewMockPP(mockCtrl)
	mux, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)
	zh := newZonesHandler(t, mux, mockAccountID)

	zh.set(map[string][]string{"test.org": {"active"}}, 2)
	mockPP = mocks.NewMockPP(mockCtrl)
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
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newGoodHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Warningf(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, domain.FQDN("sub.test.org"), ipnet.IP6, mustIP("::1"), 100, false, "hello") //nolint:lll
	require.False(t, ok)
	require.Equal(t, "", actualID)
}
