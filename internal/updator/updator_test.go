package updator_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
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

func (m *mockHandle) ListRecords(_ context.Context, _ pp.Fmt,
	domain api.FQDN, ipNet ipnet.Type) (map[string]net.IP, bool) {
	values := m.Call(eventList, 2, domain, ipNet)

	val0, _ := values[0].(map[string]net.IP)
	val1, _ := values[1].(bool)

	return val0, val1
}

func (m *mockHandle) DeleteRecord(_ context.Context, _ pp.Fmt,
	domain api.FQDN, ipNet ipnet.Type, id string) bool {
	values := m.Call(eventDelete, 1, domain, ipNet, id)
	val0, _ := values[0].(bool)
	return val0
}

func (m *mockHandle) UpdateRecord(_ context.Context, _ pp.Fmt,
	domain api.FQDN, ipNet ipnet.Type, id string, ip net.IP) bool {
	values := m.Call(eventUpdate, 1, domain, ipNet, id, ip)
	val0, _ := values[0].(bool)
	return val0
}

func (m *mockHandle) CreateRecord(_ context.Context, _ pp.Fmt,
	domain api.FQDN, ipNet ipnet.Type, ip net.IP, ttl api.TTL, proxied bool) (string, bool) {
	values := m.Call(eventCreate, 2, domain, ipNet, ip, ttl, proxied)
	val0, _ := values[0].(string)
	val1, _ := values[1].(bool)
	return val0, val1
}

func (m *mockHandle) FlushCache() {
	require.FailNow(m.t, "updator should never call FlushCache directly")
}

//nolint:funlen
func TestDo(t *testing.T) {
	t.Parallel()

	type anys = []interface{}

	const (
		domain    = api.FQDN("sub.test.org")
		ipNetwork = ipnet.IP6
		record1   = "record1"
		record2   = "record2"
		record3   = "record3"
		ttl       = api.TTL(100)
		proxied   = true
	)
	var (
		ip1 = net.ParseIP("::1")
		ip2 = net.ParseIP("::2")
	)

	for name, tc := range map[string]struct {
		ip        net.IP
		script    []interaction
		ok        bool
		ppRecords []pp.Record
	}{
		"0-nil": {
			nil,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{}, true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiAlreadyDone, `The AAAA records of "sub.test.org" are already up to date`),
			},
		},
		"0": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{}, true}},
				{eventCreate, anys{domain, ipNetwork, ip1, ttl, proxied}, anys{record1, true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiAddRecord, `Added a new AAAA record of "sub.test.org" (ID: record1)`),
			},
		},
		"1unmatched": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiUpdateRecord, `Updated a stale AAAA record of "sub.test.org" (ID: record1)`),
			},
		},
		"1unmatched-updatefail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{true}},
				{eventCreate, anys{domain, ipNetwork, ip1, ttl, proxied}, anys{record2, true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record1)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiAddRecord, `Added a new AAAA record of "sub.test.org" (ID: record2)`),
			},
		},
		"1unmatched-nil": {
			nil,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip1}, true}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record1)`),
			},
		},
		"1matched": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip1}, true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Info, pp.EmojiAlreadyDone, `The AAAA records of "sub.test.org" are already up to date`),
			},
		},
		"2matched": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip1, record2: ip1}, true}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a duplicate AAAA record of "sub.test.org" (ID: record2)`),
			},
		},
		"2matched-deletefail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip1, record2: ip1}, true}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{false}},
			},
			true,
			nil,
		},
		"2unmatched": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2, record2: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{true}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiUpdateRecord, `Updated a stale AAAA record of "sub.test.org" (ID: record1)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record2)`),
			},
		},
		"2unmatched-updatefail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2, record2: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{true}},
				{eventUpdate, anys{domain, ipNetwork, record2, ip1}, anys{true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record1)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiUpdateRecord, `Updated a stale AAAA record of "sub.test.org" (ID: record2)`),
			},
		},
		"2unmatched-updatefailtwice": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2, record2: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{true}},
				{eventUpdate, anys{domain, ipNetwork, record2, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{true}},
				{eventCreate, anys{domain, ipNetwork, ip1, ttl, proxied}, anys{record3, true}},
			},
			true,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record1)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record2)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiAddRecord, `Added a new AAAA record of "sub.test.org" (ID: record3)`),
			},
		},
		"2unmatched-updatefail-deletefail-updatefail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2, record2: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{false}},
				{eventUpdate, anys{domain, ipNetwork, record2, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{true}},
				{eventCreate, anys{domain, ipNetwork, ip1, ttl, proxied}, anys{record3, true}},
			},
			false,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record2)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiAddRecord, `Added a new AAAA record of "sub.test.org" (ID: record3)`),
				pp.NewRecord(0, pp.Error, pp.EmojiError, `Failed to (fully) update AAAA records of "sub.test.org"`),
			},
		},
		"2unmatched-updatefailtwice-createfail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{map[string]net.IP{record1: ip2, record2: ip2}, true}},
				{eventUpdate, anys{domain, ipNetwork, record1, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record1}, anys{true}},
				{eventUpdate, anys{domain, ipNetwork, record2, ip1}, anys{false}},
				{eventDelete, anys{domain, ipNetwork, record2}, anys{true}},
				{eventCreate, anys{domain, ipNetwork, ip1, ttl, proxied}, anys{record3, false}},
			},
			false,
			[]pp.Record{
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record1)`),
				pp.NewRecord(0, pp.Notice, pp.EmojiDelRecord, `Deleted a stale AAAA record of "sub.test.org" (ID: record2)`),
				pp.NewRecord(0, pp.Error, pp.EmojiError, `Failed to (fully) update AAAA records of "sub.test.org"`),
			},
		},
		"listfail": {
			ip1,
			[]interaction{
				{eventList, anys{domain, ipNetwork}, anys{nil, false}},
			},
			false,
			[]pp.Record{
				pp.NewRecord(0, pp.Error, pp.EmojiError, `Failed to (fully) update AAAA records of "sub.test.org"`),
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := &mockHandle{t: t, script: tc.script}
			ppmock := pp.NewMock()
			ok := updator.Do(context.Background(), ppmock,
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
			require.Equal(t, tc.ppRecords, ppmock.Records)
		})
	}
}
