package api_test

// vim: nowrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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

func newDeleteRecordHandler(t *testing.T, mux *http.ServeMux, id string, ip string) httpHandler {
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
			err := json.NewEncoder(w).Encode(mockDNSRecordResponse(id, ipnet.IP6, "sub.test.org", ip))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestDeleteRecord(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

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
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm deletion of stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCloudflareHarness(t)

			zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{{ID: "record1", IP: "::1", Comment: "", Tags: nil}})
			lrh.setRequestLimit(tc.listRequestLimit)

			drh := newDeleteRecordHandler(t, f.serveMux, "record1", "::1")
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

//nolint:unparam // Keep the record ID explicit so route and response coupling stays visible to callers.
func newUpdateRecordHandler(
	t *testing.T,
	mux *http.ServeMux,
	id string,
	requestIP string,
	responseIP string,
	requestParams api.RecordParams,
	responseParams api.RecordParams,
) httpHandler {
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

			var record cloudflare.UpdateDNSRecordParams
			if err := json.NewDecoder(r.Body).Decode(&record); !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			expectedTags := requestParams.Tags
			if expectedTags == nil {
				expectedTags = []string{}
			}
			if !assert.Equal(t, "AAAA", record.Type) ||
				!assert.Equal(t, "sub.test.org", record.Name) ||
				!assert.Equal(t, requestIP, record.Content) ||
				!assert.Equal(t, requestParams.TTL.Int(), record.TTL) ||
				!assert.NotNil(t, record.Proxied) ||
				!assert.NotNil(t, record.Comment) ||
				!assert.Equal(t, expectedTags, record.Tags) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !assert.Equal(t, requestParams.Proxied, *record.Proxied) ||
				!assert.Equal(t, requestParams.Comment, *record.Comment) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			responseRecord := mockDNSRecord(id, ipnet.IP6, "sub.test.org", responseIP)
			responseRecord.TTL = responseParams.TTL.Int()
			responseRecord.Proxied = &responseParams.Proxied
			responseRecord.Comment = responseParams.Comment
			responseRecord.Tags = responseParams.Tags

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(envelopDNSRecordResponse(responseRecord))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestUpdateRecord(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

	for name, tc := range map[string]struct {
		zoneRequestLimit      int
		listRequestLimit      int
		updateRequestLimit    int
		desiredParams         api.RecordParams
		ok                    bool
		prepareMocks          func(*mocks.MockPP)
		prepareMocksForCached func(*mocks.MockPP)
	}{
		"success": {
			2, 0, 1,
			params,
			true,
			nil, nil,
		},
		"zone-fails": {
			0, 0, 0,
			params,
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
			params,
			false,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm update of stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm update of stale %s record of %s (ID: %s): %v", "AAAA", "sub.test.org", api.ID("record1"), gomock.Any())
			},
		},
		"mismatched-attributes": {
			2, 0, 1,
			api.RecordParams{
				TTL:     200,
				Proxied: true,
				Comment: "hello",
				Tags:    nil,
			},
			true,
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					"The TTL for the %s record of %s (ID: %s) is %s. However, the preferred TTL is %s. You can either change the TTL to %s in the Cloudflare dashboard at %s or change the preferred TTL with TTL=%d.",
					"AAAA", "sub.test.org", api.ID("record1"),
					"1 (auto)", "200", "200", mockDNSRecordsDeeplink(mockID("test.org", 0)), 1,
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The %s record of %s (ID: %s) is %s. However, the preferred proxy setting is %s. You can either change the proxy status to "%s" in the Cloudflare dashboard at %s or change the value of PROXIED to match the current setting.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"not proxied (DNS only)", "proxied", "proxied", mockDNSRecordsDeeplink(mockID("test.org", 0)),
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The comment for %s record of %s (ID: %s) is %s. However, the preferred comment is %s. You can either change the comment in the Cloudflare dashboard at %s or change the value of RECORD_COMMENT to match the current comment.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"empty", `"hello"`, mockDNSRecordsDeeplink(mockID("test.org", 0)),
				)
			},
			func(ppfmt *mocks.MockPP) {
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					"The TTL for the %s record of %s (ID: %s) is %s. However, the preferred TTL is %s. You can either change the TTL to %s in the Cloudflare dashboard at %s or change the preferred TTL with TTL=%d.",
					"AAAA", "sub.test.org", api.ID("record1"),
					"1 (auto)", "200", "200", mockDNSRecordsDeeplink(mockID("test.org", 0)), 1,
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The %s record of %s (ID: %s) is %s. However, the preferred proxy setting is %s. You can either change the proxy status to "%s" in the Cloudflare dashboard at %s or change the value of PROXIED to match the current setting.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"not proxied (DNS only)", "proxied", "proxied", mockDNSRecordsDeeplink(mockID("test.org", 0)),
				)
				ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
					`The comment for %s record of %s (ID: %s) is %s. However, the preferred comment is %s. You can either change the comment in the Cloudflare dashboard at %s or change the value of RECORD_COMMENT to match the current comment.`,
					"AAAA", "sub.test.org", api.ID("record1"),
					"empty", `"hello"`, mockDNSRecordsDeeplink(mockID("test.org", 0)),
				)
			},
		},
		"policy-equivalent-tags": {
			2, 0, 1,
			api.RecordParams{
				TTL:     api.TTLAuto,
				Proxied: false,
				Comment: "",
				Tags:    []string{"name:value", "X:Two"},
			},
			true,
			nil, nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCloudflareHarness(t)
			mockPP := f.newPreparedPP(tc.prepareMocks)

			zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
			zh.setRequestLimit(tc.zoneRequestLimit)

			lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{{ID: "record1", IP: "::1", Comment: "", Tags: nil}})
			lrh.setRequestLimit(tc.listRequestLimit)

			responseParams := tc.desiredParams
			if name == "mismatched-attributes" {
				responseParams = api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}
			}
			if name == "mismatched-tags" {
				responseParams = api.RecordParams{
					TTL:     tc.desiredParams.TTL,
					Proxied: tc.desiredParams.Proxied,
					Comment: tc.desiredParams.Comment,
					Tags:    []string{"team:ddns"},
				}
			}
			if name == "policy-equivalent-tags" {
				responseParams = api.RecordParams{
					TTL:     tc.desiredParams.TTL,
					Proxied: tc.desiredParams.Proxied,
					Comment: tc.desiredParams.Comment,
					Tags:    []string{"x:Two", "NAME:value", "name:value"},
				}
			}
			urh := newUpdateRecordHandler(t, f.serveMux, "record1", "::2", "::2", tc.desiredParams, responseParams)
			urh.setRequestLimit(tc.updateRequestLimit)

			ok := f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
				"record1", mustIP("::2"), tc.desiredParams)
			require.Equal(t, tc.ok, ok)
			assertHandlersExhausted(t, zh, lrh, urh)

			if ok {
				lrh.setRequestLimit(1)
				urh.setRequestLimit(1)
				mockPP = f.newPreparedPP(tc.prepareMocksForCached)
				f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				_ = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
					"record1", mustIP("::2"), tc.desiredParams)
				rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.Equal(t, tc.ok, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{"record1", mustIP("::2"), responseParams}}, rs)
				assertHandlersExhausted(t, zh, lrh, urh)
			}
		})
	}
}

