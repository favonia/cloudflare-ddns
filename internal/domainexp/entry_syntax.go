package domainexp

import "github.com/favonia/cloudflare-ddns/internal/syntax"

// entryFormID distinguishes the structured-domain-entry syntax tree shapes.
type entryFormID string

const (
	entryFormFieldsEmpty   entryFormID = "fields-empty"
	entryFormFields        entryFormID = "fields"
	entryFormAssign        entryFormID = "assign"
	entryFormMAC           entryFormID = "mac"
	entryFormBracket       entryFormID = "bracket"
	entryFormComma         entryFormID = "comma"
	entryFormTrailingComma entryFormID = "trailing"
	entryFormMissingComma  entryFormID = "missing"
	entryFormLeadingComma  entryFormID = "leading"
	entryFormCommaOnly     entryFormID = "comma-only"
)

//nolint:gochecknoglobals // Immutable compiled grammar shared by all parse calls.
var entrySyntaxGrammar = syntax.MustNewPratt(
	syntax.Empty[entryFormID](),
	syntax.Form(entryFormFieldsEmpty,
		syntax.Hole(40), syntax.Symbol("{"), syntax.Symbol("}"),
	),
	syntax.Form(entryFormFields,
		syntax.Hole(40), syntax.Symbol("{"), syntax.Hole(20), syntax.Symbol("}"),
	),
	syntax.Form(entryFormAssign,
		syntax.Hole(30), syntax.Symbol("="), syntax.Hole(31),
	),
	syntax.Form(entryFormMAC,
		syntax.Keyword("mac"), syntax.Symbol("("), syntax.Hole(40), syntax.Symbol(")"),
	),
	syntax.Form(entryFormBracket,
		syntax.Symbol("["), syntax.Hole(20), syntax.Symbol("]"),
	),
	syntax.Form(entryFormComma,
		syntax.Hole(20), syntax.Symbol(","), syntax.Hole(21),
	),
	syntax.Form(entryFormTrailingComma,
		syntax.Hole(20), syntax.Symbol(","),
	),
	syntax.Form(entryFormLeadingComma,
		syntax.Symbol(","), syntax.Hole(6),
	),
	syntax.Form(entryFormCommaOnly,
		syntax.Symbol(","),
	),
	syntax.ImplicitForm(entryFormMissingComma, 5, 6),
)

// parseEntrySyntax parses one structured domain-entry list and rejects grammar
// shapes that are valid only through top-level comma compatibility.
func parseEntrySyntax(input string) (syntax.Tree[entryFormID], *syntax.ParseError) {
	tree, err := entrySyntaxGrammar.Parse(input)
	if err != nil {
		return nil, err
	}
	if err := validateEntryList(tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func validateEntryList(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	switch tree := tree.(type) {
	case syntax.EmptyTree[entryFormID]:
		return nil
	case syntax.Atom[entryFormID]:
		return nil
	case syntax.Op[entryFormID]:
		switch tree.ID {
		case entryFormFieldsEmpty:
			return requireEntryAtom(tree.Args[0])
		case entryFormFields:
			if err := requireEntryAtom(tree.Args[0]); err != nil {
				return err
			}
			return validateFieldList(tree.Args[1])
		case entryFormComma:
			if err := validateEntryList(tree.Args[0]); err != nil {
				return err
			}
			return validateEntryList(tree.Args[1])
		case entryFormTrailingComma, entryFormLeadingComma:
			return validateEntryList(tree.Args[0])
		case entryFormCommaOnly:
			return nil
		case entryFormMissingComma:
			if !isPlainEntryList(tree.Args[0]) {
				return invalidEntryTree(tree.Args[0])
			}
			if _, ok := tree.Args[1].(syntax.Atom[entryFormID]); !ok {
				return invalidEntryTree(tree.Args[1])
			}
			return nil
		default:
			return invalidEntryTree(tree)
		}
	default:
		return invalidEntryTree(tree)
	}
}

func validateFieldList(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	if tree, ok := tree.(syntax.Op[entryFormID]); ok {
		//nolint:exhaustive // Only assignment and strict-list forms are valid in a field block.
		switch tree.ID {
		case entryFormAssign:
			return validateAssignment(tree)
		case entryFormComma:
			if err := validateFieldList(tree.Args[0]); err != nil {
				return err
			}
			return validateFieldList(tree.Args[1])
		case entryFormTrailingComma:
			return validateFieldList(tree.Args[0])
		}
	}
	return invalidEntryTree(tree)
}

func validateAssignment(tree syntax.Op[entryFormID]) *syntax.ParseError {
	if _, ok := tree.Args[0].(syntax.Atom[entryFormID]); !ok {
		return invalidEntryTree(tree.Args[0])
	}
	return validateEntryValue(tree.Args[1])
}

func validateEntryValue(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	switch tree := tree.(type) {
	case syntax.Atom[entryFormID]:
		return nil
	case syntax.Op[entryFormID]:
		//nolint:exhaustive // Only mac calls and bracketed lists are valid structured values.
		switch tree.ID {
		case entryFormMAC:
			return requireEntryAtom(tree.Args[0])
		case entryFormBracket:
			return validateValueList(tree.Args[0])
		}
	}
	return invalidEntryTree(tree)
}

func validateValueList(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	if op, ok := tree.(syntax.Op[entryFormID]); ok {
		//nolint:exhaustive // Only strict-list forms are interpreted as value-list structure.
		switch op.ID {
		case entryFormComma:
			if err := validateValueList(op.Args[0]); err != nil {
				return err
			}
			return validateValueList(op.Args[1])
		case entryFormTrailingComma:
			return validateValueList(op.Args[0])
		}
	}
	return validateEntryValue(tree)
}

func requireEntryAtom(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	if _, ok := tree.(syntax.Atom[entryFormID]); ok {
		return nil
	}
	return invalidEntryTree(tree)
}

func isPlainEntryList(tree syntax.Tree[entryFormID]) bool {
	switch tree := tree.(type) {
	case syntax.Atom[entryFormID]:
		return true
	case syntax.Op[entryFormID]:
		//nolint:exhaustive // Structured-entry forms make a missing-comma list non-plain.
		switch tree.ID {
		case entryFormComma, entryFormMissingComma:
			return isPlainEntryList(tree.Args[0]) && isPlainEntryList(tree.Args[1])
		case entryFormTrailingComma, entryFormLeadingComma:
			return isPlainEntryList(tree.Args[0])
		case entryFormCommaOnly:
			return true
		}
	}
	return false
}

func invalidEntryTree(tree syntax.Tree[entryFormID]) *syntax.ParseError {
	return &syntax.ParseError{Span: tree.Span(), Cause: syntax.ErrUnexpectedToken}
}
