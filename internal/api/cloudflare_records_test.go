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
		"root": {nil, 0, "", true, []api.ID{}},
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
					"Failed to check the existence of a zone named %s: %v",
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

func TestZoneIDOfDomain(t *testing.T) {
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
				m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %s", "sub.test.org")
			},
		},
		"none/wildcard": {
			"test.org", domain.Wildcard("test.org"),
			map[string][]string{},
			2, "", false,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %s", "*.test.org")
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
						"Found multiple active zones named %s (IDs: %s); please report this at %s",
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
						"Found multiple active zones named %s (IDs: %s); please report this at %s",
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
					m.EXPECT().Infof(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account and thus skipped", "test.org", "deleted"), //nolint:lll
					m.EXPECT().Noticef(pp.EmojiError, "Failed to find the zone of %s", "test.org"),
				)
			},
		},
		"pending": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"pending"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected", "test.org", "pending"), //nolint:lll
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
						"DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected",
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
					"DNS zone %s is in an undocumented status %q in your Cloudflare account; please report this at %s",
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
			zoneID, ok := h.(api.CloudflareHandle).ZoneIDOfDomain(context.Background(), mockPP, tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			require.True(t, zh.isExhausted())

			if tc.ok {
				zh.setRequestLimit(0)
				mockPP = mocks.NewMockPP(mockCtrl) // there should be no messages
				zoneID, ok = h.(api.CloudflareHandle).ZoneIDOfDomain(context.Background(), mockPP, tc.domain)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.expected, zoneID)
				require.True(t, zh.isExhausted())
			}
		})
	}
}

func TestZoneIDOfDomainInvalid(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockPP := mocks.NewMockPP(mockCtrl)

	_, h, ok := newHandle(t, mockPP)
	require.True(t, ok)

	mockPP.EXPECT().Noticef(
		pp.EmojiError,
		"Failed to check the existence of a zone named %s: %v",
		"sub.test.org",
		gomock.Any(),
	)
	zoneID, ok := h.(api.CloudflareHandle).ZoneIDOfDomain(context.Background(), mockPP, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.Zero(t, zoneID)
}

func mockDNSRecord(id string, ipNet ipnet.Type, domain string, ip string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{ //nolint:exhaustruct
		ID:      id,
		Type:    ipNet.RecordType(),
		Name:    domain,
		Content: ip,
		TTL:     100,
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
				_, err := w.Write([]byte(`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[],"result":null}`)) //nolint:lll
				assert.NoError(t, err)
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

func TestListRecords(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zones                 map[string][]string
		zoneRequestLimit      int
		recordDomain          string
		records               []formattedRecord
		listRequestLimit      int
		input                 domain.Domain
		expected              []api.Record
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
		cached                bool
		prepareMocksForCached func(*mocks.MockPP)
	}{
		"success": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}},
			1,
			domain.FQDN("sub.test.org"),
			[]api.Record{{"record1", mustIP("::1")}, {"record2", mustIP("::2")}},
			true,
			nil, true, nil,
		},
		"success/wildcard": {
			map[string][]string{"test.org": {"active"}},
			1,
			"*.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}},
			1,
			domain.Wildcard("test.org"),
			[]api.Record{{"record1", mustIP("::1")}, {"record2", mustIP("::2")}},
			true,
			nil, true, nil,
		},
		"list-fail": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org", nil, 0,
			domain.FQDN("sub.test.org"),
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to retrieve %s records of %s: %v", "AAAA", "sub.test.org", gomock.Any(),
				)
				ppfmt.EXPECT().Hintf(pp.HintRecordPermission,
					"Double check your API token. "+
						`Make sure you granted the "Edit" permission of "Zone - DNS"`)
			},
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to retrieve %s records of %s: %v", "AAAA", "sub.test.org", gomock.Any(),
				)
				ppfmt.EXPECT().Hintf(pp.HintRecordPermission,
					"Double check your API token. "+
						`Make sure you granted the "Edit" permission of "Zone - DNS"`)
			},
		},
		"no-zone": {
			nil, 0,
			"sub.test.org", nil, 0,
			domain.FQDN("sub.test.org"),
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to check the existence of a zone named %s: %v",
					"sub.test.org", gomock.Any(),
				)
			},
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to check the existence of a zone named %s: %v",
					"sub.test.org", gomock.Any(),
				)
			},
		},
		"invalid-ip": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "not an ip"}},
			1,
			domain.FQDN("sub.test.org"),
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiImpossible,
					"Failed to parse the IP address in an %s record of %s (ID: %s): %v",
					"AAAA", "sub.test.org", "record2", gomock.Any(),
				)
			},
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(
					pp.EmojiError,
					"Failed to retrieve %s records of %s: %v",
					"AAAA", "sub.test.org", gomock.Any(),
				)
				ppfmt.EXPECT().Hintf(pp.HintRecordPermission,
					"Double check your API token. "+
						`Make sure you granted the "Edit" permission of "Zone - DNS"`)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			zh := newZonesHandler(t, mux, tc.zones)
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, mux, ipnet.IP6, tc.recordDomain, tc.records)
			lrh.setRequestLimit(tc.listRequestLimit)

			rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, tc.input)
			require.Equal(t, tc.ok, ok)
			require.False(t, cached)
			require.Equal(t, tc.expected, rs)
			require.True(t, zh.isExhausted())
			require.True(t, lrh.isExhausted())

			// testing the caching
			mockPP = mocks.NewMockPP(mockCtrl)
			if tc.prepareMocksForCached != nil {
				tc.prepareMocksForCached(mockPP)
			}
			rs, cached, ok = h.ListRecords(context.Background(), mockPP, ipnet.IP6, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.cached, cached)
			require.Equal(t, tc.expected, rs)
		})
	}
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

