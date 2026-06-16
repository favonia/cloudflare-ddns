package domainentry

import "github.com/favonia/cloudflare-ddns/internal/syntax"

// formID distinguishes the structured-domain-entry syntax tree shapes.
type formID string

const (
	formFieldsEmpty   formID = "fields-empty"
	formFields        formID = "fields"
	formAssign        formID = "assign"
	formMAC           formID = "mac"
	formBracket       formID = "bracket"
	formComma         formID = "comma"
	formTrailingComma formID = "trailing"
	formMissingComma  formID = "missing"
	formLeadingComma  formID = "leading"
	formCommaOnly     formID = "comma-only"
)

//nolint:gochecknoglobals // Immutable compiled grammar shared by all parse calls.
var syntaxGrammar = syntax.MustNewPratt(
	syntax.Empty[formID](),
	syntax.Form(formFieldsEmpty,
		syntax.Hole(40), syntax.Symbol("{"), syntax.Symbol("}"),
	),
	syntax.Form(formFields,
		syntax.Hole(40), syntax.Symbol("{"), syntax.Hole(20), syntax.Symbol("}"),
	),
	syntax.Form(formAssign,
		syntax.Hole(30), syntax.Symbol("="), syntax.Hole(31),
	),
	syntax.Form(formMAC,
		syntax.Keyword("mac"), syntax.Symbol("("), syntax.Hole(40), syntax.Symbol(")"),
	),
	syntax.Form(formBracket,
		syntax.Symbol("["), syntax.Hole(20), syntax.Symbol("]"),
	),
	syntax.Form(formComma,
		syntax.Hole(20), syntax.Symbol(","), syntax.Hole(21),
	),
	syntax.Form(formTrailingComma,
		syntax.Hole(20), syntax.Symbol(","),
	),
	syntax.Form(formLeadingComma,
		syntax.Symbol(","), syntax.Hole(6),
	),
	syntax.Form(formCommaOnly,
		syntax.Symbol(","),
	),
	syntax.ImplicitForm(formMissingComma, 5, 6),
)

// parseSyntax parses one structured domain-entry list and rejects grammar
// shapes that are valid only through top-level comma compatibility.
func parseSyntax(input string) (syntax.Tree[formID], *syntax.ParseError) {
	tree, err := syntaxGrammar.Parse(input)
	if err != nil {
		return nil, err
	}
	if err := validateList(tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func validateList(tree syntax.Tree[formID]) *syntax.ParseError {
	switch tree := tree.(type) {
	case syntax.EmptyTree[formID], syntax.Atom[formID]:
		return nil
	case syntax.Op[formID]:
		//nolint:exhaustive // Unrecognized entry-list forms fall through to invalidTree below.
		switch tree.ID {
		case formFieldsEmpty:
			return requireAtom(tree.Args[0])
		case formFields:
			if err := requireAtom(tree.Args[0]); err != nil {
				return err
			}
			return validateFieldList(tree.Args[1])
		case formComma:
			if err := validateList(tree.Args[0]); err != nil {
				return err
			}
			return validateList(tree.Args[1])
		case formTrailingComma, formLeadingComma:
			return validateList(tree.Args[0])
		case formCommaOnly:
			return nil
		case formMissingComma:
			// Explicit commas bind more tightly than missing commas, so either
			// side can contain a mixed explicit/missing plain-domain list.
			if !isPlainList(tree.Args[0]) || !isPlainList(tree.Args[1]) {
				return invalidTree(tree.Args[1])
			}
			return nil
		}
	}
	return invalidTree(tree)
}

func validateFieldList(tree syntax.Tree[formID]) *syntax.ParseError {
	if tree, ok := tree.(syntax.Op[formID]); ok {
		//nolint:exhaustive // Only assignment and strict-list forms are valid in a field block.
		switch tree.ID {
		case formAssign:
			return validateAssignment(tree)
		case formComma:
			if err := validateFieldList(tree.Args[0]); err != nil {
				return err
			}
			return validateFieldList(tree.Args[1])
		case formTrailingComma:
			return validateFieldList(tree.Args[0])
		}
	}
	return invalidTree(tree)
}

func validateAssignment(tree syntax.Op[formID]) *syntax.ParseError {
	if _, ok := tree.Args[0].(syntax.Atom[formID]); !ok {
		return invalidTree(tree.Args[0])
	}
	return validateValue(tree.Args[1])
}

func validateValue(tree syntax.Tree[formID]) *syntax.ParseError {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return nil
	case syntax.Op[formID]:
		//nolint:exhaustive // Only mac calls and bracketed lists are valid structured values.
		switch tree.ID {
		case formMAC:
			return requireAtom(tree.Args[0])
		case formBracket:
			return validateValueList(tree.Args[0])
		}
	}
	return invalidTree(tree)
}

func validateValueList(tree syntax.Tree[formID]) *syntax.ParseError {
	if op, ok := tree.(syntax.Op[formID]); ok {
		//nolint:exhaustive // Only strict-list forms are interpreted as value-list structure.
		switch op.ID {
		case formComma:
			if err := validateValueList(op.Args[0]); err != nil {
				return err
			}
			return validateValueList(op.Args[1])
		case formTrailingComma:
			return validateValueList(op.Args[0])
		}
	}
	return validateValue(tree)
}

func requireAtom(tree syntax.Tree[formID]) *syntax.ParseError {
	if _, ok := tree.(syntax.Atom[formID]); ok {
		return nil
	}
	return invalidTree(tree)
}

func isPlainList(tree syntax.Tree[formID]) bool {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return true
	case syntax.Op[formID]:
		//nolint:exhaustive // Structured-entry forms make a missing-comma list non-plain.
		switch tree.ID {
		case formComma, formMissingComma:
			return isPlainList(tree.Args[0]) && isPlainList(tree.Args[1])
		case formTrailingComma, formLeadingComma:
			return isPlainList(tree.Args[0])
		case formCommaOnly:
			return true
		}
	}
	return false
}

func invalidTree(tree syntax.Tree[formID]) *syntax.ParseError {
	return &syntax.ParseError{Span: firstTree(tree).Span(), Cause: syntax.ErrUnexpectedToken}
}

// firstTree returns the first token-bearing subtree in source order.
func firstTree(tree syntax.Tree[formID]) syntax.Tree[formID] {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		return tree
	case syntax.Op[formID]:
		if len(tree.Tokens) != 0 &&
			(len(tree.Args) == 0 || tree.Tokens[0].Span.Start < tree.Args[0].Span().Start) {
			return syntax.Atom[formID]{Token: tree.Tokens[0]}
		}
		return firstTree(tree.Args[0])
	default:
		return tree
	}
}
