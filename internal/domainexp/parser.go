// Package domainexp parses expressions containing domains.
package domainexp

import (
	"errors"
	"slices"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

var (
	// ErrSingleAnd is triggered by a single & (which should have been &&) in an expression.
	// In a domain list, & is just an invalid domain character.
	ErrSingleAnd = errors.New(`use "&&" instead of "&"`)

	// ErrSingleOr is triggered by a single | (which should have been ||) in an expression.
	// In a domain list, | is just an invalid domain character.
	ErrSingleOr = errors.New(`use "||" instead of "|"`)

	// ErrUTF8 is triggered by invalid UTF-8 strings.
	ErrUTF8 = syntax.ErrInvalidUTF8

	errNotBooleanExpression   = errors.New("not a boolean expression")
	errUnexpectedBooleanToken = errors.New("unexpected token in boolean expression")
)

// formID distinguishes the accepted expression and compatibility-list shapes.
type formID string

const (
	formIsCall        formID = "is(...)"
	formIsCallEmpty   formID = "is()"
	formSubCall       formID = "sub(...)"
	formSubCallEmpty  formID = "sub()"
	formNot           formID = "!"
	formGroup         formID = "(...)"
	formComma         formID = ","
	formMissingComma  formID = ",missing"
	formCommaOnly     formID = ",only"
	formLeadingComma  formID = ",lead"
	formTrailingComma formID = ",trail"
	formAnd           formID = "&&"
	formOr            formID = "||"
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

type parserState struct {
	// Empty-call functions are kept in first-occurrence order and deduplicated.
	emptyCallFunctions []string
	// Extra-comma diagnostics are intentionally deduplicated across one parse.
	extraComma bool
	// Missing-comma diagnostics are intentionally deduplicated across one parse.
	missingComma bool
}

// listSyntaxPreview formats potentially long list syntax for advisory messages.
func listSyntaxPreview(input string) string {
	return pp.QuotePreviewOrEmptyLabel(input, pp.AdvisoryPreviewLimit, "empty")
}

// unexpectedTokenError reports token as the source of a generic unexpected-token failure.
func unexpectedTokenError(token syntax.Token) *syntax.ParseError {
	return &syntax.ParseError{
		Span: token.Span, Cause: syntax.ErrUnexpectedToken,
	}
}

func (state *parserState) recordExtraComma() {
	state.extraComma = true
}

func (state *parserState) recordEmptyCall(function string) {
	if slices.Contains(state.emptyCallFunctions, function) {
		return
	}
	state.emptyCallFunctions = append(state.emptyCallFunctions, function)
}

func (state *parserState) recordMissingComma() {
	state.missingComma = true
}

// ParseList parses a list of comma-separated domains. Internationalized domain names are fully supported.
// Domains are validated only after parsing, which preserves each atom's source span.
func ParseList(ppfmt pp.PP, key string, input string) ([]domain.Domain, bool) {
	tree, err := domainListGrammar.Parse(input)
	if err != nil {
		reportParseError(ppfmt, key, input, err)
		return nil, false
	}

	state := &parserState{emptyCallFunctions: nil, extraComma: false, missingComma: false}
	list, err := flattenDomainList(tree, state)
	if err != nil {
		// domainListGrammar produces only the tree shapes accepted by flattenDomainList.
		reportListDiagnostics(ppfmt, key, input, state)
		ppfmt.Noticef(pp.EmojiImpossible,
			"%s (%q) was parsed into an invalid domain-list tree; this should not happen. Please report it at %s",
			key, input, pp.IssueReportingURL)
		return nil, false
	}

	domains := make([]domain.Domain, 0, len(list))
	for i, token := range list {
		d, domainErr := domain.New(token.Text)
		if domainErr != nil {
			reportListDiagnostics(ppfmt, key, input, state)
			if errors.Is(domainErr, domain.ErrNotFQDN) {
				ppfmt.Noticef(
					pp.EmojiUserError,
					`The %s domain in %s (%q) is %q, but it does not appear to be fully qualified; `+
						`a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`,
					pp.Ordinal(i+1), key, input, d.Describe(),
				)
				return nil, false
			}
			ppfmt.Noticef(pp.EmojiUserError, "The %s domain in %s (%q) is %q, but it is malformed: %v",
				pp.Ordinal(i+1), key, input, d.Describe(), domainErr)
			return nil, false
		}
		domains = append(domains, d)
	}
	reportListDiagnostics(ppfmt, key, input, state)
	return domains, true
}

//nolint:gochecknoglobals // Immutable compiled grammars shared by all parse calls.
var (
	// listForms are the compatibility-list forms shared by both grammars.
	// The asymmetric list binding powers make comma-separated and missing-comma
	// compatibility lists left-associative.
	listForms = []syntax.Rule[formID]{
		syntax.Form(formLeadingComma, syntax.Symbol(","), syntax.Hole(6)),
		syntax.Form(formCommaOnly, syntax.Symbol(",")),
		syntax.Form(formComma, syntax.Hole(5), syntax.Symbol(","), syntax.Hole(6)),
		syntax.Form(formTrailingComma, syntax.Hole(5), syntax.Symbol(",")),
		syntax.ImplicitForm(formMissingComma, 5, 6),
	}
	domainListGrammar = syntax.MustNewPratt(slices.Concat(
		[]syntax.Rule[formID]{syntax.Empty[formID]()},
		listForms,
	)...)
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

// mustFirstToken returns the first token of a successfully parsed expression.
func mustFirstToken(tree syntax.Tree[formID]) syntax.Token {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return tree.Token
	case syntax.Op[formID]:
		if len(tree.Tokens) != 0 &&
			(len(tree.Args) == 0 || tree.Tokens[0].Span.Start < tree.Args[0].Span().Start) {
			return tree.Tokens[0]
		}
		return mustFirstToken(tree.Args[0])
	default:
		panic("domainexp: parsed expression has no token")
	}
}

// flattenDomainList accepts only comma-list tree shapes and preserves atom tokens for later validation.
func flattenDomainList(tree syntax.Tree[formID], state *parserState) ([]syntax.Token, *syntax.ParseError) {
	switch tree := tree.(type) {
	case syntax.EmptyTree[formID]:
		return nil, nil
	case syntax.Atom[formID]:
		return []syntax.Token{tree.Token}, nil
	case syntax.Op[formID]:
		switch tree.ID {
		case formComma, formMissingComma:
			left, err := flattenDomainList(tree.Args[0], state)
			if err != nil {
				return nil, err
			}
			if tree.ID == formMissingComma {
				state.recordMissingComma()
			}
			right, err := flattenDomainList(tree.Args[1], state)
			if err != nil {
				return nil, err
			}
			return append(left, right...), nil
		case formCommaOnly:
			state.recordExtraComma()
			return nil, nil
		case formLeadingComma, formTrailingComma:
			if tree.ID == formLeadingComma {
				state.recordExtraComma()
			}
			return flattenDomainList(tree.Args[0], state)
		default:
			return nil, unexpectedTokenError(tree.Tokens[0])
		}
	default:
		// Unreachable for trees produced by the grammars in this package.
		// Defensively handles a nil Tree and implementations not listed above,
		// such as pointers to the three value-type implementations.
		return nil, &syntax.ParseError{
			Span: syntax.Span{Start: 0, End: 0}, Cause: syntax.ErrUnexpectedToken,
		}
	}
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

// reportListDiagnostics emits the compatibility warnings accumulated while flattening a domain list.
func reportListDiagnostics(ppfmt pp.PP, key string, input string, state *parserState) {
	if state.extraComma {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) contains extra commas; this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input))
	}
	if state.missingComma {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"%s (%s) contains missing commas; this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input))
	}
}

