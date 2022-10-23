package domainexp_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/domainexp"
	"github.com/favonia/cloudflare-ddns/internal/mocks"
)

func TestParseExpression(t *testing.T) {
	t.Parallel()
	type f = domain.FQDN
	type w = domain.Wildcard
	for name, tc := range map[string]struct {
		input         string
		ok            bool
		domain        domain.Domain
		expected      bool
		prepareMockPP func(m *mocks.MockPP)
	}{
		"true":             {"true", true, f(""), true, nil},
		"f":                {"f", true, w(""), false, nil},
		"and/t-0":          {"t && 0", true, f(""), false, nil},
		"or/F-1":           {"F || 1", true, w(""), true, nil},
		"is/matched/1":     {"is(example.com)", true, f("example.com"), true, nil},
		"is/matched/idn/1": {"is(☕.de)", true, f("xn--53h.de"), true, nil},
		"is/matched/idn/2": {"is(Xn--53H.de)", true, f("xn--53h.de"), true, nil},
		"is/matched/idn/3": {"is(*.Xn--53H.de)", true, w("xn--53h.de"), true, nil},
		"is/unmatched/1":   {"is(example.org)", true, f("example.com"), false, nil},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			mockPP := mocks.NewMockPP(mockCtrl)
			if tc.prepareMockPP != nil {
				tc.prepareMockPP(mockPP)
			}

			pred, ok := domainexp.ParseExpression(mockPP, tc.input)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, tc.expected, pred(tc.domain))
			}
		})
	}
}
