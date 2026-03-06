package api

import (
	"context"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestMatchManagedWAFListItemComment(t *testing.T) {
	t.Parallel()

	regex := regexp.MustCompile("^managed$")

	require.True(t, matchManagedWAFListItemComment(nil, "any comment"))
	require.True(t, matchManagedWAFListItemComment(regex, "managed"))
	require.False(t, matchManagedWAFListItemComment(regex, "foreign"))
}

func TestStartDeletingWAFListItemsAsyncWithNoIDs(t *testing.T) {
	t.Parallel()

	var h CloudflareHandle
	ok := h.startDeletingWAFListItemsAsync(
		context.Background(),
		pp.New(io.Discard, false, pp.Quiet),
		WAFList{AccountID: "", Name: ""},
		"",
		nil,
	)
	require.True(t, ok)
}
