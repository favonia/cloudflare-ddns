package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		ID:     string(mockID(name, i)),
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

func newZonesHandler(t *testing.T, mux *http.ServeMux, zoneStatuses map[string][]string) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc("GET /zones", func(w http.ResponseWriter, r *http.Request) {
		if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		zoneName := r.URL.Query().Get("name")
		zoneStatuses := zoneStatuses[zoneName]

		if !assert.Equal(t, url.Values{
			"name":     {zoneName},
			"per_page": {strconv.Itoa(zonePageSize)},
		}, r.URL.Query()) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(mockZonesResponse(zoneName, zoneStatuses))
		assert.NoError(t, err)
	})

	return httpHandler{requestLimit: &requestLimit}
}

func TestListZonesTwo(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zones        map[string][]string
		requestLimit int
		input        string
		ok           bool
		output       []api.ID
	}{
		"root": {
			nil,
			0,
			"",
			true,
			[]api.ID{},
		},
		"two": {
			map[string][]string{"test.org": {"active", "active"}},
			1,
			"test.org",
			true, mockIDs("test.org", 0, 1),
		},
		"empty": {
			map[string][]string{},
			1,
			"test.org",
			true,
			[]api.ID{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)
			zh := newZonesHandler(t, mux, tc.zones)

			zh.setRequestLimit(tc.requestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			output, ok := h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.True(t, zh.isExhausted())

			zh.setRequestLimit(0)
			mockPP = mocks.NewMockPP(mockCtrl)
			output, ok = h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			require.True(t, zh.isExhausted())

			if tc.requestLimit > 0 {
				h.(api.CloudflareHandle).FlushCache() //nolint:forcetypeassert

				mockPP = mocks.NewMockPP(mockCtrl)
				mockPP.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to check the existence of a zone named %q: %v",
					"test.org",
					gomock.Any(),
				)
				output, ok = h.(api.CloudflareHandle).ListZones(context.Background(), mockPP, tc.input)
				require.False(t, ok)
				require.Zero(t, output)
				require.True(t, zh.isExhausted())
			}
		})
	}
}

func TestZoneOfDomain(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zone          string
		domain        domain.Domain
		zoneStatuses  map[string][]string
		requestLimit  int
		expected      api.ID
		ok            bool
		prepareMockPP func(*mocks.MockPP)
	}{
		"root":     {"test.org", domain.FQDN("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil},     //nolint:lll
		"wildcard": {"test.org", domain.Wildcard("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil}, //nolint:lll
		"one":      {"test.org", domain.FQDN("sub.test.org"), map[string][]string{"test.org": {"active"}}, 2, mockID("test.org", 0), true, nil}, //nolint:lll
		"none": {
			"test.org", domain.FQDN("sub.test.org"),
			map[string][]string{},
			3, "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %q", "sub.test.org")
			},
		},
		"none/wildcard": {
			"test.org", domain.Wildcard("test.org"),
			map[string][]string{},
			2, "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %q", "*.test.org")
			},
		},
		"multiple": {
			"test.org", domain.FQDN("sub.test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiImpossible,
						"Found multiple active zones named %q (IDs: %s); please report this at %s",
						"test.org", pp.EnglishJoin(mockIDsAsStrings("test.org", 0, 1)), pp.IssueReportingURL,
					),
				)
			},
		},
		"multiple/wildcard": {
			"test.org", domain.Wildcard("test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			1, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(
						pp.EmojiImpossible,
						"Found multiple active zones named %q (IDs: %s); please report this at %s",
						"test.org", pp.EnglishJoin(mockIDsAsStrings("test.org", 0, 1)), pp.IssueReportingURL,
					),
				)
			},
		},
		"deleted": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"deleted"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiWarning, "Zone %q is %q and thus skipped", "test.org", "deleted"),
					m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %q", "test.org"),
				)
			},
		},
		"pending": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"pending"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiWarning, "Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected", "test.org", "pending"), //nolint:lll
				)
			},
		},
		"initializing": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"initializing"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiWarning,
						"Zone %q is %q; your Cloudflare setup is incomplete; some features might not work as expected",
						"test.org", "initializing"),
				)
			},
		},
		"undocumented": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"some-undocumented-status"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible,
					"Zone %q is in an undocumented status %q; please report this at %s",
					"test.org", "some-undocumented-status", pp.IssueReportingURL)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)
			zh := newZonesHandler(t, mux, tc.zoneStatuses)

			zh.setRequestLimit(tc.requestLimit)
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}
			zoneID, ok := h.(api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			require.True(t, zh.isExhausted())

			if tc.ok {
				zh.setRequestLimit(0)
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

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	zoneID, ok := h.(api.CloudflareHandle).ZoneOfDomain(context.Background(), mockPP, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.Zero(t, zoneID)
}

func mockDNSRecord(id string, ipNet ipnet.Type, domain string, ip string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{ //nolint:exhaustruct
		ID:      id,
		Type:    ipNet.RecordType(),
		Name:    domain,
		Content: ip,
	}
}

type formattedRecord struct {
	ID string
	IP string
}

func mockDNSListResponse(ipNet ipnet.Type, domain string, rs []formattedRecord) cloudflare.DNSListResponse {
	if len(rs) > dnsRecordPageSize {
		panic("mockDNSResponse got too many IPs")
	}

	raw := make([]cloudflare.DNSRecord, 0, len(rs))
	for _, r := range rs {
		raw = append(raw, mockDNSRecord(r.ID, ipNet, domain, r.IP))
	}

	return cloudflare.DNSListResponse{
		Result:     raw,
		ResultInfo: mockResultInfo(len(rs), dnsRecordPageSize),
		Response:   mockResponse(),
	}
}

func newListRecordsHandler(t *testing.T, mux *http.ServeMux,
	ipNet ipnet.Type, domain string, rs []formattedRecord, //nolint: unparam
) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("GET /zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !assert.Equal(t, url.Values{
				"name":     {domain},
				"page":     {"1"},
				"per_page": {strconv.Itoa(dnsRecordPageSize)},
				"type":     {ipNet.RecordType()},
			}, r.URL.Query()) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSListResponse(ipNet, domain, rs))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

