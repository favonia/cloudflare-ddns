package domainexp

import (
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

	state := entryBuildState{
		entries:      nil,
		diagnostics:  nil,
		extraComma:   false,
		missingComma: false,
	}
	state.buildEntryList(tree)
	return state.entries, state.diagnostics, nil
}

type entryBuildState struct {
	entries      []Entry
	diagnostics  []EntryDiagnostic
	extraComma   bool
	missingComma bool
}

// buildEntryList reports whether conversion failed semantically. Explicit
// top-level commas are the only points where conversion resumes after failure.
func (state *entryBuildState) buildEntryList(tree syntax.Tree[entryFormID]) bool {
	switch tree := tree.(type) {
	case syntax.EmptyTree[entryFormID]:
		return false
	case syntax.Atom[entryFormID]:
		entry, diagnostic := buildEntry(tree)
		if diagnostic != nil {
			state.diagnostics = append(state.diagnostics, *diagnostic)
			return true
		}
		state.entries = append(state.entries, entry)
		return false
	case syntax.Op[entryFormID]:
		//nolint:exhaustive // Only top-level entry-list forms are valid here.
		switch tree.ID {
		case entryFormFieldsEmpty, entryFormFields:
			entry, diagnostic := buildEntry(tree)
			if diagnostic != nil {
				state.diagnostics = append(state.diagnostics, *diagnostic)
				return true
			}
			state.entries = append(state.entries, entry)
			return false
		case entryFormComma:
			leftFailed := state.buildEntryList(tree.Args[0])
			rightFailed := state.buildEntryList(tree.Args[1])
			return leftFailed || rightFailed
		case entryFormMissingComma:
			leftFailed := state.buildEntryList(tree.Args[0])
			state.recordMissingComma(syntax.Span{
				Start: tree.Args[0].Span().End,
				End:   tree.Args[1].Span().Start,
			})
			if leftFailed {
				return true
			}
			return state.buildEntryList(tree.Args[1])
		case entryFormTrailingComma:
			return state.buildEntryList(tree.Args[0])
		case entryFormLeadingComma:
			state.recordExtraComma(tree.Tokens[0].Span)
			return state.buildEntryList(tree.Args[0])
		case entryFormCommaOnly:
			state.recordExtraComma(tree.Tokens[0].Span)
			return false
		}
	}

	panic("domainexp: invalid parsed entry-list tree; this should not happen; please report it")
}

func (state *entryBuildState) recordExtraComma(span syntax.Span) {
	if state.extraComma {
		return
	}
	state.extraComma = true
	state.diagnostics = append(state.diagnostics, EntryDiagnostic{Span: span, Cause: ErrExtraComma})
}

func (state *entryBuildState) recordMissingComma(span syntax.Span) {
	if state.missingComma {
		return
	}
	state.missingComma = true
	state.diagnostics = append(state.diagnostics, EntryDiagnostic{Span: span, Cause: ErrMissingComma})
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
