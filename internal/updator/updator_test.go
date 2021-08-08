package updator_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/quiet"
	"github.com/favonia/cloudflare-ddns/internal/updator"
)

type eventType int

const (
	eventList eventType = iota
	eventDelete
	eventUpdate
	eventCreate
)

type interaction struct {
	event     eventType
	arguments []interface{}
	values    []interface{}
}

type mockHandle struct {
	t      *testing.T
	script []interaction
}

func (m *mockHandle) Call(event eventType, numValues int, arguments ...interface{}) []interface{} {
	require.Greater(m.t, len(m.script), 0)

	require.Equal(m.t, m.script[0].event, event)
	require.Equal(m.t, len(m.script[0].values), numValues)
	require.Equal(m.t, m.script[0].arguments, arguments)

	values := m.script[0].values
	m.script = m.script[1:]

	return values
}

func (m *mockHandle) IsExhausted() bool {
	return len(m.script) == 0
}

func (m *mockHandle) ListRecords(_ context.Context, _ pp.Indent,
	domain api.FQDN, ipNet ipnet.Type) (map[string]net.IP, bool) {
	values := m.Call(eventList, 2, domain, ipNet)

	val0, _ := values[0].(map[string]net.IP)
	val1, _ := values[1].(bool)

	return val0, val1
}

func (m *mockHandle) DeleteRecord(_ context.Context, _ pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, id string) bool {
	values := m.Call(eventDelete, 1, domain, ipNet, id)
	val0, _ := values[0].(bool)
	return val0
}

func (m *mockHandle) UpdateRecord(ctx context.Context, indent pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, id string, ip net.IP) bool {
	values := m.Call(eventUpdate, 1, domain, ipNet, id, ip)
	val0, _ := values[0].(bool)
	return val0
}

func (m *mockHandle) CreateRecord(ctx context.Context, indent pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, ip net.IP, ttl int, proxied bool) (string, bool) {
	values := m.Call(eventCreate, 2, domain, ipNet, ip, ttl, proxied)
	val0, _ := values[0].(string)
	val1, _ := values[1].(bool)
	return val0, val1
}

func (m *mockHandle) FlushCache() {
	require.FailNow(m.t, "updator should never call FlushCache directly")
}

//nolint: funlen
func TestDo(t *testing.T) {
	t.Parallel()

	const (
		quiet     = quiet.VERBOSE
		domain    = api.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		record3   = "record3"
		ttl       = 100
		proxied   = true
	)
	var (
		ip1 = net.ParseIP("::1")
		ip2 = net.ParseIP("::2")
	)

	for name, tc := range map[string]struct {
		ip      net.IP
		ttl     api.TTL
		proxied bool
		script  []interaction
		ok      bool
	}{
		"0-nil": {
			ip: nil,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{}, true},
				},
			},
			ok: true,
		},
		"0": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{}, true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, ttl, proxied},
					values:    []interface{}{record1, true},
				},
			},
			ok: true,
		},
		"1unmatched": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{true},
				},
			},
			ok: true,
		},
		"1unmatched-updatefail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, ttl, proxied},
					values:    []interface{}{record2, true},
				},
			},
			ok: true,
		},
		"1unmatched-nil": {
			ip: nil,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip1}, true},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{true},
				},
			},
			ok: true,
		},
		"1matched": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip1}, true},
				},
			},
			ok: true,
		},
		"2matched": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip1, record2: ip1}, true},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{true},
				},
			},
			ok: true,
		},
		"2matched-deletefail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip1, record2: ip1}, true},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{false},
				},
			},
			ok: true,
		},
		"2unmatched": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2, record2: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{true},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{true},
				},
			},
			ok: true,
		},
		"2unmatched-updatefail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2, record2: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record2, ip1},
					values:    []interface{}{true},
				},
			},
			ok: true,
		},
		"2unmatched-updatefailtwice": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2, record2: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record2, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, ttl, proxied},
					values:    []interface{}{record3, true},
				},
			},
			ok: true,
		},
		"2unmatched-updatefail-deletefail-updatefail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2, record2: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{false},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record2, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, ttl, proxied},
					values:    []interface{}{record3, true},
				},
			},
			ok: false,
		},
		"2unmatched-updatefailtwice-createfail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{record1: ip2, record2: ip2}, true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record1, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record1},
					values:    []interface{}{true},
				},
				{
					event:     eventUpdate,
					arguments: []interface{}{domain, ipNetwork, record2, ip1},
					values:    []interface{}{false},
				},
				{
					event:     eventDelete,
					arguments: []interface{}{domain, ipNetwork, record2},
					values:    []interface{}{true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, ttl, proxied},
					values:    []interface{}{record3, false},
				},
			},
			ok: false,
		},
		"listfail": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{nil, false},
				},
			},
			ok: false,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := &mockHandle{t: t, script: tc.script}
			ok := updator.Do(context.Background(), 3,
				quiet,
				&updator.Args{
					Handle:    m,
					IPNetwork: ipNetwork,
					IP:        tc.ip,
					Domain:    domain,
					TTL:       ttl,
					Proxied:   proxied,
				})
			require.Equal(t, tc.ok, ok)
			require.True(t, m.IsExhausted())
		})
	}
}
