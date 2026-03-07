package api_test

import (
	"net/netip"
	"slices"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/api"
)

func TestIDString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "record123", api.ID("record123").String())
}

func TestWAFListDescribe(t *testing.T) {
	t.Parallel()

	require.Equal(t, "account456/list-name", api.WAFList{
		AccountID: "account456",
		Name:      "list-name",
	}.Describe())
}

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

func TestWAFListItemKeepsComment(t *testing.T) {
	t.Parallel()

	item := api.WAFListItem{
		ID:      "item1",
		Prefix:  netip.MustParsePrefix("10.0.0.1/32"),
		Comment: "managed",
	}

	require.Equal(t, api.ID("item1"), item.ID)
	require.Equal(t, netip.MustParsePrefix("10.0.0.1/32"), item.Prefix)
	require.Equal(t, "managed", item.Comment)
}