// reportParseError translates a generic domain-list parse failure into an operator message.
func reportParseError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	if errors.Is(err, syntax.ErrUnexpectedToken) {
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when "," is expected`,
			key, input, input[err.Span.Start:err.Span.End])
		return
	}
	ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
}

// reportExpressionDiagnostics emits call and compatibility-list warnings in their intended message order.
func reportExpressionDiagnostics(ppfmt pp.PP, key string, input string, state *parserState) {
	switch len(state.emptyCallFunctions) {
	case 0:
	case 1:
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%q) uses %s() with an empty domain list, which always evaluates to false`,
			key, input, state.emptyCallFunctions[0])
	default:
		functions := pp.EnglishJoinMapOrEmptyLabel(
			func(function string) string { return function + "()" },
			state.emptyCallFunctions,
			"",
		)
		ppfmt.Noticef(pp.EmojiUserWarning,
			`%s (%q) uses %s with empty domain lists, which always evaluate to false`,
			key, input, functions)
	}
	if state.extraComma {
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains extra commas inside is(...) or sub(...); "+
				"this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input),
		)
	}
	if state.missingComma {
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			"%s (%s) contains missing commas inside is(...) or sub(...); "+
				"this is accepted for now but will be rejected in version 2.0.0",
			key, listSyntaxPreview(input),
		)
	}
}

// reportExpressionError translates a classified expression failure into an operator message.
func reportExpressionError(ppfmt pp.PP, key string, input string, err *syntax.ParseError) {
	expectedToken, expectedTokenOK := errors.AsType[*syntax.ExpectedTokenError](err)
	missingToken, missingTokenOK := errors.AsType[*syntax.MissingTokenError](err)
	switch {
	case errors.Is(err, errNotBooleanExpression):
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, input)
	case errors.Is(err, errUnexpectedBooleanToken):
		ppfmt.Noticef(
			pp.EmojiUserError,
			"%s (%q) is not a boolean expression: got unexpected token %q",
			key, input, input[err.Span.Start:err.Span.End],
		)
	case expectedTokenOK:
		ppfmt.Noticef(pp.EmojiUserError,
			`%s (%q) has unexpected token %q when %q is expected`,
			key, input, expectedToken.Got, expectedToken.Expected)
	case missingTokenOK:
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, missingToken.Expected)
	case errors.Is(err, syntax.ErrUnexpectedToken):
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, input[err.Span.Start:err.Span.End])
	default:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is malformed: %v", key, input, err.Cause)
	}
}