func newCreateRecordHandlerWithComment(
	t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string, comment string,
) httpHandler {
	t.Helper()
	return newCreateRecordHandlerWithParams(t, mux, id, ipNet, domain, ip,
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: comment, Tags: nil},
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: comment, Tags: nil},
	)
}

func newCreateRecordHandlerWithCommentAndTags(
	t *testing.T, mux *http.ServeMux, id string, ipNet ipnet.Type, domain string, ip string, comment string, tags []string,
) httpHandler {
	t.Helper()
	return newCreateRecordHandlerWithParams(t, mux, id, ipNet, domain, ip,
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: comment, Tags: tags},
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: comment, Tags: tags},
	)
}

func newCreateRecordHandlerWithParams(
	t *testing.T,
	mux *http.ServeMux,
	id string,
	ipNet ipnet.Type,
	domain string,
	ip string,
	requestParams api.RecordParams,
	responseParams api.RecordParams,
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
				!assert.Equal(t, requestParams.TTL.Int(), record.TTL) ||
				!assert.NotNil(t, record.Proxied) ||
				!assert.Equal(t, requestParams.Comment, record.Comment) ||
				!assert.Equal(t, requestParams.Tags, record.Tags) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !assert.Equal(t, requestParams.Proxied, *record.Proxied) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			record.ID = id
			record.TTL = responseParams.TTL.Int()
			record.Proxied = &responseParams.Proxied
			record.Comment = responseParams.Comment
			record.Tags = responseParams.Tags

			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(envelopDNSRecordResponse(record))
			assert.NoError(t, err)
		})

	return httpHandler{requestLimit: &requestLimit}
}

