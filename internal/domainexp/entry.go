package domainexp

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// Entry is one parsed domain declaration.
type Entry struct {
	Domain          domain.Domain
	HostID6Opinions []hostid6.Set
	Span            syntax.Span
}

// EntryDiagnostic describes one semantic failure in a parsed domain entry.
type EntryDiagnostic struct {
	Span  syntax.Span
	Cause error
}

// ParseEntries parses structured domain entries without merging declarations or assignments.
func ParseEntries(input string) ([]Entry, []EntryDiagnostic, *syntax.ParseError) {
	tree, err := parseEntrySyntax(input)
	if err != nil {
		return nil, nil, err
	}

	entries, diagnostics, _ := buildEntryList(tree)
	return entries, deduplicateCompatibilityDiagnostics(diagnostics), nil
}

// buildEntryList reports whether conversion failed semantically. Explicit
// top-level commas are the only points where conversion resumes after failure.
func buildEntryList(tree syntax.Tree[entryFormID]) ([]Entry, []EntryDiagnostic, bool) {
	switch tree := tree.(type) {
	case syntax.EmptyTree[entryFormID]:
		return nil, nil, false
	case syntax.Atom[entryFormID]:
		entry, diagnostic := buildEntry(tree)
		if diagnostic != nil {
			return nil, []EntryDiagnostic{*diagnostic}, true
		}
		return []Entry{entry}, nil, false
	case syntax.Op[entryFormID]:
		//nolint:exhaustive // Only top-level entry-list forms are valid here.
		switch tree.ID {
		case entryFormFieldsEmpty, entryFormFields:
			entry, diagnostic := buildEntry(tree)
			if diagnostic != nil {
				return nil, []EntryDiagnostic{*diagnostic}, true
			}
			return []Entry{entry}, nil, false
		case entryFormComma:
			leftEntries, leftDiagnostics, leftFailed := buildEntryList(tree.Args[0])
			rightEntries, rightDiagnostics, rightFailed := buildEntryList(tree.Args[1])
			return append(leftEntries, rightEntries...),
				append(leftDiagnostics, rightDiagnostics...),
				leftFailed || rightFailed
		case entryFormMissingComma:
			leftEntries, leftDiagnostics, leftFailed := buildEntryList(tree.Args[0])
			leftDiagnostics = append(leftDiagnostics, EntryDiagnostic{
				Span: syntax.Span{
					Start: tree.Args[0].Span().End,
					End:   tree.Args[1].Span().Start,
				},
				Cause: ErrMissingComma,
			})
			if leftFailed {
				return leftEntries, leftDiagnostics, true
			}
			rightEntries, rightDiagnostics, rightFailed := buildEntryList(tree.Args[1])
			return append(leftEntries, rightEntries...), append(leftDiagnostics, rightDiagnostics...), rightFailed
		case entryFormTrailingComma:
			return buildEntryList(tree.Args[0])
		case entryFormLeadingComma:
			entries, diagnostics, failed := buildEntryList(tree.Args[0])
			return entries, append([]EntryDiagnostic{{
				Span: tree.Tokens[0].Span, Cause: ErrExtraComma,
			}}, diagnostics...), failed
		case entryFormCommaOnly:
			return nil, []EntryDiagnostic{{
				Span: tree.Tokens[0].Span, Cause: ErrExtraComma,
			}}, false
		}
	}

	panic("domainexp: invalid parsed entry-list tree; this should not happen; please report it")
}

func deduplicateCompatibilityDiagnostics(diagnostics []EntryDiagnostic) []EntryDiagnostic {
	result := make([]EntryDiagnostic, 0, len(diagnostics))
	extraComma := false
	missingComma := false
	for _, diagnostic := range diagnostics {
		switch {
		case errors.Is(diagnostic.Cause, ErrExtraComma):
			if extraComma {
				continue
			}
			extraComma = true
		case errors.Is(diagnostic.Cause, ErrMissingComma):
			if missingComma {
				continue
			}
			missingComma = true
		}
		result = append(result, diagnostic)
	}
	return result
}

func buildEntry(tree syntax.Tree[entryFormID]) (Entry, *EntryDiagnostic) {
	var domainTree syntax.Tree[entryFormID]
	var fieldsTree syntax.Tree[entryFormID]

	switch tree := tree.(type) {
	case syntax.Atom[entryFormID]:
		domainTree = tree
	case syntax.Op[entryFormID]:
		domainTree = tree.Args[0]
		if tree.ID == entryFormFields {
			fieldsTree = tree.Args[1]
		}
	default:
		panic("domainexp: invalid parsed entry tree; this should not happen; please report it")
	}

	domainAtom := mustEntryAtom(domainTree)
	dom, err := domain.New(domainAtom.Token.Text)
	if err != nil {
		var noEntry Entry
		return noEntry, &EntryDiagnostic{
			Span:  domainAtom.Span(),
			Cause: fmt.Errorf("%w: %w", ErrInvalidDomain, err),
		}
	}

	opinions, diagnostic := buildFields(fieldsTree)
	if diagnostic != nil {
		var noEntry Entry
		return noEntry, diagnostic
	}
	return Entry{Domain: dom, HostID6Opinions: opinions, Span: tree.Span()}, nil
}

