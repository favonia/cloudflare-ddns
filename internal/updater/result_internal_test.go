package updater

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/config"
	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/heartbeat"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
	"github.com/favonia/cloudflare-ddns/internal/provider/protocol"
	"github.com/favonia/cloudflare-ddns/internal/setter"
)

func semanticConfig() *config.UpdateConfig {
	return &config.UpdateConfig{ //nolint:exhaustruct_v5 // These tests supply only fields read by the exercised updater paths.
		Domains:          map[ipnet.Family][]domain.Domain{},
		Provider:         map[ipnet.Family]provider.Provider{},
		DetectionFilter:  map[ipnet.Family]ipfilter.Filter{ipnet.IP4: ipfilter.KeepAll(), ipnet.IP6: ipfilter.KeepAll()},
		DefaultPrefixLen: map[ipnet.Family]int{ipnet.IP4: 32, ipnet.IP6: 64},
		DetectionTimeout: time.Second,
		UpdateTimeout:    time.Second,
		TTL:              api.TTLAuto,
	}
}

func semanticParams() api.RecordParams {
	return api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "", Tags: nil}
}

// semanticDNSResult keeps mock setter progress and reached effects coherent.
// Tests supply operation facts; this helper only builds their API projection.
func semanticDNSResult(code setter.ResponseCode, before *setter.DNSSnapshot, satisfied []netip.Addr, operations []setter.DNSOperation) setter.DNSResult {
	var o dnsOutcome
	o.result = setter.DNSResult{Code: code, Before: before, Progress: nil, Effects: nil}
	if before != nil {
		o.result.Progress = &setter.DNSProgress{Satisfied: satisfied, Operations: operations}
	}
	reportingSyncEffects(&o)
	return o.result
}

// These package-internal assertions exercise the retained evidence contract used
// by notification inference. Public heartbeat text cannot expose per-scope facts.
func TestMessageRetainsScopeEvidence(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	a, b, unchanged := domain.FQDN("a.example"), domain.FQDN("b.example"), domain.FQDN("unchanged.example")
	c.Domains[ipnet.IP4] = []domain.Domain{a, unchanged, b}
	ips := []netip.Addr{netip.MustParseAddr("192.0.2.9")}
	otherIPs := []netip.Addr{netip.MustParseAddr("192.0.2.10")}
	beforeA := &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{{ID: "a", IP: netip.MustParseAddr("192.0.2.1")}}}
	beforeB := &setter.DNSSnapshot{Cached: false, Records: []api.RecordFact{{ID: "b", IP: netip.MustParseAddr("192.0.2.2")}}}
	resultA := semanticDNSResult(setter.ResponseUpdated, beforeA, nil, []setter.DNSOperation{{Kind: api.RecordUpdate, Status: setter.DNSOperationCompleted, Record: api.RecordFact{ID: "a", IP: ips[0]}, Membership: api.MembershipManaged}})
	resultB := semanticDNSResult(setter.ResponseUpdated, beforeB, nil, []setter.DNSOperation{{Kind: api.RecordUpdate, Status: setter.DNSOperationCompleted, Record: api.RecordFact{ID: "b", IP: ips[0]}, Membership: api.MembershipManaged}})
	resultUnchanged := semanticDNSResult(setter.ResponseNoop, &setter.DNSSnapshot{Cached: false, Records: []api.RecordFact{{ID: "unchanged", IP: otherIPs[0]}}}, otherIPs, nil)
	gomock.InOrder(
		s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, a, ips, semanticParams()).Return(resultA),
		s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, unchanged, otherIPs, semanticParams()).Return(resultUnchanged),
		s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, b, ips, semanticParams()).Return(resultB),
	)
	msg := setIPs(context.Background(), pp.NewSilent(), c, s, ipnet.IP4, dnsTargetsByDomain{a: ips, unchanged: otherIPs, b: ips})
	require.Equal(t, []dnsOutcome{
		{filtered: false, scope: dnsScope{domain: a, family: ipnet.IP4}, cleanup: false, target: ips, result: resultA},
		{filtered: false, scope: dnsScope{domain: unchanged, family: ipnet.IP4}, cleanup: false, target: otherIPs, result: resultUnchanged},
		{filtered: false, scope: dnsScope{domain: b, family: ipnet.IP4}, cleanup: false, target: ips, result: resultB},
	}, msg.dns)
	require.Equal(t, heartbeat.Message{OK: true, Lines: []string{"Set A records for a.example, b.example to 192.0.2.9"}}, msg.HeartbeatMessage)
	require.False(t, msg.Failed())
	require.Empty(t, msg.problems)
}