func TestCreateRecord(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

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
				ppfmt.EXPECT().Noticef(pp.EmojiError, "Could not confirm creation of new %s record of %s: %v", "AAAA", "sub.test.org", gomock.Any())
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

			crh := newCreateRecordHandlerWithParams(
				t,
				f.serveMux,
				"record1",
				ipnet.IP6,
				"sub.test.org",
				"::1",
				api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
				api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
			)
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

func TestCreateRecordWithTags(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
		Tags:    []string{"team:ddns", "Env:Prod"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", nil)
	lrh.setRequestLimit(0)
	crh := newCreateRecordHandlerWithCommentAndTags(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", "managed",
		[]string{"team:ddns", "Env:Prod"})
	crh.setRequestLimit(1)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordWarnsOnlyForNewUndocumentedResponseTags(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
		Tags:    []string{"team:ddns"},
	}
	responseParams := api.RecordParams{
		TTL:     params.TTL,
		Proxied: params.Proxied,
		Comment: params.Comment,
		Tags:    []string{"team:ddns", "featureflag", ":prod"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPreparedPP(func(ppfmt *mocks.MockPP) {
		ppfmt.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found tags %s in an %s record of %s (ID: %s) that are not in Cloudflare's documented name:value form; this should not happen and please report this at %s",
			`"featureflag" and ":prod"`,
			"AAAA",
			"sub.test.org",
			api.ID("record1"),
			pp.IssueReportingURL,
		)
	})
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", nil)
	lrh.setRequestLimit(0)
	crh := newCreateRecordHandlerWithParams(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", params, responseParams)
	crh.setRequestLimit(1)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordDoesNotWarnForRequestOwnedUndocumentedTags(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
		Tags:    []string{"featureflag", "team:ddns"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()
	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", nil)
	lrh.setRequestLimit(0)
	crh := newCreateRecordHandlerWithParams(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", params, params)
	crh.setRequestLimit(1)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordManagedCacheSkipsUnmanagedComment(t *testing.T) {
	t.Parallel()

	managedRecordsCommentRegex := regexp.MustCompile("^managed$")
	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "unmanaged", Tags: nil}

	f := newCloudflareHarnessWithOptions(t, api.HandleOptions{
		CacheExpiration:                   defaultHandleOptions().CacheExpiration,
		ManagedRecordsCommentRegex:        managedRecordsCommentRegex,
		ManagedWAFListItemsCommentRegex:   nil,
		AllowWholeWAFListDeleteOnShutdown: true,
	})
	mockPP := f.newPP()

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{})
	lrh.setRequestLimit(1)

	crh := newCreateRecordHandlerWithComment(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", "unmanaged")
	crh.setRequestLimit(1)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.False(t, cached)
	require.Empty(t, rs)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.True(t, cached)
	require.Empty(t, rs)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordManagedCachePrependsCreatedRecord(t *testing.T) {
	t.Parallel()

	listParams := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "managed",
		Tags:    nil,
	}
	createParams := api.RecordParams{
		TTL:     300,
		Proxied: true,
		Comment: "managed",
		Tags:    []string{"team:ddns"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{
		{ID: "record2", IP: "::3", Comment: "managed", Tags: []string{"env:prod"}},
	})
	lrh.setRequestLimit(1)

	crh := newCreateRecordHandlerWithParams(
		t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", createParams, createParams,
	)
	crh.setRequestLimit(1)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), listParams)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record2", IP: mustIP("::3"), RecordParams: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "managed",
			Tags:    []string{"env:prod"},
		}},
	}, rs)

	id, ok := f.handle.CreateRecord(
		context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), createParams,
	)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), listParams)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record1", IP: mustIP("::1"), RecordParams: createParams},
		{ID: "record2", IP: mustIP("::3"), RecordParams: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "managed",
			Tags:    []string{"env:prod"},
		}},
	}, rs)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordManagedCacheUsesDesiredMetadataEvenIfCreateResponseDiffers(t *testing.T) {
	t.Parallel()

	requestParams := api.RecordParams{
		TTL:     300,
		Proxied: true,
		Comment: "managed",
		Tags:    []string{"Team:Alpha", "env:prod"},
	}
	responseParams := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "",
		Tags:    []string{"env:prod", "team:alpha"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{})
	lrh.setRequestLimit(1)

	crh := newCreateRecordHandlerWithParams(
		t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", requestParams, responseParams,
	)
	crh.setRequestLimit(1)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), requestParams)
	require.True(t, ok)
	require.False(t, cached)
	require.Empty(t, rs)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), requestParams)
	require.True(t, ok)
	require.Equal(t, api.ID("record1"), id)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), requestParams)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record1", IP: mustIP("::1"), RecordParams: requestParams},
	}, rs)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestCreateRecordFailureInvalidatesManagedCache(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

	f := newCloudflareHarness(t)
	mockPP := f.newPreparedPP(func(ppfmt *mocks.MockPP) {
		ppfmt.EXPECT().Noticef(
			pp.EmojiError,
			"Could not confirm creation of new %s record of %s: %v",
			"AAAA", "sub.test.org", gomock.Any(),
		)
	})

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{
		{ID: "record1", IP: "::2", Comment: "", Tags: nil},
	})
	lrh.setRequestLimit(2)

	crh := newCreateRecordHandlerWithParams(
		t,
		f.serveMux,
		"record2",
		ipnet.IP6,
		"sub.test.org",
		"::1",
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
	)
	crh.setRequestLimit(0)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{{ID: "record1", IP: mustIP("::2"), RecordParams: params}}, rs)

	id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
	require.False(t, ok)
	require.Zero(t, id)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{{ID: "record1", IP: mustIP("::2"), RecordParams: params}}, rs)
	assertHandlersExhausted(t, zh, lrh, crh)
}

