package domainexp

import (
	"errors"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

const (
	formIsCall       formID = "is(...)"
	formIsCallEmpty  formID = "is()"
	formSubCall      formID = "sub(...)"
	formSubCallEmpty formID = "sub()"
	formNot          formID = "!"
	formGroup        formID = "(...)"
	formAnd          formID = "&&"
	formOr           formID = "||"
)

// Expr is the AST node interface for domain expressions.
type Expr interface {
	expr()
}

type literalExpr struct {
	value bool
}

func (literalExpr) expr() {}

type callExpr struct {
	function string
	domains  []string
}

func (callExpr) expr() {}

type unaryExpr struct {
	operator formID
	operand  Expr
}

func (unaryExpr) expr() {}

type binaryExpr struct {
	operator formID
	left     Expr
	right    Expr
}

func (binaryExpr) expr() {}

//nolint:gochecknoglobals // Immutable compiled grammars shared by all parse calls.
var (
	// Boolean operators bind more tightly than list forms so is(...) and sub(...)
	// arguments can be validated as lists after the shared parser builds a tree.
	expressionGrammar = syntax.MustNewPratt(slices.Concat(
		[]syntax.Rule[formID]{
			syntax.Form(formIsCallEmpty,
				syntax.Keyword("is"), syntax.Symbol("("), syntax.Symbol(")"),
			),
			syntax.Form(formIsCall,
				syntax.Keyword("is"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
			),
			syntax.Form(formSubCallEmpty,
				syntax.Keyword("sub"), syntax.Symbol("("), syntax.Symbol(")"),
			),
			syntax.Form(formSubCall,
				syntax.Keyword("sub"), syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")"),
			),
			syntax.Form(formNot, syntax.Symbol("!"), syntax.Hole(30)),
			syntax.Form(formGroup, syntax.Symbol("("), syntax.Hole(0), syntax.Symbol(")")),
			syntax.Form(formAnd, syntax.Hole(20), syntax.Symbol("&&"), syntax.Hole(21)),
			syntax.Form(formOr, syntax.Hole(10), syntax.Symbol("||"), syntax.Hole(11)),
		},
		listForms,
	)...)
)

// parseBooleanLiteral recognizes the spellings accepted by strconv.ParseBool.
func parseBooleanLiteral(token syntax.Token) (Expr, bool) {
	switch token.Text {
	case "1", "t", "T", "TRUE", "true", "True":
		return literalExpr{value: true}, true
	case "0", "f", "F", "FALSE", "false", "False":
		return literalExpr{value: false}, true
	default:
		return nil, false
	}
}

// buildCallExpr validates and normalizes an is(...) or sub(...) call while
// recording compatibility-list diagnostics.
func buildCallExpr(tree syntax.Op[formID], state *parserState) (Expr, *syntax.ParseError) {
	var function string
	//nolint:exhaustive // buildExpr dispatches only call forms here
	switch tree.ID {
	case formIsCall, formIsCallEmpty:
		function = "is"
	case formSubCall, formSubCallEmpty:
		function = "sub"
	}
	if tree.ID == formIsCallEmpty || tree.ID == formSubCallEmpty {
		state.recordEmptyCall(function)
		return callExpr{
			function: function,
			domains:  nil,
		}, nil
	}
	list, err := flattenDomainList(tree.Args[0], state)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		state.recordEmptyCall(function)
	}
	domains := make([]string, 0, len(list))
	for _, token := range list {
		domains = append(domains, domain.StringToASCII(token.Text))
	}
	return callExpr{
		function: function,
		domains:  domains,
	}, nil
}