func TestMessageOwnsTargetsAndBefore(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	d := domain.FQDN("a.example")
	c.Domains[ipnet.IP4] = []domain.Domain{d}
	ip := netip.MustParseAddr("192.0.2.9")
	ips := []netip.Addr{ip}
	oldIP := netip.MustParseAddr("192.0.2.1")
	before := &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{{ID: "old", IP: oldIP}}}
	effect := api.RecordMutation{Status: api.MutationUnconfirmed, Kind: api.RecordUpdate, Record: api.RecordFact{ID: "old", IP: ip}, Membership: api.MembershipManaged}
	result := semanticDNSResult(setter.ResponseFailed, before, nil, []setter.DNSOperation{{Kind: api.RecordUpdate, Status: setter.DNSOperationUnconfirmed, Record: effect.Record, Membership: effect.Membership}})
	effects := result.Effects
	s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, d, ips, semanticParams()).Return(result)
	msg := setIPs(context.Background(), pp.NewSilent(), c, s, ipnet.IP4, sharedDNSTargets(c.Domains[ipnet.IP4], ips))
	require.Len(t, msg.dns, 1)
	ips[0] = netip.MustParseAddr("192.0.2.99")
	before.Cached = false
	before.Records[0].ID = "mutated"
	effects[0].Status = api.MutationCompleted
	require.Equal(t, []netip.Addr{ip}, msg.dns[0].target)
	require.Equal(t, &setter.DNSSnapshot{Cached: true, Records: []api.RecordFact{{ID: "old", IP: oldIP}}}, msg.dns[0].result.Before)
	require.Equal(t, []api.RecordMutation{effect}, msg.dns[0].result.Effects)
	require.True(t, msg.Failed())

	merged := mergeMessages(msg)
	msg.dns[0].target[0] = netip.Addr{}
	msg.dns[0].result.Before.Records[0].ID = "changed-again"
	msg.dns[0].result.Effects[0].Status = api.MutationCompleted
	require.Equal(t, []netip.Addr{ip}, merged.dns[0].target)
	require.Equal(t, api.ID("old"), merged.dns[0].result.Before.Records[0].ID)
	require.Equal(t, []api.RecordMutation{effect}, merged.dns[0].result.Effects)
}