func TestUpdateRecordManagedCacheDropsNowUnmanagedRecord(t *testing.T) {
	t.Parallel()

	managedRecordsCommentRegex := regexp.MustCompile("^managed$")
	managedParams := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "managed", Tags: nil}

	f := newCloudflareHarnessWithOptions(t, api.HandleOptions{
		CacheExpiration:                   defaultHandleOptions().CacheExpiration,
		ManagedRecordsCommentRegex:        managedRecordsCommentRegex,
		ManagedWAFListItemsCommentRegex:   nil,
		AllowWholeWAFListDeleteOnShutdown: true,
	})
	mockPP := f.newPreparedPP(func(ppfmt *mocks.MockPP) {
		ppfmt.EXPECT().Noticef(pp.EmojiUserWarning,
			`The comment for %s record of %s (ID: %s) is %s. However, the preferred comment is %s. You can either change the comment in the Cloudflare dashboard at %s or change the value of RECORD_COMMENT to match the current comment.`,
			"AAAA", "sub.test.org", api.ID("record1"),
			`"unmanaged"`, `"managed"`, mockDNSRecordsDeeplink(mockID("test.org", 0)),
		)
	})

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{
		{ID: "record1", IP: "::1", Comment: "managed", Tags: nil},
	})
	lrh.setRequestLimit(1)

	urh := newUpdateRecordHandler(
		t,
		f.serveMux,
		"record1",
		"::2",
		"::2",
		managedParams,
		api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "unmanaged", Tags: nil},
	)
	urh.setRequestLimit(1)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), managedParams)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record1", IP: mustIP("::1"), RecordParams: managedParams},
	}, rs)

	ok = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
		"record1", mustIP("::2"), managedParams)
	require.True(t, ok)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), managedParams)
	require.True(t, ok)
	require.True(t, cached)
	require.Empty(t, rs)
	assertHandlersExhausted(t, zh, lrh, urh)
}