func TestDeleteRecord(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zoneRequestLimit   int
		listRequestLimit   int
		deleteRequestLimit int
		ok                 bool
		prepareMocks       func(*mocks.MockPP)
	}{
		"success": {
			2, 0, 1,
			true,
			nil,
		},
		"zone-fails": {
			0, 0, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to check the existence of a zone named %s: %v",
					"sub.test.org", gomock.Any(),
				)
			},
		},
		"delete-fails": {
			2, 0, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to delete a stale %s record of %s (ID: %s): %v",
					"AAAA", "sub.test.org", api.ID("record1"), gomock.Any(),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
			lrh.setRequestLimit(tc.listRequestLimit)

			drh := newDeleteRecordHandler(t, mux, "record1", ipnet.IP6, "sub.test.org", "::1")
			drh.setRequestLimit(tc.deleteRequestLimit)

			ok = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
			require.Equal(t, tc.ok, ok)
			require.True(t, zh.isExhausted())
			require.True(t, lrh.isExhausted())
			require.True(t, drh.isExhausted())

			if ok {
				lrh.setRequestLimit(1)
				drh.setRequestLimit(1)
				mockPP = mocks.NewMockPP(mockCtrl)
				if tc.prepareMocks != nil {
					tc.prepareMocks(mockPP)
				}
				h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
				_ = h.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
				rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.Empty(t, rs)
				require.True(t, zh.isExhausted())
				require.True(t, lrh.isExhausted())
				require.True(t, drh.isExhausted())
			}
		})
	}
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

