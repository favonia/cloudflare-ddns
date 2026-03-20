package setter

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestPartitionRecordsReturnsSparseMatchesAndOrderedUnmatched(t *testing.T) {
	t.Parallel()

	ip1 := netip.MustParseAddr("::1")
	ip2 := netip.MustParseAddr("::2")
	ip3 := netip.MustParseAddr("::3")
	ip4 := netip.MustParseAddr("::4")
	targets := []netip.Addr{ip1, ip2, ip3}
	records := []api.Record{
		{ID: "record2", IP: ip2, RecordParams: api.RecordParams{TTL: 0, Proxied: false, Comment: "", Tags: nil}},
		{ID: "record4", IP: ip4, RecordParams: api.RecordParams{TTL: 0, Proxied: false, Comment: "", Tags: nil}},
	}

	matched, unmatched, outdated := partitionRecords(targets, records)
	require.Len(t, matched, 1)
	require.Contains(t, matched, ip2)
	require.Equal(t, []netip.Addr{ip1, ip3}, unmatched)
	require.Len(t, outdated, 1)
	require.Equal(t, api.ID("record4"), outdated[0].ID)
}

func TestPartitionRecordsTreatsIPv4MappedIPv6AsMatchingCanonicalIPv4(t *testing.T) {
	t.Parallel()

	target := netip.MustParseAddr("192.0.2.10")
	targets := []netip.Addr{target}
	records := []api.Record{
		{ID: "mapped", IP: netip.MustParseAddr("::ffff:192.0.2.10")},
		{ID: "invalid", IP: netip.Addr{}},
	}

	matched, unmatched, outdated := partitionRecords(targets, records)
	require.Len(t, matched, 1)
	require.Contains(t, matched, target)
	require.Equal(t, []Record{{ID: "mapped"}}, matched[target])
	require.Empty(t, unmatched)
	require.Equal(t, []Record{{ID: "invalid"}}, outdated)
}

func TestResolveScalarValue(t *testing.T) {
	t.Parallel()

	value, ambiguous := resolveScalarValue("default", nil)
	require.Equal(t, "default", value)
	require.False(t, ambiguous)

	value, ambiguous = resolveScalarValue("default", []string{"same", "same", "same"})
	require.Equal(t, "same", value)
	require.False(t, ambiguous)

	value, ambiguous = resolveScalarValue("default", []string{"a", "b"})
	require.Equal(t, "default", value)
	require.True(t, ambiguous)
}

func TestResolveScalarValueOrderInvariant(t *testing.T) {
	t.Parallel()

	fallback := "fallback"
	input := []string{"z", "a", "z", "z"}

	valueA, ambiguousA := resolveScalarValue(fallback, input)
	slices.Reverse(input)
	valueB, ambiguousB := resolveScalarValue(fallback, input)

	require.Equal(t, valueA, valueB)
	require.Equal(t, ambiguousA, ambiguousB)
}

func TestAmbiguityWarningsWarnDeduplicatesPerUnitAndField(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	ppfmt := pp.New(&buf, false, pp.Verbose)
	warnings := newAmbiguityWarnings()

	warnings.warn(ppfmt, 2, "AAAA records of sub.test.org", "comments", "fallback value")
	warnings.warn(ppfmt, 5, "AAAA records of sub.test.org", "comments", "fallback value")
	warnings.warn(ppfmt, 2, "AAAA records of sub.test.org", "TTL values", "fallback value")
	warnings.warn(ppfmt, 3, "A records of sub.test.org", "comments", "fallback value")

	require.Equal(t,
		"The 2 outdated AAAA records of sub.test.org disagree on comments; using fallback value\n"+
			"The 2 outdated AAAA records of sub.test.org disagree on TTL values; using fallback value\n"+
			"The 3 outdated A records of sub.test.org disagree on comments; using fallback value\n",
		buf.String(),
	)
}

func TestReconcileAndPartitionRecordsSortsOutputsByID(t *testing.T) {
	t.Parallel()

	fallback := api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "hello", Tags: nil}
	records := []Record{
		{ID: "record3", RecordParams: fallback},
		{ID: "record1", RecordParams: fallback},
		{ID: "record2", RecordParams: api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "other", Tags: nil}},
	}

	resolved, matching, nonMatching := reconcileAndPartitionRecords(
		fallback,
		records,
		pp.NewSilent(),
		newAmbiguityWarnings(),
		"AAAA records of sub.test.org",
	)

	require.Equal(t, fallback, resolved)
	require.Equal(t, []Record{
		{ID: "record1", RecordParams: fallback},
		{ID: "record3", RecordParams: fallback},
	}, matching)
	require.Equal(t, []Record{
		{ID: "record2", RecordParams: api.RecordParams{TTL: api.TTLAuto, Proxied: false, Comment: "other", Tags: nil}},
	}, nonMatching)
}

func TestReconcileAndPartitionRecordsMixesInheritedAndFallbackFields(t *testing.T) {
	t.Parallel()

	fallback := api.RecordParams{
		TTL:     600,
		Proxied: false,
		Comment: "fallback-comment",
		Tags:    []string{"region:us"},
	}
	records := []Record{
		{
			ID: "record-b",
			RecordParams: api.RecordParams{
				TTL:     120,
				Proxied: true,
				Comment: "carry-me",
				Tags:    []string{"env:prod", "team:alpha"},
			},
		},
		{
			ID: "record-a",
			RecordParams: api.RecordParams{
				TTL:     120,
				Proxied: false,
				Comment: "carry-me",
				Tags:    []string{"env:prod"},
			},
		},
	}

	var buf strings.Builder
	ppfmt := pp.New(&buf, false, pp.Verbose)

	resolved, matching, nonMatching := reconcileAndPartitionRecords(
		fallback,
		records,
		ppfmt,
		newAmbiguityWarnings(),
		"AAAA records of sub.test.org",
	)

	require.Equal(t,
		"The 2 outdated AAAA records of sub.test.org disagree on tags; using common subset\n"+
			"The 2 outdated AAAA records of sub.test.org disagree on proxy states; using fallback value\n",
		buf.String(),
	)
	require.Equal(t, api.RecordParams{
		TTL:     120,
		Proxied: false,
		Comment: "carry-me",
		Tags:    []string{"env:prod"},
	}, resolved)
	require.Equal(t, []Record{
		{
			ID: "record-a",
			RecordParams: api.RecordParams{
				TTL:     120,
				Proxied: false,
				Comment: "carry-me",
				Tags:    []string{"env:prod"},
			},
		},
	}, matching)
	require.Equal(t, []Record{
		{
			ID: "record-b",
			RecordParams: api.RecordParams{
				TTL:     120,
				Proxied: true,
				Comment: "carry-me",
				Tags:    []string{"env:prod", "team:alpha"},
			},
		},
	}, nonMatching)
}
