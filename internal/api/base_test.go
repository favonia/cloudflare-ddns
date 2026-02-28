package api_test

import (
	"net/netip"
	"regexp"
	"slices"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

func TestCompareWAFList(t *testing.T) {
	t.Parallel()

	require.NoError(t, quick.Check(
		func(ls []api.WAFList) bool {
			copied := make([]api.WAFList, len(ls))
			copy(copied, ls)

			slices.SortFunc(ls, api.CompareWAFList)

			require.ElementsMatch(t, copied, ls)
			require.True(t, slices.IsSortedFunc(ls, api.CompareWAFList))

			return true
		},
		nil,
	))
}

func TestManagedRecordFilterMatchComment(t *testing.T) {
	t.Parallel()

	t.Run("nil regex matches all comments", func(t *testing.T) {
		t.Parallel()

		filter := api.ManagedRecordFilter{CommentRegex: nil}
		require.True(t, filter.MatchComment(""))
		require.True(t, filter.MatchComment("managed"))
	})

	t.Run("compiled regex filters comments", func(t *testing.T) {
		t.Parallel()

		filter := api.ManagedRecordFilter{CommentRegex: regexp.MustCompile("^managed$")}
		require.True(t, filter.MatchComment("managed"))
		require.False(t, filter.MatchComment(""))
		require.False(t, filter.MatchComment("unmanaged"))
	})
}

func TestManagedRecordFilterFilterRecords(t *testing.T) {
	t.Parallel()

	record1 := api.Record{
		ID: api.ID("record1"),
		IP: netip.MustParseAddr("::1"),
		RecordParams: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "managed",
		},
	}
	record2 := api.Record{
		ID: api.ID("record2"),
		IP: netip.MustParseAddr("::2"),
		RecordParams: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "unmanaged",
		},
	}
	record3 := api.Record{
		ID: api.ID("record3"),
		IP: netip.MustParseAddr("::3"),
		RecordParams: api.RecordParams{
			TTL:     api.TTLAuto,
			Proxied: false,
			Comment: "managed",
		},
	}
	records := []api.Record{record1, record2, record3}

	t.Run("nil regex keeps all records", func(t *testing.T) {
		t.Parallel()

		filter := api.ManagedRecordFilter{CommentRegex: nil}
		require.Equal(t, records, filter.FilterRecords(records))
	})

	t.Run("compiled regex keeps matching records in order", func(t *testing.T) {
		t.Parallel()

		filter := api.ManagedRecordFilter{CommentRegex: regexp.MustCompile("^managed$")}
		require.Equal(t, []api.Record{record1, record3}, filter.FilterRecords(records))
	})
}
