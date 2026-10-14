package api_test

import (
	"context"
	"regexp"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestRecordMutationPrerequisiteFailure(t *testing.T) {
	t.Parallel()
	for _, kind := range []api.RecordMutationKind{api.RecordCreate, api.RecordUpdate, api.RecordDelete} {
		t.Run(map[api.RecordMutationKind]string{api.RecordCreate: "create", api.RecordUpdate: "update", api.RecordDelete: "delete"}[kind], func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				f := newCloudflareHarness(t)
				p := f.newPP()
				zh := newZonesHandler(t, f.serveMux, map[string][]string{})
				zh.setRequestLimit(0)
				p.EXPECT().Noticef(pp.EmojiError, "Failed to check if a zone named %s exists: %v", "sub.test.org", gomock.Any())
				var result api.RecordMutation
				switch kind {
				case api.RecordCreate:
					result = f.handle.CreateRecord(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), api.RecordParams{TTL: api.TTL(0), Proxied: false, Comment: "", Tags: nil})
				case api.RecordUpdate:
					result = f.handle.UpdateRecord(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", mustIP("::1"), api.RecordParams{TTL: api.TTL(0), Proxied: false, Comment: "", Tags: nil})
				case api.RecordDelete:
					result = f.handle.DeleteRecord(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), "record1", api.RegularDeletionMode)
				}
				require.Equal(t, api.MutationNoEffect, result.Status)
				require.Equal(t, kind, result.Kind)
				require.Equal(t, api.MembershipUnknown, result.Membership)
				if kind == api.RecordCreate {
					require.Empty(t, result.Record.ID)
				} else {
					require.Equal(t, api.ID("record1"), result.Record.ID)
				}
			})
		})
	}
}

func TestRecordMutationReturnedMembership(t *testing.T) {
	t.Parallel()
	for _, comment := range []string{"managed", "unmanaged"} {
		t.Run(comment, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				options := defaultHandleOptions()
				options.ManagedRecordsCommentRegex = regexp.MustCompile("^managed$")
				f := newCloudflareHarnessWithOptions(t, options)
				p := f.newPP()
				params := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "managed", Tags: nil}
				zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
				zh.setRequestLimit(2)
				lrh := newListRecordsHandler(t, f.serveMux, ipnet.IP6, "sub.test.org", []formattedRecord{})
				lrh.setRequestLimit(1)
				responseParams := params
				responseParams.Comment = comment
				crh := newCreateRecordHandlerWithParams(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", params, responseParams)
				crh.setRequestLimit(1)
				_, _, ok := f.handle.ListRecords(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.True(t, ok)
				result := f.handle.CreateRecord(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), params)
				require.Equal(t, api.MutationCompleted, result.Status)
				require.Equal(t, api.RecordFact{ID: "record1", IP: mustIP("::1")}, result.Record)
				membership := api.MembershipManaged
				if comment == "unmanaged" {
					membership = api.MembershipUnmanaged
				}
				require.Equal(t, membership, result.Membership)
				rs, cached, ok := f.handle.ListRecords(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), params)
				require.True(t, ok)
				require.True(t, cached)
				require.Equal(t, []api.Record{{ID: "record1", IP: mustIP("::1"), RecordParams: params}}, rs)
				assertHandlersExhausted(t, zh, lrh, crh)
			})
		})
	}
}

func TestRecordMutationUnconfirmedMembership(t *testing.T) {
	t.Parallel()
	for _, selector := range []*regexp.Regexp{nil, regexp.MustCompile(""), regexp.MustCompile("^managed$")} {
		name := "nil"
		if selector != nil {
			name = selector.String()
			if name == "" {
				name = "empty"
			}
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				options := defaultHandleOptions()
				options.ManagedRecordsCommentRegex = selector
				f := newCloudflareHarnessWithOptions(t, options)
				p := f.newPP()
				zh := newZonesHandler(t, f.serveMux, map[string][]string{"test.org": {"active"}})
				zh.setRequestLimit(2)
				crh := newCreateRecordHandlerWithComment(t, f.serveMux, "record1", ipnet.IP6, "sub.test.org", "::1", "managed")
				crh.setRequestLimit(0)
				p.EXPECT().Noticef(pp.EmojiError, "Could not confirm creation of new %s record for %s: %v", "AAAA", "sub.test.org", gomock.Any())
				result := f.handle.CreateRecord(context.Background(), p, ipnet.IP6, domain.FQDN("sub.test.org"), mustIP("::1"), api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "managed", Tags: nil})
				require.Equal(t, api.MutationUnconfirmed, result.Status)
				require.Empty(t, result.Record.ID)
				expected := api.MembershipManaged
				if selector != nil && selector.String() != "" {
					expected = api.MembershipUnknown
				}
				require.Equal(t, expected, result.Membership)
				assertHandlersExhausted(t, zh, crh)
			})
		})
	}
}
