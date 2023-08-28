// Package fuzzer_test implements the fuzzing interface for OSS-Fuzz.
package fuzzer_test

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type ErrorMatcher struct {
	Error error
}

func (m ErrorMatcher) Matches(x any) bool {
	err, ok := x.(error)
	return ok && errors.Is(err, m.Error)
}

func (m ErrorMatcher) String() string {
	return m.Error.Error()
}

const key string = "KEY"

// FuzzParseList fuzz test [domainexp.ParseList].
func FuzzParseList(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes() //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) contains an ill-formed domain %q: %v`, key, input, gomock.Any(), gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, gomock.Any()).AnyTimes()                 //nolint:lll

		_, _ = domainexp.ParseList(mockPP, key, input)
	})
}

// FuzzParseExpression fuzz test [domainexp.ParseExpression].
func FuzzParseExpression(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		mockCtrl := gomock.NewController(t)
		mockPP := mocks.NewMockPP(mockCtrl)
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleAnd).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is ill-formed: %v`, key, input, domainexp.ErrSingleOr).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, input, ErrorMatcher{domainexp.ErrUTF8}).AnyTimes() //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, input, gomock.Any(), gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, gomock.Any()).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is not a boolean expression: got unexpected token %q`, key, input, gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, input).AnyTimes()
		mockPP.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) contains an ill-formed domain %q: %v`, key, input, gomock.Any(), gomock.Any()).AnyTimes() //nolint:lll
		mockPP.EXPECT().Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, gomock.Any()).AnyTimes()                 //nolint:lll

		_, _ = domainexp.ParseExpression(mockPP, key, input)
	})
}
