package updator_test

import (
	"context"
	"net"
	"testing"

	//"github.com/stretchr/testify/assert"
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
	eventFlush
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
	if len(m.script) == 0 {
		require.Fail(m.t, "scrcipt was exhausted")
	}

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
	return values[0].(map[string]net.IP), values[1].(bool)
}

func (m *mockHandle) DeleteRecord(_ context.Context, _ pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, id string) bool {

	values := m.Call(eventDelete, 1, domain, ipNet, id)
	return values[0].(bool)
}

func (m *mockHandle) UpdateRecord(ctx context.Context, indent pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, id string, ip net.IP) bool {

	values := m.Call(eventUpdate, 1, domain, ipNet, id, ip)
	return values[0].(bool)
}

func (m *mockHandle) CreateRecord(ctx context.Context, indent pp.Indent,
	domain api.FQDN, ipNet ipnet.Type, ip net.IP, ttl int, proxied bool) (string, bool) {

	values := m.Call(eventCreate, 2, domain, ipNet, ip, ttl, proxied)
	return values[0].(string), values[1].(bool)
}

func (m *mockHandle) FlushCache() {
	m.Call(eventFlush, 0)
}

func TestDo(t *testing.T) {
	t.Parallel()

	const (
		quiet     = quiet.VERBOSE
		domain    = api.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		ttl       = 100
		proxied   = true
	)
	var (
		ip1 = net.ParseIP("::1")
	//ip2 = net.ParseIP("::2")
	)

	for name, tc := range map[string]struct {
		ip      net.IP
		ttl     api.TTL
		proxied bool
		script  []interaction
		ok      bool
	}{
		"empty-nil": {
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
		"empty-one": {
			ip: ip1,
			script: []interaction{
				{
					event:     eventList,
					arguments: []interface{}{domain, ipNetwork},
					values:    []interface{}{map[string]net.IP{}, true},
				},
				{
					event:     eventCreate,
					arguments: []interface{}{domain, ipNetwork, ip1, 100, true},
					values:    []interface{}{record1, true},
				},
			},
			ok: true,
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
