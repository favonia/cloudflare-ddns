package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func newDeleteRecordHandler(t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string) httpHandler {
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

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: ""}

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
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any())
			},
		},
		"delete-fails": {
			2, 0, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to delete a stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCloudflareHarness(t)

			zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
			lrh.setRequestLimit(tc.listRequestLimit)

			drh := newDeleteRecordHandler(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1")
			drh.setRequestLimit(tc.deleteRequestLimit)

			ok := f.handle.DeleteRecord(context.Background(), f.newPreparedPP(tc.prepareMocks), ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, zh, lrh, drh)

			if ok {
				lrh.setRequestLimit(1)
				drh.setRequestLimit(1)
				mockPP := f.newPreparedPP(tc.prepareMocks)
				f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				_ = f.handle.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", false)
				rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.Empty(t, rs)
				assertHandlersExhausted(t, zh, lrh, drh)
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

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: ""}

	for name, tc := range map[string]struct {
		zoneRequestLimit      int
		listRequestLimit      int
		updateRequestLimit    int
		currentParams         api.RecordParams
		expectedParams        api.RecordParams
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
		prepareMocksForCached func(*mocks.MockPP)
	}{
		"success": {
			2, 0, 1,
			params, params,
			true,
			nil, nil,
		},
		"zone-fails": {
			0, 0, 0,
			params, params,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any())
			},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any())
			},
		},
		"update-fails": {
			2, 0, 0,
			params, params,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to update a stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to update a stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
		},
		"mismatched-attributes": {
			2, 0, 1,
			api.RecordParams{
				TTL:     300,
				Proxied: true,
				Comment: "aloha",
			},
			api.RecordParams{
				TTL:     200,
				Proxied: true,
				Comment: "hello",
			},
			true,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					"The TTL for the %s record of %s (ID: %s) is %s. However, it is expected to be %s. You can either change the TTL to %s in the Cloudflare dashboard at https://dash.cloudflare.com or change the expected TTL with TTL=%d.",
					"AAAA", "sub.test.org", api.ID("record1"),
					"1 (auto)", "200", "200", 1,
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
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCloudflareHarness(t)
			mockPP := f.newPreparedPP(tc.prepareMocks)

			zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{{"record1", "::1"}})
			lrh.setRequestLimit(tc.listRequestLimit)

			urh := newUpdateRecordHandler(t, f.serveMux, "record1", "::2")
			urh.setRequestLimit(tc.updateRequestLimit)

			ok := f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
				"record1", mustIP("::2"), tc.currentParams, tc.expectedParams)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, zh, lrh, urh)

			if ok {
				lrh.setRequestLimit(1)
				urh.setRequestLimit(1)
				mockPP = f.newPreparedPP(tc.prepareMocksForCached)
				f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				_ = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
					"record1", mustIP("::2"), params, tc.expectedParams)
				rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{"record1", mustIP("::2"), params}}, rs)
				assertHandlersExhausted(t, zh, lrh, urh)
			}
		})
	}
}

func newCreateRecordHandler(t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string) httpHandler {
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
				!assert.Equal(t, 1, record.TTL) ||
				!assert.False(t, *record.Proxied) ||
				!assert.Empty(t, record.Comment) {
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

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: ""}

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
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to check the existence of a zone named %s: %v", "sub.test.org", gomock.Any()).Times(2)
			},
		},
		"create-fails": {
			2, 1, 0,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Failed to add a new %s record of %s: %v", "AAAA", "sub.test.org", gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCloudflareHarness(t)
			mockPP := f.newPreparedPP(tc.prepareMocks)

			zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{})
			lrh.setRequestLimit(tc.listRequestLimit)

			crh := newCreateRecordHandler(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1")
			crh.setRequestLimit(tc.createRequestLimit)

			f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
			actualID, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, api.ID("record1"), actualID)
				rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.True(t, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{"record1", mustIP("::1"), params}}, rs)
			} else {
				require.Zero(t, actualID)
			}
		})
	}
}