//nolint:dupl
func TestListRecords(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org",
		[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}})
	lrh.setRequestLimit(1)

	expected := []api.Record{{"record1", mustIP("::1")}, {"record2", mustIP("::2")}}

	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, rs)
	require.True(t, lrh.isExhausted())

	// testing the caching
	mockPP = mocks.NewMockPP(mockCtrl)
	rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, expected, rs)
}

func TestListRecordsInvalidIPAddress(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org",
		[]formattedRecord{{"record1", "::1"}, {"record2", "not an ip"}})
	lrh.setRequestLimit(1)

	mockPP.EXPECT().Noticef(
		pp.EmojiImpossible,
		"Failed to parse the IP address in an %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		"record2",
		gomock.Any(),
	)
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)
	require.True(t, lrh.isExhausted())

	// testing the (no) caching
	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to retrieve %s records of %q: %v",
		"AAAA",
		"sub.test.org",
		gomock.Any(),
	)
	rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)
	require.True(t, lrh.isExhausted())
}

//nolint:dupl
func TestListRecordsWildcard(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "*.test.org",
		[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}})
	lrh.setRequestLimit(1)

	expected := []api.Record{{"record1", mustIP("::1")}, {"record2", mustIP("::2")}}

	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.Wildcard("test.org"))
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, expected, rs)
	require.True(t, lrh.isExhausted())

	// testing the caching
	mockPP = mocks.NewMockPP(mockCtrl)
	rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.Wildcard("test.org"))
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, expected, rs)
}

func TestListRecordsInvalidDomain(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve %s records of %q: %v", "A", "sub.test.org", gomock.Any())
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP4, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve %s records of %q: %v", "AAAA", "sub.test.org", gomock.Any()) //nolint:lll
	rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)
}

func TestListRecordsInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP4, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)

	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP4, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.False(t, cached)
	require.Zero(t, rs)
}

func envelopDNSRecordResponse(record cloudflare.DNSRecord) cloudflare.DNSRecordResponse {
	return cloudflare.DNSRecordResponse{
		Result:     record,
		ResultInfo: mockResultInfo(1, dnsRecordPageSize),
		Response:   mockResponse(),
	}
}

func mockDNSRecordResponse(id string, ipNet ipnet.Type, domain string, ip string) cloudflare.DNSRecordResponse {
	return envelopDNSRecordResponse(mockDNSRecord(id, ipNet, domain, ip))
}

