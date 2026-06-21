package ipfilter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// The grammar only ever produces the known forms, so these trees cannot escape
// [Parse]. The tests pin the defensive contract: junk trees degrade to a
// notFilterFault instead of panicking or building a bogus expression.

func TestBuildExprRejectsImpossibleTrees(t *testing.T) {
	t.Parallel()
	for name, tree := range map[string]syntax.Tree[formID]{
		"atom":         syntax.Atom[formID]{Token: syntax.Token{Text: "", Span: syntax.Span{Start: 0, End: 0}}},
		"unknown-form": syntax.Op[formID]{ID: formID("?"), Tokens: nil, Args: nil},
		"empty-tree":   syntax.EmptyTree[formID]{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, f := buildExpr(tree, ipnet.IP4, true)
			require.Nil(t, got)
			require.Equal(t, notFilterFault{}, f)
		})
	}
}

func TestBuildAddrInRejectsNonAtom(t *testing.T) {
	t.Parallel()
	got, f := buildAddrIn(syntax.EmptyTree[formID]{}, ipnet.IP4)
	require.Nil(t, got)
	require.Equal(t, notFilterFault{}, f)
}
