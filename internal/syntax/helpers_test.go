package syntax_test

import (
	"testing"

	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

func render(tree syntax.Tree[string]) string {
	switch tree := tree.(type) {
	case syntax.EmptyTree[string]:
		return "<empty>"
	case syntax.Atom[string]:
		return tree.Token.Text
	case syntax.Op[string]:
		switch tree.ID {
		case "sum(...)":
			return "sum(" + render(tree.Args[0]) + ")"
		case "+", "*":
			return "(" + render(tree.Args[0]) + " " + tree.ID + " " + render(tree.Args[1]) + ")"
		case "!":
			return "(!" + render(tree.Args[0]) + ")"
		case "(...)":
			return "(" + render(tree.Args[0]) + ")"
		case ",":
			return render(tree.Args[0]) + ", " + render(tree.Args[1])
		case "juxtapose":
			return "(" + render(tree.Args[0]) + " " + render(tree.Args[1]) + ")"
		default:
			return tree.ID
		}
	default:
		return ""
	}
}

func arithmeticGrammar(t *testing.T) syntax.Pratt[string] {
	t.Helper()
	return syntax.MustNewPratt(
		syntax.Form("sum(...)",
			syntax.Keyword("sum"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
		syntax.Form("(...)",
			syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
		),
		syntax.Form("!", syntax.Symbol("!"), syntax.Hole(30)),
		syntax.Form(",", syntax.Hole(0), syntax.Symbol(","), syntax.Hole(1)),
		syntax.Form("+", syntax.Hole(10), syntax.Symbol("+"), syntax.Hole(11)),
		syntax.Form("*", syntax.Hole(20), syntax.Symbol("*"), syntax.Hole(21)),
	)
}
