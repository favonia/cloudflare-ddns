package setter

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type noopPP struct{}

func (noopPP) IsShowing(pp.Verbosity) bool                  { return false }
func (noopPP) Indent() pp.PP                                { return noopPP{} }
func (noopPP) BlankLineIfVerbose()                          {}
func (noopPP) Infof(pp.Emoji, string, ...any)               {}
func (noopPP) Noticef(pp.Emoji, string, ...any)             {}
func (noopPP) Suppress(pp.ID)                               {}
func (noopPP) InfoOncef(pp.ID, pp.Emoji, string, ...any)    {}
func (noopPP) NoticeOncef(pp.ID, pp.Emoji, string, ...any)  {}

func TestPartitionRecordsReturnsSparseMatchesAndOrderedUnmatched(t *testing.T) {
	t.Parallel()

	ip1 := netip.MustParseAddr("::1")
	ip2 := netip.MustParseAddr("::2")
	ip3 := netip.MustParseAddr("::3")
	ip4 := netip.MustParseAddr("::4")
	targets := []netip.Addr{ip1, ip2, ip3}
	records := []api.Record{
		{ID: "record2", IP: ip2},
		{ID: "record4", IP: ip4},
	}

	matched, unmatched, stale := partitionRecords(targets, records)
	require.Len(t, matched, 1)
	require.Contains(t, matched, ip2)
	require.Equal(t, []netip.Addr{ip1, ip3}, unmatched)
	require.Len(t, stale, 1)
	require.Equal(t, api.ID("record4"), stale[0].ID)
}

func TestReconcileAndPartitionRecordsSortsOutputsByID(t *testing.T) {
	t.Parallel()

	configured := api.RecordParams{TTL: api.TTLAuto, Comment: "hello"}
	records := []Record{
		{ID: "record3", RecordParams: configured},
		{ID: "record1", RecordParams: configured},
		{ID: "record2", RecordParams: api.RecordParams{TTL: api.TTLAuto, Comment: "other"}},
	}

	resolved, matching, nonMatching := reconcileAndPartitionRecords(
		configured,
		records,
		noopPP{},
		newAmbiguityWarnings(),
		"AAAA records of sub.test.org",
	)

	require.Equal(t, configured, resolved)
	require.Equal(t, []Record{
		{ID: "record1", RecordParams: configured},
		{ID: "record3", RecordParams: configured},
	}, matching)
	require.Equal(t, []Record{
		{ID: "record2", RecordParams: api.RecordParams{TTL: api.TTLAuto, Comment: "other"}},
	}, nonMatching)
}

