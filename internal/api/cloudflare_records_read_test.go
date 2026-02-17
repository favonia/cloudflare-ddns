package api_test

// vim: nowrap

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

func mockDNSRecord(id string, ipNet ipnet.Type, domain string, ip string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{ //nolint:exhaustruct
		ID:      id,
		Type:    ipNet.RecordType(),
		Name:    domain,
		Content: ip,
		TTL:     1,
		Comment: "",
		Proxied: nil,
	}
}

type formattedRecord struct {
	ID string
	IP string
}

func mockDNSListResponse(ipNet ipnet.Type, domain string, rs []formattedRecord) cloudflare.DNSListResponse {
	// Pagination is intentionally delegated to cloudflare-go (ListDNSRecords).
	// These tests mock a single page only to focus on this package's logic.
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
				_, err := w.Write([]byte(`{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"messages":[],"result":null}`))
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

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: ""}

	for name, tc := range map[string]struct {
		zones            map[string][]string
		zoneRequestLimit int
		recordDomain     string
		records          []formattedRecord
		listRequestLimit int
		input            domain.Domain
		expectedParams   api.RecordParams
		expected         []api.Record
		ok               bool
		prepareMocks     func(*mocks.MockPP)
	}{
		"success": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}},
			1,
			domain.FQDN("sub.test.org"), params,
			[]api.Record{{"record1", mustIP("::1"), params}, {"record2", mustIP("::2"), params}},
			true,
			nil,
		},
		"success/wildcard": {
			map[string][]string{"test.org": {"active"}},
			1,
			"*.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "::2"}},
			1,
			domain.Wildcard("test.org"), params,
			[]api.Record{{"record1", mustIP("::1"), params}, {"record2", mustIP("::2"), params}},
			true,
			nil,
		},
		"list-fail": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org", nil, 0,
			domain.FQDN("sub.test.org"), params,
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to retrieve %s records of %s: %v", "AAAA", "sub.test.org", gomock.Any())
				ppfmt.EXPECT().NoticeOncef(pp.MessageRecordPermission, pp.EmojiHint, `Double check your API token. Make sure you granted the "Edit" permission of "Zone - DNS"`)
			},
		},
		"no-zone": {
			nil, 0,
			"sub.test.org", nil, 0,
			domain.FQDN("sub.test.org"), params,
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any())
			},
		},
		"invalid-ip": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org",
			[]formattedRecord{{"record1", "::1"}, {"record2", "not an ip"}},
			1,
			domain.FQDN("sub.test.org"), params,
			nil,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiImpossible, "Failed to parse the IP address in an %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record2"), gomock.Any())
			},
		},
		"mismatched-attributes": {
			map[string][]string{"test.org": {"active"}},
			2,
			"sub.test.org",
			[]formattedRecord{{"record1", "::1"}},
			1,
			domain.FQDN("sub.test.org"),
			api.RecordParams{
				TTL:     100,
				Proxied: true,
				Comment: "hello",
			},
			[]api.Record{{"record1", mustIP("::1"), params}},
			true,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					"The TTL for the %s record of %s (ID: %s) is %s. However, it is expected to be %s. You can either change the TTL to %s in the Cloudflare dashboard at https://dash.cloudflare.com or change the expected TTL with TTL=%d.",
					"AAAA", "sub.test.org", api.ID("record1"),
					"1 (auto)", "100", "100", 1,
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The %s record of %s (ID: %s) is %s. However, it is %sexpected to be proxied. You can either change the proxy status to "%s" in the Cloudflare dashboard at https://dash.cloudflare.com or change the value of PROXIED to match the current setting.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"not proxied (DNS only)", "", "proxied",
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The comment for %s record of %s (ID: %s) is %s. However, it is expected to be %s. You can either change the comment in the Cloudflare dashboard at https://dash.cloudflare.com or change the value of RECORD_COMMENT to match the current comment.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"empty", `"hello"`,
				)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newCloudflareFixture(t)

			zh := newZonesHandler(t, f.serveMux, tc.zones)
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, tc.recordDomain, tc.records)
			lrh.setRequestLimit(tc.listRequestLimit)

			rs, cached, ok := f.handle.ListRecords(context.Background(), f.newPreparedPP(tc.prepareMocks), ipnet.IP6, tc.input, tc.expectedParams)
			require.Equal(t, tc.ok, ok)
			require.False(t, cached)
			require.Equal(t, tc.expected, rs)
			assertHandlersExhausted(t, zh, lrh)
		})
	}
}

func TestListRecordsCache(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: ""}

	f := newCloudflareFixture(t)
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}, {"record2", "::2"}})

	zh.setRequestLimit(2)
	lrh.setRequestLimit(1)
	rs, cached, ok := f.handle.ListRecords(context.Background(), f.newPP(), ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{{"record1", mustIP("::1"), params}, {"record2", mustIP("::2"), params}}, rs)
	assertHandlersExhausted(t, zh, lrh)

	zh.setRequestLimit(0)
	lrh.setRequestLimit(0)
	rs, cached, ok = f.handle.ListRecords(context.Background(), f.newPP(), ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{{"record1", mustIP("::1"), params}, {"record2", mustIP("::2"), params}}, rs)
	assertHandlersExhausted(t, zh, lrh)
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
