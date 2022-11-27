// Package fuzzer_test implements the fuzzing interface for OSS-Fuzz.
package fuzzer_test

import (
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// FuzzParseList fuzz test [domainexp.ParseList].
func FuzzParseList(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: %v`, gomock.Any(), domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: %v`, gomock.Any(), domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: unexpected token %q`, gomock.Any(), gomock.Any()).AnyTimes()        //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, `Domain %q was added but it is ill-formed: %v`, gomock.Any(), gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, `Please insert a comma "," before %q`, gomock.Any()).AnyTimes()

		_, _ = domainexp.ParseList(mockPP, input)
	})
}

// FuzzParseExpression fuzz test [domainexp.ParseExpression].
func FuzzParseExpression(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: %v`, gomock.Any(), domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: %v`, gomock.Any(), domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: unexpected token %q`, gomock.Any(), gomock.Any()).AnyTimes()                  //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: wanted %q; got %q`, gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()      //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: wanted %q; reached end of string`, gomock.Any(), gomock.Any()).AnyTimes()     //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: wanted a boolean expression; got %q`, gomock.Any(), gomock.Any()).AnyTimes()  //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `Failed to parse %q: wanted a boolean expression; reached end of string`, gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, "Domain %q was added but it is ill-formed: %v", gomock.Any(), gomock.Any()).AnyTimes()           //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, `Please insert a comma "," before %q`, gomock.Any()).AnyTimes()

		_, _ = domainexp.ParseExpression(mockPP, input)
	})
}