func TestUpdateRecordWarnsOnlyForNewUndocumentedResponseTags(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "",
		Tags:    []string{"team:ddns"},
	}
	responseParams := api.RecordParams{
		TTL:     params.TTL,
		Proxied: params.Proxied,
		Comment: params.Comment,
		Tags:    []string{"team:ddns", "featureflag"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPreparedPP(func(ppfmt *mocks.MockPP) {
		ppfmt.EXPECT().Noticef(
			pp.EmojiImpossible,
			"Found tags %s in an %s record of %s (ID: %s) that are not in Cloudflare's documented name:value form; this should not happen and please report this at %s",
			`"featureflag"`,
			"AAAA",
			"sub.test.org",
			api.ID("record1"),
			pp.IssueReportingURL,
		)
	})

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	urh := newUpdateRecordHandler(t, f.serveMux, "record1", "::2", "::2", params, responseParams)
	urh.setRequestLimit(1)

	ok := f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"), params)
	require.True(t, ok)
	assertHandlersExhausted(t, zh, urh)
}

func TestUpdateRecordDoesNotWarnForRequestOwnedUndocumentedTags(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{
		TTL:     api.TTLAuto,
		Proxied: false,
		Comment: "",
		Tags:    []string{"featureflag", "team:ddns"},
	}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)
	urh := newUpdateRecordHandler(t, f.serveMux, "record1", "::2", "::2", params, params)
	urh.setRequestLimit(1)

	ok := f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::2"), params)
	require.True(t, ok)
	assertHandlersExhausted(t, zh, urh)
}

func TestUpdateRecordManagedCachePrependsMissingRecord(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

	f := newCloudflareHarness(t)
	mockPP := f.newPP()

	zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
	zh.setRequestLimit(2)

	lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{
		{ID: "record2", IP: "::3", Comment: "", Tags: nil},
	})
	lrh.setRequestLimit(1)

	urh := newUpdateRecordHandler(
		t,
		f.serveMux,
		"record1",
		"::2",
		"::2",
		params,
		params,
	)
	urh.setRequestLimit(1)

	rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.False(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record2", IP: mustIP("::3"), RecordParams: params},
	}, rs)

	ok = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
		"record1", mustIP("::2"), params)
	require.True(t, ok)

	rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
	require.True(t, ok)
	require.True(t, cached)
	require.Equal(t, []api.Record{
		{ID: "record1", IP: mustIP("::2"), RecordParams: params},
		{ID: "record2", IP: mustIP("::3"), RecordParams: params},
	}, rs)
	assertHandlersExhausted(t, zh, lrh, urh)
}

