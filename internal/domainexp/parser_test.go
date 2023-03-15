package domainexp_test

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func TestParseList(t *testing.T) {
	t.Parallel()
	key := "key"
	type f = domain.FQDN
	type w = domain.Wildcard
	type ds = []domain.Domain
	for name, tc := range map[string]struct {
		input         string
		ok            bool
		expected      ds
		prepareMockPP func(m *mocks.MockPP)
	}{
		"a":        {"a", true, ds{f("a")}, nil},
		"ab":       {" a ,  b ", true, ds{f("a"), f("b")}, nil},
		"abc":      {" a ,  b ,,,,,, c ", true, ds{f("a"), f("b"), f("c")}, nil},
		"wildcard": {" a ,  b ,,,,,, *.c ", true, ds{f("a"), f("b"), w("c")}, nil},
		"missing-comma": {
			" a b c d ", true,
			ds{f("a"), f("b"), f("c"), f("d")},
			func(m *mocks.MockPP) {
				gomock.InOrder(
					m.EXPECT().Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, " a b c d ", "b"),
					m.EXPECT().Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, " a b c d ", "c"),
					m.EXPECT().Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, " a b c d ", "d"),
				)
			},
		},
		"illformed/1": {
			"&", false, nil,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "&", domainexp.ErrSingleAnd)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			list, ok := domainexp.ParseList(mockPP, key, tc.input)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expected, list)
		})
	}
}

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

//nolint:funlen
func TestParseExpression(t *testing.T) {
	t.Parallel()
	key := "key"
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		input         string
		ok            bool
		domain        domain.Domain
		expected      bool
		prepareMockPP func(m *mocks.MockPP)
	}{
		"empty": {
			"", false, nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, "")
			},
		},
		"const/1": {"true", true, nil, true, nil},
		"const/2": {"f", true, nil, false, nil},
		"&&/1":    {"t && 0", true, nil, false, nil},
		"&&/2": {
			"t &&", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, "t &&")
			},
		},
		"&&/&/1": {
			"true & true", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "true & true", domainexp.ErrSingleAnd)
			},
		},
		"&&/&/2": {
			"true &", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "true &", domainexp.ErrSingleAnd)
			},
		},
		"||/1": {"F || 1", true, nil, true, nil},
		"||/2": {
			"F ||", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is not a boolean expression`, key, "F ||")
			},
		},
		"||/|/1": {
			"false | false", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "false | false", domainexp.ErrSingleOr)
			},
		},
		"||/|/2": {
			"false |", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "false |", domainexp.ErrSingleOr)
			},
		},
		"is/1":          {"is(example.com)", true, f("example.com"), true, nil},
		"is/2":          {"is(example.com)", true, f("sub.example.com"), false, nil},
		"is/3":          {"is(example.org)", true, f("example.com"), false, nil},
		"is/wildcard/1": {"is(example.com)", true, w("example.com"), false, nil},
		"is/wildcard/2": {"is(*.example.com)", true, w("example.com"), true, nil},
		"is/wildcard/3": {"is(*.example.com)", true, f("example.com"), false, nil},
		"is/idn/1":      {"is(☕.de)", true, f("xn--53h.de"), true, nil},
		"is/idn/2":      {"is(Xn--53H.de)", true, f("xn--53h.de"), true, nil},
		"is/idn/3":      {"is(*.Xn--53H.de)", true, w("xn--53h.de"), true, nil},
		"is/error/1": {
			"is)", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, "is)", ")", "(")
			},
		},
		"is/error/2": {
			"is(&&", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, "is(&&", "&&")
			},
		},
		"is/error/3": {
			"is", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, "is", "(")
			},
		},
		"sub/1":     {"sub(example.com)", true, f("example.com"), false, nil},
		"sub/2":     {"sub(example.com)", true, w("example.com"), true, nil},
		"sub/3":     {"sub(example.com)", true, f("sub.example.com"), true, nil},
		"sub/4":     {"sub(example.com)", true, f("subexample.com"), false, nil},
		"sub/idn/1": {"sub(☕.de)", true, f("www.xn--53h.de"), true, nil},
		"sub/idn/2": {"sub(Xn--53H.de)", true, f("www.xn--53h.de"), true, nil},
		"sub/idn/3": {"sub(Xn--53H.de)", true, w("xn--53h.de"), true, nil},
		"not/1":     {"!0", true, nil, true, nil},
		"not/2":     {"!!!!!!!!!!!0", true, nil, true, nil},
		"not/3": {
			"!(", false, nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, "!(")
			},
		},
		"nested/1": {"((true)||(false))&&((false)||(true))", true, nil, true, nil},
		"nested/2": {
			"((", false, nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, "((")
			},
		},
		"nested/3": {
			"(true", false, nil, true,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is missing %q at the end", key, "(true", ")")
			},
		},
		"error/extra": {
			"0 1", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, "0 1", "1")
			},
		},
		"utf8/invalid": {
			"\200\300", false, nil, false,
			func(m *mocks.MockPP) {
				m.EXPECT().Errorf(pp.EmojiUserError, "%s (%q) is ill-formed: %v", key, "\200\300", ErrorMatcher{domainexp.ErrUTF8}) //nolint:lll
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			pred, ok := domainexp.ParseExpression(mockPP, "key", tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, tc.expected, pred(tc.domain))
			}
		})
	}
}