func TestMessagePrerequisiteProblems(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"detection", "filtering", "derivation", "missing-target"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			s := mocks.NewMockSetter(ctrl) // No setter operation is authorized by any prerequisite failure.
			c := semanticConfig()
			c.WAFLists = []api.WAFList{{AccountID: "account", Name: "must-not-invoke"}}
			d := domain.FQDN("a.example")
			var msg Message
			var want operationProblem
			switch scenario {
			case "detection":
				p := mocks.NewMockProvider(ctrl)
				p.EXPECT().GetRawData(gomock.Any(), gomock.Any(), ipnet.IP4, 32).Return(provider.NewUnavailableDetectionResult())
				c.Provider[ipnet.IP4] = p
				c.Domains[ipnet.IP4] = []domain.Domain{d}
				msg = UpdateIPs(context.Background(), pp.NewSilent(), c, s)
				want = operationProblem{kind: problemDetection, family: ipnet.IP4, domains: nil}
			case "filtering":
				filter, ok := ipfilter.Parse(pp.NewSilent(), "TEST", ipnet.IP4, "addr-in(203.0.113.0/24)")
				require.True(t, ok)
				c.DetectionFilter[ipnet.IP4] = filter
				c.Provider[ipnet.IP4] = protocol.Static{ProviderName: "test", RawEntries: []ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("192.0.2.1"), 32)}}
				c.Domains[ipnet.IP4] = []domain.Domain{d}
				msg = UpdateIPs(context.Background(), pp.NewSilent(), c, s)
				want = operationProblem{kind: problemFiltering, family: ipnet.IP4, domains: nil}
			case "derivation":
				c.Provider[ipnet.IP6] = protocol.Static{ProviderName: "test", RawEntries: []ipnet.RawEntry{ipnet.RawEntryFrom(netip.MustParseAddr("2001:db8::1"), 128)}}
				derivation, err := hostid6.Literal(netip.MustParseAddr("::2"))
				require.NoError(t, err)
				c.HostID6 = map[domain.Domain]hostid6.Set{d: hostid6.NewSet(derivation), domain.FQDN("b.example"): hostid6.DefaultSet()}
				c.Domains[ipnet.IP6] = []domain.Domain{d, domain.FQDN("b.example")}
				msg = UpdateIPs(context.Background(), pp.NewSilent(), c, s)
				want = operationProblem{kind: problemDerivation, family: ipnet.IP6, domains: []domain.Domain{d, domain.FQDN("b.example")}}
			case "missing-target":
				c.Domains[ipnet.IP4] = []domain.Domain{d}
				msg = setIPs(context.Background(), pp.NewSilent(), c, s, ipnet.IP4, nil)
				want = operationProblem{kind: problemMissingTarget, family: ipnet.IP4, domains: []domain.Domain{d}}
			}
			require.Equal(t, []operationProblem{want}, msg.problems)
			require.Empty(t, msg.dns)
			require.Empty(t, msg.waf)
			require.True(t, msg.Failed())
			for family := range c.Domains {
				c.Domains[family][0] = domain.FQDN("mutated.example")
			}
			require.Equal(t, []operationProblem{want}, msg.problems)
			merged := mergeMessages(msg)
			if len(msg.problems[0].domains) > 0 {
				msg.problems[0].domains[0] = domain.FQDN("changed-again.example")
			}
			require.Equal(t, []operationProblem{want}, merged.problems)
		})
	}
}

func TestMessageExplicitEmptyAndUnmanagedFamily(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	d := domain.FQDN("a.example")
	c.Domains[ipnet.IP4] = []domain.Domain{d}
	c.Domains[ipnet.IP6] = []domain.Domain{domain.FQDN("unmanaged.example")}
	c.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	result := semanticDNSResult(setter.ResponseNoop, &setter.DNSSnapshot{Cached: false, Records: nil}, nil, nil)
	s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, d, gomock.Len(0), semanticParams()).Return(result)
	msg := UpdateIPs(context.Background(), pp.NewSilent(), c, s)
	require.Len(t, msg.dns, 1)
	require.Equal(t, dnsScope{domain: d, family: ipnet.IP4}, msg.dns[0].scope)
	require.Empty(t, msg.dns[0].target)
	require.NotNil(t, msg.dns[0].result.Before)
	require.Empty(t, msg.problems)
	require.Equal(t, heartbeat.Message{OK: true, Lines: nil}, msg.HeartbeatMessage)
	require.False(t, msg.cleanup)
}

