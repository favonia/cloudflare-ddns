package domainexp

import (
	"errors"
	"slices"

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

type isExpr struct {
	domains []domain.Domain
}

func (isExpr) expr() {}

type subExpr struct {
	suffixes []domain.Suffix
}

func (subExpr) expr() {}

// invalidDomainError is the cause of a ParseError when an is()/sub() argument
// is malformed (any error other than the soft, accepted-and-kept cases). It
// carries the canonical form for quoting.
type invalidDomainError struct {
	domain string // canonical text of the rejected argument
	cause  error  // the underlying domain.New / domain.NewSuffix error
}

func (e *invalidDomainError) Error() string { return e.cause.Error() }

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

// buildIsCall validates an is(...) call. Malformed arguments hard-fail; a
// too-short argument (domain.ErrTooFewLabels) is accepted and kept — it matches
// nothing exactly as v1.16.2 did — and recorded for the #1 advisory.
func buildIsCall(tree syntax.Op[formID], state *parserState) (Expr, *syntax.ParseError) {
	if tree.ID == formIsCallEmpty {
		state.recordEmptyCall("is")
		return isExpr{domains: nil}, nil
	}
	list, err := flattenDomainList(tree.Args[0], state)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		state.recordEmptyCall("is")
	}
	domains := make([]domain.Domain, 0, len(list))
	for _, token := range list {
		d, derr := domain.New(token.Text)
		switch {
		case derr == nil:
			domains = append(domains, d)
		case errors.Is(derr, domain.ErrTooFewLabels):
			state.recordShortIsTarget(d.String())
			domains = append(domains, d)
		default:
			return nil, &syntax.ParseError{
				Span:  token.Span,
				Cause: &invalidDomainError{domain: d.String(), cause: derr},
			}
		}
	}
	return isExpr{domains: domains}, nil
}

// buildSubCall validates a sub(...) call over domain.Suffix values. Wildcards
// are skipped (a wildcard has no strict subdomains) and recorded for the #2/L1
// advisory; the resulting suffix list may be empty, which evaluates to false.
func buildSubCall(tree syntax.Op[formID], state *parserState) (Expr, *syntax.ParseError) {
	if tree.ID == formSubCallEmpty {
		state.recordEmptyCall("sub")
		return subExpr{suffixes: nil}, nil
	}
	list, err := flattenDomainList(tree.Args[0], state)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		state.recordEmptyCall("sub")
	}
	suffixes := make([]domain.Suffix, 0, len(list))
	for _, token := range list {
		s, serr := domain.NewSuffix(token.Text)
		switch {
		case serr == nil:
			suffixes = append(suffixes, s)
		case errors.Is(serr, domain.ErrWildcardSuffix):
			// Skip + record the wildcard for the #2/L1 advisory. Parse it as a
			// Domain only to render the canonical "*.X" form for the message.
			wd, _ := domain.New(token.Text)
			state.recordSubWildcard(wd)
		default:
			return nil, &syntax.ParseError{
				Span:  token.Span,
				Cause: &invalidDomainError{domain: domain.StringToASCII(token.Text), cause: serr},
			}
		}
	}
	return subExpr{suffixes: suffixes}, nil
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
		case formIsCall, formIsCallEmpty:
			return buildIsCall(tree, state)
		case formSubCall, formSubCallEmpty:
			return buildSubCall(tree, state)
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
//   - Domain arguments must be valid: a malformed argument such as b.*.a.org
//     (a non-leftmost wildcard) is rejected. sub(.) and sub(org) are valid
//     (the root and single-label suffixes); is(.) and is(org) are accepted but
//     match nothing, and raise an advisory since such a short target is rarely
//     intended. sub(*.example.org) is skipped with an advisory, since a wildcard
//     has no valid strict subdomains.
//   - ! exp, where exp is a boolean expression, representing logical negation of exp.
//   - exp1 || exp2, where exp1 and exp2 are boolean expressions, representing logical disjunction of exp1 and exp2.
//   - exp1 && exp2, where exp1 and exp2 are boolean expressions, representing logical conjunction of exp1 and exp2.
//
// One can use parentheses to group expressions, such as !(is(hello.org) && (is(hello.io) || is(hello.me))).
func ParseExpression(ppfmt pp.PP, key string, input string) (Expr, bool) {
	state := &parserState{
		emptyCallFunctions: nil,
		extraComma:         false,
		missingComma:       false,
		shortIsTargets:     nil,
		subWildcards:       nil,
	}
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

// Evaluate evaluates a parsed expression for a domain.
func Evaluate(expr Expr, dom domain.Domain) bool {
	switch expr := expr.(type) {
	case literalExpr:
		return expr.value
	case isExpr:
		return slices.ContainsFunc(expr.domains, func(d domain.Domain) bool {
			return d.DNSNameASCII() == dom.DNSNameASCII()
		})
	case subExpr:
		return slices.ContainsFunc(expr.suffixes, func(s domain.Suffix) bool {
			return dom.HasStrictSuffix(s)
		})
	case unaryExpr:
		return !Evaluate(expr.operand, dom)
	case binaryExpr:
		//nolint:exhaustive // Unrecognized expression forms fall through to false below.
		switch expr.operator {
		case "&&":
			return Evaluate(expr.left, dom) && Evaluate(expr.right, dom)
		case "||":
			return Evaluate(expr.left, dom) || Evaluate(expr.right, dom)
		}
	}
	return false
}