func buildFields(tree syntax.Tree[entryFormID]) ([]hostid6.Set, *EntryDiagnostic) {
	if tree == nil {
		return nil, nil
	}

	op := mustEntryOp(tree)
	switch op.ID {
	case entryFormAssign:
		opinion, diagnostic := buildAssignment(op)
		if diagnostic != nil {
			return nil, diagnostic
		}
		return []hostid6.Set{opinion}, nil
	case entryFormComma:
		left, diagnostic := buildFields(op.Args[0])
		if diagnostic != nil {
			return nil, diagnostic
		}
		right, diagnostic := buildFields(op.Args[1])
		return append(left, right...), diagnostic
	case entryFormTrailingComma:
		return buildFields(op.Args[0])
	default:
		panic("domainexp: invalid parsed field-list tree; this should not happen; please report it")
	}
}

func buildAssignment(tree syntax.Op[entryFormID]) (hostid6.Set, *EntryDiagnostic) {
	field := mustEntryAtom(tree.Args[0])
	if field.Token.Text != "hostid6" {
		return hostid6.Set{}, &EntryDiagnostic{
			Span:  field.Span(),
			Cause: ErrUnknownDomainField,
		}
	}

	values, diagnostic := buildHostID6Values(tree.Args[1])
	if diagnostic != nil {
		return hostid6.Set{}, diagnostic
	}
	if len(values) == 0 {
		return hostid6.Set{}, &EntryDiagnostic{
			Span:  tree.Args[1].Span(),
			Cause: ErrEmptyHostID6Set,
		}
	}
	return hostid6.NewSet(values...), nil
}

func buildHostID6Values(tree syntax.Tree[entryFormID]) ([]hostid6.Derivation, *EntryDiagnostic) {
	switch tree := tree.(type) {
	case syntax.Atom[entryFormID]:
		if tree.Token.Text == "preserve" {
			return []hostid6.Derivation{hostid6.Preserve()}, nil
		}

		addr, err := netip.ParseAddr(tree.Token.Text)
		if err == nil {
			var derivation hostid6.Derivation
			derivation, err = hostid6.Literal(addr)
			if err == nil {
				return []hostid6.Derivation{derivation}, nil
			}
		}
		return nil, &EntryDiagnostic{
			Span:  tree.Span(),
			Cause: fmt.Errorf("%w: %w", ErrInvalidHostID6, err),
		}
	case syntax.Op[entryFormID]:
		//nolint:exhaustive // Only structured host-ID values are valid here.
		switch tree.ID {
		case entryFormMAC:
			atom := mustEntryAtom(tree.Args[0])
			mac, err := hostid6.ParseMAC(atom.Token.Text)
			if err != nil {
				return nil, &EntryDiagnostic{
					Span:  atom.Span(),
					Cause: fmt.Errorf("%w: %w", ErrInvalidMAC, err),
				}
			}
			return []hostid6.Derivation{hostid6.MAC(mac)}, nil
		case entryFormBracket:
			return buildHostID6ValueList(tree.Args[0])
		}
	}

	panic("domainexp: invalid parsed hostid6 value tree; this should not happen; please report it")
}

func buildHostID6ValueList(tree syntax.Tree[entryFormID]) ([]hostid6.Derivation, *EntryDiagnostic) {
	if op, ok := tree.(syntax.Op[entryFormID]); ok {
		//nolint:exhaustive // Only strict value-list forms are interpreted here.
		switch op.ID {
		case entryFormComma:
			left, diagnostic := buildHostID6ValueList(op.Args[0])
			if diagnostic != nil {
				return nil, diagnostic
			}
			right, diagnostic := buildHostID6ValueList(op.Args[1])
			return append(left, right...), diagnostic
		case entryFormTrailingComma:
			return buildHostID6ValueList(op.Args[0])
		}
	}
	return buildHostID6Values(tree)
}

func mustEntryAtom(tree syntax.Tree[entryFormID]) syntax.Atom[entryFormID] {
	atom, ok := tree.(syntax.Atom[entryFormID])
	if !ok {
		panic("domainexp: parsed entry tree requires an atom; this should not happen; please report it")
	}
	return atom
}

func mustEntryOp(tree syntax.Tree[entryFormID]) syntax.Op[entryFormID] {
	op, ok := tree.(syntax.Op[entryFormID])
	if !ok {
		panic("domainexp: parsed entry tree requires an operation; this should not happen; please report it")
	}
	return op
}
