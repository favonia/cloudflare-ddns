package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
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

			f := newCloudflareFixture(t)
			zh := newZonesHandler(t, f.serveMux, tc.zones)

			zh.setRequestLimit(tc.requestLimit)
			output, ok := f.cfHandle.ListZones(context.Background(), f.newPP(), tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.output, output)
			assertHandlersExhausted(t, zh)

			if tc.requestLimit > 0 {
				f.cfHandle.FlushCache()

				mockPP := f.newPP()
				mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "test.org", gomock.Any())
				output, ok = f.cfHandle.ListZones(context.Background(), mockPP, tc.input)
				require.False(t, ok)
				require.Zero(t, output)
				assertHandlersExhausted(t, zh)
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
		"root":     {"test.org", domain.FQDN("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil},
		"wildcard": {"test.org", domain.Wildcard("test.org"), map[string][]string{"test.org": {"active"}}, 1, mockID("test.org", 0), true, nil},
		"one":      {"test.org", domain.FQDN("sub.test.org"), map[string][]string{"test.org": {"active"}}, 2, mockID("test.org", 0), true, nil},
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
					m.EXPECT().Noticef(pp.EmojiImpossible, "Found multiple active zones named %s (IDs: %s); please report this at %s", "test.org", pp.EnglishJoin(mockIDsAsStrings("test.org", 0, 1)), pp.IssueReportingURL),
				)
			},
		},
		"multiple/wildcard": {
			"test.org", domain.Wildcard("test.org"),
			map[string][]string{"test.org": {"active", "active"}},
			1, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiImpossible, "Found multiple active zones named %s (IDs: %s); please report this at %s", "test.org", pp.EnglishJoin(mockIDsAsStrings("test.org", 0, 1)), pp.IssueReportingURL),
				)
			},
		},
		"deleted": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"deleted"}},
			2, "", false,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Infof(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account and thus skipped", "test.org", "deleted"),
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
					m.EXPECT().Noticef(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected", "test.org", "pending"),
				)
			},
		},
		"initializing": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"initializing"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Noticef(pp.EmojiWarning, "DNS zone %s is %q in your Cloudflare account; some features (e.g., proxying) might not work as expected", "test.org", "initializing"),
				)
			},
		},
		"undocumented": {
			"test.org", domain.FQDN("test.org"),
			map[string][]string{"test.org": {"some-undocumented-status"}},
			1, mockID("test.org", 0), true,
			func(m *mocks.MockPP) {
				m.EXPECT().Noticef(pp.EmojiImpossible, "DNS zone %s is in an undocumented status %q in your Cloudflare account; please report this at %s", "test.org", "some-undocumented-status", pp.IssueReportingURL)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareFixture(t)

			zh := newZonesHandler(t, f.serveMux, tc.zoneStatuses)
			zh.setRequestLimit(tc.requestLimit)

			zoneID, ok := f.cfHandle.ZoneIDOfDomain(context.Background(), f.newPreparedPP(tc.prepareMockPP), tc.domain)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, zoneID)
			assertHandlersExhausted(t, zh)
		})
	}
}

func TestListZonesTwoCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareFixture(t)
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active", "active"}})

	zh.setRequestLimit(1)
	output, ok := f.cfHandle.ListZones(context.Background(), f.newPP(), "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), output)
	assertHandlersExhausted(t, zh)

	zh.setRequestLimit(0)
	output, ok = f.cfHandle.ListZones(context.Background(), f.newPP(), "test.org")
	require.True(t, ok)
	require.Equal(t, mockIDs("test.org", 0, 1), output)
	assertHandlersExhausted(t, zh)
}

func TestZoneIDOfDomainCache(t *testing.T) {
	t.Parallel()

	f := newCloudflareFixture(t)
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})

	zh.setRequestLimit(2)
	zoneID, ok := f.cfHandle.ZoneIDOfDomain(context.Background(), f.newPP(), domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
	assertHandlersExhausted(t, zh)

	zh.setRequestLimit(0)
	zoneID, ok = f.cfHandle.ZoneIDOfDomain(context.Background(), f.newPP(), domain.FQDN("sub.test.org"))
	require.True(t, ok)
	require.Equal(t, mockID("test.org", 0), zoneID)
	assertHandlersExhausted(t, zh)
}

func TestZoneIDOfDomainInvalid(t *testing.T) {
	t.Parallel()

	f := newCloudflareFixture(t)
	mockPP := f.newPP()

	mockPP.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any())
	zoneID, ok := f.cfHandle.ZoneIDOfDomain(context.Background(), mockPP, domain.FQDN("sub.test.org"))
	require.False(t, ok)
	require.Zero(t, zoneID)
}
