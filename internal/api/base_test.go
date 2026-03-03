package api_test

import (
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