// buildExpr converts a generic parse tree into a domain expression while
// recording diagnostics found in call arguments.
func buildExpr(tree syntax.Tree[formID], state *parserState) (Expr, *syntax.ParseError) {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		if expr, ok := parseBooleanLiteral(tree.Token); ok {
			return expr, nil
		}
		return nil, &syntax.ParseError{
			Span: tree.Token.Span, Cause: errUnexpectedBooleanToken,
		}
	case syntax.Op[formID]:
		switch tree.ID {
		case formMissingComma:
			// Boolean expressions cannot be adjacent without an operator, so report the
			// first token of the right expression. The successfully parsed implicit-form
			// hole always contains such a token.
			token := mustFirstToken(tree.Args[1])
			return nil, &syntax.ParseError{
				Span: token.Span, Cause: errUnexpectedBooleanToken,
			}
		case formNot:
			operand, err := buildExpr(tree.Args[0], state)
			if err != nil {
				return nil, err
			}
			return unaryExpr{
				operator: tree.ID,
				operand:  operand,
			}, nil
		case formAnd, formOr:
			left, err := buildExpr(tree.Args[0], state)
			if err != nil {
				return nil, err
			}
			right, err := buildExpr(tree.Args[1], state)
			if err != nil {
				return nil, err
			}
			return binaryExpr{
				operator: tree.ID,
				left:     left,
				right:    right,
			}, nil
		case formGroup:
			// Grouping shapes the parse tree; the group itself has no meaning.
			return buildExpr(tree.Args[0], state)
		case formIsCall, formSubCall, formIsCallEmpty, formSubCallEmpty:
			return buildCallExpr(tree, state)
		default:
			return nil, &syntax.ParseError{
				Span: tree.Tokens[0].Span, Cause: errUnexpectedBooleanToken,
			}
		}
	default:
		return nil, &syntax.ParseError{
			Span: syntax.Span{Start: 0, End: 0}, Cause: errNotBooleanExpression,
		}
	}
}

// ParseExpression parses a boolean expression containing domains.
// A boolean expression must have one of the following forms:
//
//   - A boolean value accepted by [strconv.ParseBool], such as t as true or FALSE as false.
//   - is(example.org), which matches the domain example.org. Note that is(*.example.org)
//     only matches the wildcard domain *.example.org; use sub(example.org) to match
//     all subdomains of example.org (including *.example.org).
//   - sub(example.org), which matches subdomains of example.org, such as www.example.org and *.example.org.
//     It does not match the domain example.org itself.
//   - ! exp, where exp is a boolean expression, representing logical negation of exp.
//   - exp1 || exp2, where exp1 and exp2 are boolean expressions, representing logical disjunction of exp1 and exp2.
//   - exp1 && exp2, where exp1 and exp2 are boolean expressions, representing logical conjunction of exp1 and exp2.
//
// One can use parentheses to group expressions, such as !(is(hello.org) && (is(hello.io) || is(hello.me))).
func ParseExpression(ppfmt pp.PP, key string, input string) (Expr, bool) {
	state := &parserState{emptyCallFunctions: nil, extraComma: false, missingComma: false}
	tree, err := expressionGrammar.Parse(input)
	if err != nil {
		// Translate generic parser failures into more useful expression diagnostics:
		// single & and | get actionable replacement guidance, while an expression
		// ending before its operand is identified as not being a boolean expression.
		if detail, ok := errors.AsType[*syntax.UnrecognizedSymbolError](err); ok {
			switch detail.LeadingRune {
			case '&':
				err.Cause = ErrSingleAnd
			case '|':
				err.Cause = ErrSingleOr
			}
		}
		if errors.Is(err, syntax.ErrUnexpectedEOF) {
			err = &syntax.ParseError{
				Span: err.Span, Cause: errNotBooleanExpression,
			}
		}
		reportExpressionDiagnostics(ppfmt, key, input, state)
		reportExpressionError(ppfmt, key, input, err)
		return nil, false
	}
	// Building the expression records diagnostics found inside is(...) and sub(...).
	expr, err := buildExpr(tree, state)
	reportExpressionDiagnostics(ppfmt, key, input, state)
	if err != nil {
		reportExpressionError(ppfmt, key, input, err)
		return nil, false
	}
	return expr, true
}

// hasStrictSuffix reports whether suffix is a proper dot-delimited suffix of s.
func hasStrictSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix) && len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.'
}

// Evaluate evaluates a parsed expression for a domain.
func Evaluate(expr Expr, dom domain.Domain) bool {
	switch expr := expr.(type) {
	case literalExpr:
		return expr.value
	case callExpr:
		switch expr.function {
		case "is":
			return slices.Contains(expr.domains, dom.DNSNameASCII())
		case "sub":
			asciiDomain := dom.DNSNameASCII()
			return slices.ContainsFunc(expr.domains, func(pattern string) bool {
				return hasStrictSuffix(asciiDomain, pattern)
			})
		default:
			return false
		}
	case unaryExpr:
		return !Evaluate(expr.operand, dom)
	case binaryExpr:
		switch expr.operator {
		case "&&":
			return Evaluate(expr.left, dom) && Evaluate(expr.right, dom)
		case "||":
			return Evaluate(expr.left, dom) || Evaluate(expr.right, dom)
		default:
			return false
		}
	default:
		return false
	}
}