func TestRecordWriteSequenceAfterCachedList(t *testing.T) {
	t.Parallel()

	params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}

	t.Run("update+delete", func(t *testing.T) {
		t.Parallel()
		f := newCloudflareHarness(t)
		mockPP := f.newPP()

		zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
		zh.setRequestLimit(2)

		lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org",
			[]formattedRecord{{ID: "record1", IP: "::1", Comment: "", Tags: nil}, {ID: "record2", IP: "::3", Comment: "", Tags: nil}})
		lrh.setRequestLimit(1)

		urh := newUpdateRecordHandler(
			t,
			f.serveMux,
			"record1",
			"::2",
			"::2",
			params,
			params,
		)
		urh.setRequestLimit(1)

		drh := newDeleteRecordHandler(t, f.serveMux, "record2", "::3")
		drh.setRequestLimit(1)

		rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.False(t, cached)
		require.Equal(t, []api.Record{
			{"record1", mustIP("::1"), params},
			{"record2", mustIP("::3"), params},
		}, rs)

		ok = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
			"record1", mustIP("::2"), params)
		require.True(t, ok)

		ok = f.handle.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
			"record2", api.RegularDelitionMode)
		require.True(t, ok)

		rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.True(t, cached)
		require.Equal(t, []api.Record{{"record1", mustIP("::2"), params}}, rs)
		assertHandlersExhausted(t, zh, lrh, urh, drh)
	})

	t.Run("create+delete", func(t *testing.T) {
		t.Parallel()
		f := newCloudflareHarness(t)
		mockPP := f.newPP()

		zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
		zh.setRequestLimit(2)

		lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{})
		lrh.setRequestLimit(1)

		crh := newCreateRecordHandlerWithParams(
			t,
			f.serveMux,
			"record1",
			ipnet.IP6,
			"sub.test.org",
			"::1",
			api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
			api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
		)
		crh.setRequestLimit(1)

		drh := newDeleteRecordHandler(t, f.serveMux, "record1", "::1")
		drh.setRequestLimit(1)

		rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.False(t, cached)
		require.Empty(t, rs)

		id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
		require.True(t, ok)
		require.Equal(t, api.ID("record1"), id)

		ok = f.handle.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
			"record1", api.RegularDelitionMode)
		require.True(t, ok)

		rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.True(t, cached)
		require.Empty(t, rs)
		assertHandlersExhausted(t, zh, lrh, crh, drh)
	})

	t.Run("mixed/update+create+delete", func(t *testing.T) {
		t.Parallel()
		f := newCloudflareHarness(t)
		mockPP := f.newPP()

		zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
		zh.setRequestLimit(2)

		lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org",
			[]formattedRecord{{ID: "record1", IP: "::1", Comment: "", Tags: nil}, {ID: "record2", IP: "::3", Comment: "", Tags: nil}})
		lrh.setRequestLimit(1)

		urh := newUpdateRecordHandler(
			t,
			f.serveMux,
			"record1",
			"::2",
			"::2",
			params,
			params,
		)
		urh.setRequestLimit(1)

		crh := newCreateRecordHandlerWithParams(
			t,
			f.serveMux,
			"record3",
			ipnet.IP6,
			"sub.test.org",
			"::4",
			api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
			api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil},
		)
		crh.setRequestLimit(1)

		drh := newDeleteRecordHandler(t, f.serveMux, "record2", "::3")
		drh.setRequestLimit(1)

		rs, cached, ok := f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.False(t, cached)
		require.Equal(t, []api.Record{
			{"record1", mustIP("::1"), params},
			{"record2", mustIP("::3"), params},
		}, rs)

		ok = f.handle.UpdateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
			"record1", mustIP("::2"), params)
		require.True(t, ok)

		id, ok := f.handle.CreateRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::4"), params)
		require.True(t, ok)
		require.Equal(t, api.ID("record3"), id)

		ok = f.handle.DeleteRecord(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"),
			"record2", api.RegularDelitionMode)
		require.True(t, ok)

		rs, cached, ok = f.handle.ListRecords(context.Background(), mockPP, ipnet.IP6, domain.FQDN("sub.test.org"), params)
		require.True(t, ok)
		require.True(t, cached)
		require.Equal(t, []api.Record{
			{"record3", mustIP("::4"), params},
			{"record1", mustIP("::2"), params},
		}, rs)
		assertHandlersExhausted(t, zh, lrh, urh, crh, drh)
	})
}
