package api

import (
	"context"
	"net/netip"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestMatchManagedWAFListItemComment(t *testing.T) {
	t.Parallel()

	regex := regexp.MustCompile("^managed$")
	policyWithNil := HandleOwnershipPolicy{
		ManagedRecordsCommentRegex:        nil,
		ManagedWAFListItemsCommentRegex:   nil,
		AllowWholeWAFListDeleteOnShutdown: false,
	}
	policyWithRegex := HandleOwnershipPolicy{
		ManagedRecordsCommentRegex:        nil,
		ManagedWAFListItemsCommentRegex:   regex,
		AllowWholeWAFListDeleteOnShutdown: false,
	}

	require.True(t, policyWithNil.MatchManagedWAFListItemComment("any comment"))
	require.True(t, policyWithRegex.MatchManagedWAFListItemComment("managed"))
	require.False(t, policyWithRegex.MatchManagedWAFListItemComment("foreign"))
}

func TestStartDeletingWAFListItemsAsyncWithNoIDs(t *testing.T) {
	t.Parallel()

	var h cloudflareHandle
	ok := h.startDeletingWAFListItemsAsync(
		context.Background(),
		pp.NewSilent(),
		WAFList{AccountID: "", Name: ""},
		"",
		nil,
	)
	require.True(t, ok)
}

func TestHintUnexpectedWAFListItemCommentAfterMutationAcceptsNewExpectedComment(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		hintUnexpectedWAFListItemCommentAfterMutation(
			pp.NewSilent(),
			WAFList{AccountID: "account", Name: "list"},
			map[ID]string{},
			[]WAFListItem{
				{ID: "new-item", Prefix: netip.MustParsePrefix("192.0.2.1/32"), Comment: "expected"},
			},
			map[string]bool{"expected": true},
		)
	})
}

func TestDescribeInScopeWAFFamilies(t *testing.T) {
	t.Parallel()

	require.Equal(t, "IPv4 and IPv6", describeInScopeWAFFamilies(map[ipnet.Family]bool{
		ipnet.IP4: true,
		ipnet.IP6: true,
	}))
	require.Equal(t, "IPv4", describeInScopeWAFFamilies(map[ipnet.Family]bool{
		ipnet.IP4: true,
	}))
	require.Equal(t, "IPv6", describeInScopeWAFFamilies(map[ipnet.Family]bool{
		ipnet.IP6: true,
	}))
	require.Equal(t, "no", describeInScopeWAFFamilies(map[ipnet.Family]bool{}))
}

func TestDescribeAllowedWAFListItemComments(t *testing.T) {
	t.Parallel()

	require.Equal(t, "none", describeAllowedWAFListItemComments(map[string]bool{}))
	require.Equal(t, `"a", "z", empty`, describeAllowedWAFListItemComments(map[string]bool{
		"z": true,
		"":  true,
		"a": true,
	}))
}