func newDeleteRecordHandler(t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string,
) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("DELETE /zones/%s/dns_records/%s", mockID("test.org", 0), id),
		func(w http.ResponseWriter, r *http.Request) {
			if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !assert.Empty(t, r.URL.Query()) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSRecordResponse(id, ipNet, domain, ip))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestDeleteRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
	lrh.setRequestLimit(1)

	drh := newDeleteRecordHandler(t, mux, "record1", ipnet.IP6, "sub.test.org", "::1")
	drh.setRequestLimit(1)

	ok = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
	require.True(t, ok)
	require.True(t, drh.isExhausted())

	drh.setRequestLimit(1)
	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	_ = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.True(t, cached)
	require.Empty(t, rs)
	require.True(t, drh.isExhausted())
}

func TestDeleteRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to delete a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		api.ID("record1"),
		gomock.Any(),
	)
	ok = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
	require.False(t, ok)
}

func TestDeleteRecordZoneInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
	require.False(t, ok)
}

func newUpdateRecordHandler(t *testing.T, mux *http.ServeMux, id string, ip string) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("PATCH /zones/%s/dns_records/%s", mockID("test.org", 0), id),
		func(w http.ResponseWriter, r *http.Request) {
			if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !assert.Empty(t, r.URL.Query()) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var record cloudflare.DNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if !assert.Equal(t, ip, record.Content) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockDNSRecordResponse("record1", ipnet.IP6, "sub.test.org", "::2"))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestUpdateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
	lrh.setRequestLimit(1)

	urh := newUpdateRecordHandler(t, mux, "record1", "::2")
	urh.setRequestLimit(1)

	ok = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"))
	require.True(t, ok)
	require.True(t, urh.isExhausted())

	urh.setRequestLimit(1)
	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	_ = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"))
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{{"record1", mustIP("::2")}}, rs)
}

func TestUpdateRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to update a stale %s record of %q (ID: %s): %v",
		"AAAA",
		"sub.test.org",
		api.ID("record1"),
		gomock.Any(),
	)
	ok = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::1"))
	require.False(t, ok)
}

func TestUpdateRecordInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	ok = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::1"))
	require.False(t, ok)
}

func newCreateRecordHandler(t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string,
) httpHandler {
	t.Helper()

	var requestLimit int

	mux.HandleFunc(fmt.Sprintf("POST /zones/%s/dns_records", mockID("test.org", 0)),
		func(w http.ResponseWriter, r *http.Request) {
			if !checkRequestLimit(t, &requestLimit) || !checkToken(t, r) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !assert.Empty(t, r.URL.Query()) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var record cloudflare.DNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if !assert.Equal(t, domain, record.Name) ||
				!assert.Equal(t, ipNet.RecordType(), record.Type) ||
				!assert.Equal(t, ip, record.Content) ||
				!assert.Equal(t, 100, record.TTL) ||
				!assert.False(t, *record.Proxied) ||
				!assert.Equal(t, "hello", record.Comment) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			record.ID = id

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(envelopDNSRecordResponse(record))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestCreateRecordValid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{})
	lrh.setRequestLimit(1)

	crh := newCreateRecordHandler(t, mux, "record1", ipnet.IP6, "sub.test.org", "::1")
	crh.setRequestLimit(1)

	mockPP = mocks.NewMockPP(mockCtrl)
	h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	actualID, ok := h.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), 100, false, "hello") //nolint:lll
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), actualID)
	rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{{"record1", mustIP("::1")}}, rs)
}

func TestCreateRecordInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)
	mux, h, ok := newHandle(t, mockPP)
	require.True(t, ok)
	zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	mockPP = mocks.NewMockPP(mockCtrl)
	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to add a new %s record of %q: %v",
		"AAAA",
		"sub.test.org",
		gomock.Any(),
	)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), 100, false, "hello") //nolint:lll
	require.False(t, ok)
	require.Zero(t, actualID)
}

func TestCreateRecordInvalidZone(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %q: %v",
		"sub.test.org",
		gomock.Any(),
	)
	actualID, ok := h.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), 100, false, "hello") //nolint:lll
	require.False(t, ok)
	require.Zero(t, actualID)
}
