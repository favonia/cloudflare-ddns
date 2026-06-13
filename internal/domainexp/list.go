package domainexp

import (
	"errors"
	"slices"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

const (
	formComma         formID = ","
	formMissingComma  formID = ",missing"
	formCommaOnly     formID = ",only"
	formLeadingComma  formID = ",lead"
	formTrailingComma formID = ",trail"
)

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
)

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