func TestMessageRetainsCleanupAndWAFOutcomes(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	d := domain.FQDN("a.example")
	c.Domains[ipnet.IP4] = []domain.Domain{d}
	c.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	list := api.WAFList{AccountID: "account", Name: "list"}
	c.WAFLists = []api.WAFList{list}
	result := semanticDNSResult(setter.ResponseNoop, &setter.DNSSnapshot{Cached: true, Records: nil}, nil, nil)
	wafResult := setter.WAFResult{Before: nil, Batches: nil, Cleanup: &api.WAFCleanupResult{Code: api.WAFListCleanupFailed, Before: nil, Stages: nil}, Code: setter.ResponseFailed}
	gomock.InOrder(
		s.EXPECT().FinalDelete(gomock.Any(), gomock.Any(), ipnet.IP4, d, semanticParams()).Return(result),
		s.EXPECT().FinalClearWAFList(gomock.Any(), gomock.Any(), list, "", map[ipnet.Family]bool{ipnet.IP4: true}, api.CleanupWait).Return(wafResult),
	)
	msg := FinalDeleteIPs(context.Background(), pp.NewSilent(), c, s, api.CleanupWait)
	require.Equal(t, []dnsOutcome{{filtered: false, scope: dnsScope{domain: d, family: ipnet.IP4}, cleanup: true, target: nil, result: result}}, msg.dns)
	require.Equal(t, []wafOutcome{{list: list, cleanup: true, result: wafResult, filtered: true, scope: wafScope{ipnet.IP4: true}}}, msg.waf)
	require.True(t, msg.cleanup)
	require.True(t, msg.Failed())

	c = semanticConfig()
	msg = FinalDeleteIPs(context.Background(), pp.NewSilent(), c, s, api.CleanupWait)
	require.True(t, msg.cleanup)
	require.Empty(t, msg.dns)
	require.Empty(t, msg.waf)
	require.False(t, msg.Failed())
}

func TestMessageRetainsWAFNoop(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	list := api.WAFList{AccountID: "account", Name: "list"}
	c.WAFLists = []api.WAFList{list}
	c.Provider[ipnet.IP4] = provider.NewStaticEmpty()
	result := setter.WAFResult{Code: setter.ResponseNoop, Before: &api.WAFSnapshot{Exists: false, Cached: false, Items: nil}, Batches: nil, Cleanup: nil}
	s.EXPECT().SetWAFList(gomock.Any(), gomock.Any(), list, "", map[ipnet.Family]setter.WAFTargets{ipnet.IP4: setter.NewAvailableWAFTargets([]netip.Prefix{})}, "").Return(result)
	msg := UpdateIPs(context.Background(), pp.NewSilent(), c, s)
	require.Equal(t, []wafOutcome{{list: list, cleanup: false, result: result, filtered: true, scope: wafScope{ipnet.IP4: true}}}, msg.waf)
	require.False(t, msg.Failed())
	require.Empty(t, msg.HeartbeatMessage.Lines)
}

func TestMessageOwnsProgress(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	s := mocks.NewMockSetter(ctrl)
	c := semanticConfig()
	d := domain.FQDN("a.example")
	ip := netip.MustParseAddr("192.0.2.1")
	c.Domains[ipnet.IP4] = []domain.Domain{d}
	oldIP := netip.MustParseAddr("192.0.2.2")
	operation := reportingOperation(api.RecordDelete, setter.DNSOperationUnconfirmed, "record", oldIP.String())
	// Keep the matching target and attempt to remove a different outdated address.
	before := &setter.DNSSnapshot{Cached: false, Records: []api.RecordFact{{ID: "kept", IP: ip}, {ID: "record", IP: oldIP}}}
	result := semanticDNSResult(setter.ResponseFailed, before, []netip.Addr{ip}, []setter.DNSOperation{operation})
	progress := result.Progress
	s.EXPECT().SetIPs(gomock.Any(), gomock.Any(), ipnet.IP4, d, []netip.Addr{ip}, semanticParams()).Return(result)
	msg := setIPs(context.Background(), pp.NewSilent(), c, s, ipnet.IP4, sharedDNSTargets(c.Domains[ipnet.IP4], []netip.Addr{ip}))
	progress.Satisfied[0] = netip.Addr{}
	progress.Operations[0].Status = setter.DNSOperationCompleted
	require.Equal(t, []netip.Addr{ip}, msg.dns[0].result.Progress.Satisfied)
	require.Equal(t, []setter.DNSOperation{operation}, msg.dns[0].result.Progress.Operations)
	merged := mergeMessages(msg)
	msg.dns[0].result.Progress.Satisfied[0] = netip.Addr{}
	msg.dns[0].result.Progress.Operations[0].Record.ID = "changed"
	require.Equal(t, []netip.Addr{ip}, merged.dns[0].result.Progress.Satisfied)
	require.Equal(t, []setter.DNSOperation{operation}, merged.dns[0].result.Progress.Operations)
}
