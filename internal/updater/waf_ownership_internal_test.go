package updater

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func TestWAFEvidenceOwnsEveryMutableLayer(t *testing.T) {
	t.Parallel()
	item := api.WAFListItem{ID: "original", Prefix: netip.MustParsePrefix("192.0.2.0/24"), Comment: "owned"}
	snapshot := &api.WAFSnapshot{Exists: true, Cached: true, Items: []api.WAFListItem{item}}
	batchResult := &api.WAFBatchResult{PrerequisiteMutation: true, OK: true, Status: api.WAFBatchCompleted, Returned: snapshot, ReturnedItems: []api.WAFListItem{item}}
	source := wafOutcome{filtered: false, list: api.WAFList{AccountID: "", Name: ""}, code: setter.ResponseNoop, cleanup: false, configured: map[ipnet.Family]bool{ipnet.IP4: true}, active: map[ipnet.Family]bool{ipnet.IP4: true}, result: setter.WAFResult{Code: 0, Before: snapshot, Batches: []setter.WAFBatch{{Create: []api.WAFListCreateItem{{Prefix: item.Prefix, Comment: item.Comment}}, Delete: []api.WAFListItem{item}, Result: batchResult}}, Cleanup: &api.WAFCleanupResult{Code: 0, Before: snapshot, Stages: []api.WAFCleanupStage{{WholeList: false, Items: []api.WAFListItem{item}, Result: *batchResult}}}}}
	mutations := map[string]func(*wafOutcome){
		"before":           func(o *wafOutcome) { o.result.Before.Items[0].ID = "changed" },
		"create":           func(o *wafOutcome) { o.result.Batches[0].Create[0].Comment = "changed" },
		"delete":           func(o *wafOutcome) { o.result.Batches[0].Delete[0].ID = "changed" },
		"prerequisite":     func(o *wafOutcome) { o.result.Batches[0].Result.PrerequisiteMutation = false },
		"result":           func(o *wafOutcome) { o.result.Batches[0].Result.OK = false },
		"returned":         func(o *wafOutcome) { o.result.Batches[0].Result.Returned.Items[0].ID = "changed" },
		"membership":       func(o *wafOutcome) { o.result.Batches[0].Result.ReturnedItems[0].ID = "changed" },
		"cleanup":          func(o *wafOutcome) { o.result.Cleanup.Before.Items[0].ID = "changed" },
		"stage":            func(o *wafOutcome) { o.result.Cleanup.Stages[0].Items[0].ID = "changed" },
		"stage returned":   func(o *wafOutcome) { o.result.Cleanup.Stages[0].Result.Returned.Items[0].ID = "changed" },
		"stage membership": func(o *wafOutcome) { o.result.Cleanup.Stages[0].Result.ReturnedItems[0].ID = "changed" },
		"configured":       func(o *wafOutcome) { o.configured[ipnet.IP4] = false },
		"active":           func(o *wafOutcome) { o.active[ipnet.IP4] = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			input := source.clone()
			message := newMessage()
			message.waf = []wafOutcome{input}
			merged := mergeMessages(message)
			assessment := NewReportingState(false).Assess(message)
			mutate(&input)
			require.Equal(t, source, merged.waf[0])
			require.Equal(t, source, assessment.waf[0])
		})
	}
}

func TestWAFConfiguredOwnershipSurvivesUnavailableFamily(t *testing.T) {
	t.Parallel()
	c := semanticConfig()
	c.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	c.Provider[ipnet.IP6] = provider.NewStaticEmpty()
	list := api.WAFList{AccountID: "account", Name: "list"}
	c.WAFLists = []api.WAFList{list}
	targets := map[ipnet.Family]setter.WAFTargets{ipnet.IP4: setter.NewAvailableWAFTargets(nil), ipnet.IP6: setter.NewUnavailableWAFTargets()}
	result := setter.WAFResult{Code: 0, Batches: nil, Cleanup: nil, Before: &api.WAFSnapshot{Cached: false, Exists: true, Items: []api.WAFListItem{{Comment: "", ID: "six", Prefix: netip.MustParsePrefix("2001:db8::/32")}}}}
	mock := mocks.NewMockSetter(gomock.NewController(t))
	mock.EXPECT().SetWAFList(gomock.Any(), gomock.Any(), list, "", targets, "").Return(result)
	message := setWAFLists(context.Background(), pp.NewSilent(), c, mock, targets)
	result.Before.Items[0].ID = "mutated"
	c.Provider[ipnet.IP6] = nil
	require.False(t, message.waf[0].filtered)
	require.True(t, message.waf[0].configured[ipnet.IP6])
	require.False(t, message.waf[0].active[ipnet.IP6])
	require.Equal(t, api.ID("six"), message.waf[0].result.Before.Items[0].ID)
	require.Empty(t, message.waf[0].content().unchanged)
}
