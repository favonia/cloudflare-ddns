package domainexp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func renderEntrySyntax(tree syntax.Tree[entryFormID]) string {
	switch tree := tree.(type) {
	case syntax.EmptyTree[entryFormID]:
		return "empty"
	case syntax.Atom[entryFormID]:
		return tree.Token.Text
	case syntax.Op[entryFormID]:
		args := make([]string, 0, len(tree.Args))
		for _, arg := range tree.Args {
			args = append(args, renderEntrySyntax(arg))
		}
		return fmt.Sprintf("%s(%s)", tree.ID, strings.Join(args, ", "))
	default:
		return "<unexpected>"
	}
}

func TestEntrySyntaxAccepts(t *testing.T) {
	t.Parallel()

	for input, expected := range map[string]string{
		"":                                      "empty",
		"example.org":                           "example.org",
		"example.org{}":                         "fields-empty(example.org)",
		"example.org{hostid6=::1}":              "fields(example.org, assign(hostid6, ::1))",
		"example.org{hostid6=::1,}":             "fields(example.org, trailing(assign(hostid6, ::1)))",
		"example.org{hostid6=::1,hostid6=::1,}": "fields(example.org, trailing(comma(assign(hostid6, ::1), assign(hostid6, ::1))))",
		"example.org{hostid6=[preserve,::1,mac(00-11-22-33-44-55),],}": "fields(example.org, trailing(assign(hostid6, bracket(trailing(comma(comma(preserve, ::1), mac(00-11-22-33-44-55)))))))",
		"example.org,example.net,":                                     "trailing(comma(example.org, example.net))",
		"example.org example.net":                                      "missing(example.org, example.net)",
		"example.org,,example.net":                                     "comma(example.org, leading(example.net))",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			tree, err := parseEntrySyntax(input)
			require.Nil(t, err)
			require.Equal(t, expected, renderEntrySyntax(tree))
		})
	}
}

func TestEntrySyntaxRejects(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"example.org{hostid6=[]}",
		"example.org{hostid6=[::1,,::2]}",
		"example.org{hostid6=::1,,hostid6=::2}",
		"example.org{hostid6=}",
		"example.org{=::1}",
		"example.org{",
		"example.org{} example.net",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			tree, err := parseEntrySyntax(input)
			require.Nil(t, tree)
			require.NotNil(t, err)
		})
	}
}