func TestUpdateRecord(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zoneRequestLimit   int
		listRequestLimit   int
		updateRequestLimit int
		expectedTTL        api.TTL
		expectedProxied    bool
		expectedComment    string
		ok                 bool
		prepareMocks       func(*mocks.MockPP)
	}{
		"success": {
			2, 0, 1,
			100, false, "",
			true,
			nil,
		},
		"zone-fails": {
			0, 0, 0,
			100, false, "",
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to check the existence of a zone named %s: %v",
					"sub.test.org", gomock.Any(),
				)
			},
		},
		"update-fails": {
			2, 0, 0,
			100, false, "",
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to update a stale %s record of %s (ID: %s): %v",
					"AAAA", "sub.test.org", api.ID("record1"), gomock.Any(),
				)
			},
		},
		"mismatch-attribute": {
			2, 0, 1,
			1, true, "hello",
			true,
			func(ppfmt *mocks.MockPP) {
				const hintText = "The updater will not overwrite proxy statuses, TTLs, or record comments; " +
					"you can change them in your Cloudflare dashboard at https://dash.cloudflare.com"

				gomock.InOrder(
					ppfmt.EXPECT().Infof(pp.EmojiUserWarning,
						"The TTL of the %s record of %s (ID: %s) to be updated differs from the value of TTL (%s) and will be kept", //nolint:lll
						"AAAA", "sub.test.org", api.ID("record1"), "1 (auto)",
					),
					ppfmt.EXPECT().Hintf(pp.HintMismatchedRecordAttributes, hintText),
					ppfmt.EXPECT().Infof(pp.EmojiUserWarning,
						"The proxy status of the %s record of %s (ID: %s) to be updated differs from the value of PROXIED (%v for this domain) and will be kept", //nolint:lll
						"AAAA", "sub.test.org", api.ID("record1"), true,
					),
					ppfmt.EXPECT().Hintf(pp.HintMismatchedRecordAttributes, hintText),
					ppfmt.EXPECT().Infof(pp.EmojiUserWarning,
						"The comment of the %s record of %s (ID: %s) to be updated differs from the value of RECORD_COMMENT (%q) and will be kept", //nolint:lll
						"AAAA", "sub.test.org", api.ID("record1"), "hello",
					),
					ppfmt.EXPECT().Hintf(pp.HintMismatchedRecordAttributes, hintText),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
			lrh.setRequestLimit(tc.listRequestLimit)

			urh := newUpdateRecordHandler(t, mux, "record1", "::2")
			urh.setRequestLimit(tc.updateRequestLimit)

			ok = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"),
				tc.expectedTTL, tc.expectedProxied, tc.expectedComment)
			require.Equal(t, tc.ok, ok)
			require.True(t, zh.isExhausted())
			require.True(t, lrh.isExhausted())
			require.True(t, urh.isExhausted())

			if ok {
				lrh.setRequestLimit(1)
				urh.setRequestLimit(1)
				mockPP = mocks.NewMockPP(mockCtrl)
				if tc.prepareMocks != nil {
					tc.prepareMocks(mockPP)
				}
				h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
				_ = h.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"),
					tc.expectedTTL, tc.expectedProxied, tc.expectedComment)
				rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{"record1", mustIP("::2")}}, rs)
				require.True(t, zh.isExhausted())
				require.True(t, lrh.isExhausted())
				require.True(t, urh.isExhausted())
			}
		})
	}
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

func TestCreateRecord(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		zoneRequestLimit   int
		listRequestLimit   int
		createRequestLimit int
		ok                 bool
		prepareMocks       func(*mocks.MockPP)
	}{
		"success": {
			2, 1, 1,
			true,
			nil,
		},
		"zone-fails": {
			0, 0, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to check the existence of a zone named %s: %v",
					"sub.test.org", gomock.Any(),
				).Times(2)
			},
		},
		"create-fails": {
			2, 1, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError,
					"Failed to add a new %s record of %s: %v",
					"AAAA", "sub.test.org", gomock.Any(),
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMocks != nil {
				tc.prepareMocks(mockPP)
			}

			mux, h, ok := newHandle(t, mockPP)
			require.True(t, ok)

			zh := newZonesHandler(t, mux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, mux, ipnet.IP6, "sub.test.org", []formattedRecord{})
			lrh.setRequestLimit(tc.listRequestLimit)

			crh := newCreateRecordHandler(t, mux, "record1", ipnet.IP6, "sub.test.org", "::1")
			crh.setRequestLimit(tc.createRequestLimit)

			h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
			actualID, ok := h.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), 100, false, "hello") //nolint:lll
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, api.ID("record1"), actualID)
				rs, cached, ok := h.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"))
				require.True(t, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{"record1", mustIP("::1")}}, rs)
			} else {
				require.Zero(t, actualID)
			}
		})
	}
}
