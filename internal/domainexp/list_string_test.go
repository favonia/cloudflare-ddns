package domainexp_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestParseListRootQuotesString(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	ppfmt := mocks.NewMockPP(mockCtrl)
	// The not-fully-qualified message quotes the domain (4th vararg) as "." not "".
	ppfmt.EXPECT().Noticef(pp.EmojiUserError, gomock.Any(), "1st", "DOMAINS", ".", ".")

	_, ok := domainexp.ParseList(ppfmt, "DOMAINS", ".")
	require.False(t, ok)
}
