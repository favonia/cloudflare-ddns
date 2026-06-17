// Package domainentry parses structured domain declarations such as
// "example.com{hostid6=...}".
//
// Unlike the operator-facing parsers in internal/domainexp, which report
// problems directly through a pp.PP as they parse, this package returns
// structured Diagnostic values for the caller to render. It therefore does not
// depend on internal/pp.
package domainentry

import (
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

// DiagnosticKind classifies a semantic failure in a parsed domain entry. It is
// a plain enum rather than an error so that classification stays separate from
// the underlying detail: the detail (when any) lives in Diagnostic.Detail, and
// Description can switch exhaustively over the kind without unwrapping anything.
type DiagnosticKind int

const (
	// KindInvalidDomain reports a malformed or non-fully-qualified domain.
	KindInvalidDomain DiagnosticKind = iota
	// KindUnknownDomainField reports an unsupported structured-domain field.
	KindUnknownDomainField
	// KindInvalidHostID6 reports an invalid IPv6 host-ID literal.
	KindInvalidHostID6
	// KindInvalidMAC reports an invalid 48-bit MAC address.
	KindInvalidMAC
	// KindExtraComma reports extra top-level commas accepted for compatibility.
	KindExtraComma
	// KindMissingComma reports missing top-level commas accepted for compatibility.
	KindMissingComma
)

// Entry is one parsed domain declaration.
type Entry struct {
	Domain          domain.Domain
	HostID6Opinions []hostid6.Set
	Span            syntax.Span
}

// Diagnostic describes one semantic failure in a parsed domain entry. Kind is
// the classification; Detail carries the underlying error for kinds that have
// one (it is nil for KindUnknownDomainField and the comma kinds).
type Diagnostic struct {
	Span   syntax.Span
	Kind   DiagnosticKind
	Detail error
}

// Description renders the source-specific semantic failure without setting context.
func (diagnostic Diagnostic) Description(input string) string {
	source := input[diagnostic.Span.Start:diagnostic.Span.End]

	switch diagnostic.Kind {
	case KindInvalidDomain:
		return fmt.Sprintf("invalid domain %q: %v", source, diagnostic.Detail)
	case KindUnknownDomainField:
		return fmt.Sprintf("unknown domain field %q", source)
	case KindInvalidHostID6:
		return fmt.Sprintf("invalid hostid6 value %q: %v", source, diagnostic.Detail)
	case KindInvalidMAC:
		return fmt.Sprintf("invalid hostid6 MAC address %q: %v", source, diagnostic.Detail)
	case KindExtraComma:
		return "extra comma"
	case KindMissingComma:
		return "missing comma"
	}

	panic("domainentry: unknown diagnostic kind; this should not happen; please report it")
}

// Parse parses structured domain entries without merging declarations or assignments.
func Parse(input string) ([]Entry, []Diagnostic, *syntax.ParseError) {
	tree, err := parseSyntax(input)
	if err != nil {
		return nil, nil, err
	}

	state := buildState{
		entries:      nil,
		diagnostics:  nil,
		extraComma:   false,
		missingComma: false,
	}
	state.buildList(tree)
	return state.entries, state.diagnostics, nil
}

type buildState struct {
	entries      []Entry
	diagnostics  []Diagnostic
	extraComma   bool
	missingComma bool
}

// buildList reports whether conversion failed semantically. Explicit
// top-level commas are the only points where conversion resumes after failure.
func (state *buildState) buildList(tree syntax.Tree[formID]) bool {
	switch tree := tree.(type) {
	case syntax.EmptyTree[formID]:
		return false
	case syntax.Atom[formID]:
		entry, diagnostic := buildEntry(tree)
		if diagnostic != nil {
			state.diagnostics = append(state.diagnostics, *diagnostic)
			return true
		}
		state.entries = append(state.entries, entry)
		return false
	case syntax.Op[formID]:
		//nolint:exhaustive // Only top-level entry-list forms are valid here.
		switch tree.ID {
		case formFieldsEmpty, formFields:
			entry, diagnostic := buildEntry(tree)
			if diagnostic != nil {
				state.diagnostics = append(state.diagnostics, *diagnostic)
				return true
			}
			state.entries = append(state.entries, entry)
			return false
		case formComma:
			leftFailed := state.buildList(tree.Args[0])
			rightFailed := state.buildList(tree.Args[1])
			return leftFailed || rightFailed
		case formMissingComma:
			leftFailed := state.buildList(tree.Args[0])
			state.recordMissingComma(syntax.Span{
				Start: tree.Args[0].Span().End,
				End:   tree.Args[1].Span().Start,
			})
			if leftFailed {
				return true
			}
			return state.buildList(tree.Args[1])
		case formTrailingComma:
			return state.buildList(tree.Args[0])
		case formLeadingComma:
			state.recordExtraComma(tree.Tokens[0].Span)
			return state.buildList(tree.Args[0])
		case formCommaOnly:
			state.recordExtraComma(tree.Tokens[0].Span)
			return false
		}
	}

	panic("domainentry: invalid parsed entry-list tree; this should not happen; please report it")
}

func (state *buildState) recordExtraComma(span syntax.Span) {
	if state.extraComma {
		return
	}
	state.extraComma = true
	state.diagnostics = append(state.diagnostics, Diagnostic{Span: span, Kind: KindExtraComma, Detail: nil})
}

func (state *buildState) recordMissingComma(span syntax.Span) {
	if state.missingComma {
		return
	}
	state.missingComma = true
	state.diagnostics = append(state.diagnostics, Diagnostic{Span: span, Kind: KindMissingComma, Detail: nil})
}

func buildEntry(tree syntax.Tree[formID]) (Entry, *Diagnostic) {
	var domainTree syntax.Tree[formID]
	var fieldsTree syntax.Tree[formID]

	switch tree := tree.(type) {
	case syntax.Atom[formID]:
		domainTree = tree
	case syntax.Op[formID]:
		domainTree = tree.Args[0]
		if tree.ID == formFields {
			fieldsTree = tree.Args[1]
		}
	default:
		panic("domainentry: invalid parsed entry tree; this should not happen; please report it")
	}

	domainAtom := mustAtom(domainTree)
	dom, err := domain.New(domainAtom.Token.Text)
	if err != nil {
		var noEntry Entry
		return noEntry, &Diagnostic{
			Span:   domainAtom.Span(),
			Kind:   KindInvalidDomain,
			Detail: err,
		}
	}

	opinions, diagnostic := buildFields(fieldsTree)
	if diagnostic != nil {
		var noEntry Entry
		return noEntry, diagnostic
	}
	return Entry{Domain: dom, HostID6Opinions: opinions, Span: tree.Span()}, nil
}

func buildFields(tree syntax.Tree[formID]) ([]hostid6.Set, *Diagnostic) {
	if tree == nil {
		return nil, nil
	}

	op := mustOp(tree)
	switch op.ID {
	case formAssign:
		opinion, diagnostic := buildAssignment(op)
		if diagnostic != nil {
			return nil, diagnostic
		}
		return []hostid6.Set{opinion}, nil
	case formComma:
		left, diagnostic := buildFields(op.Args[0])
		if diagnostic != nil {
			return nil, diagnostic
		}
		right, diagnostic := buildFields(op.Args[1])
		return append(left, right...), diagnostic
	case formTrailingComma:
		return buildFields(op.Args[0])
	default:
		panic("domainentry: invalid parsed field-list tree; this should not happen; please report it")
	}
}

func buildAssignment(tree syntax.Op[formID]) (hostid6.Set, *Diagnostic) {
	field := mustAtom(tree.Args[0])
	if field.Token.Text != "hostid6" {
		return hostid6.Set{}, &Diagnostic{
			Span:   field.Span(),
			Kind:   KindUnknownDomainField,
			Detail: nil,
		}
	}

	values, diagnostic := buildHostID6Values(tree.Args[1])
	if diagnostic != nil {
		return hostid6.Set{}, diagnostic
	}
	return hostid6.NewSet(values...), nil
}

func buildHostID6Values(tree syntax.Tree[formID]) ([]hostid6.Derivation, *Diagnostic) {
	switch tree := tree.(type) {
	case syntax.Atom[formID]:
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
		return nil, &Diagnostic{
			Span:   tree.Span(),
			Kind:   KindInvalidHostID6,
			Detail: err,
		}
	case syntax.Op[formID]:
		//nolint:exhaustive // Only structured host-ID values are valid here.
		switch tree.ID {
		case formMAC:
			atom := mustAtom(tree.Args[0])
			mac, err := hostid6.ParseMAC(atom.Token.Text)
			if err != nil {
				return nil, &Diagnostic{
					Span:   atom.Span(),
					Kind:   KindInvalidMAC,
					Detail: err,
				}
			}
			return []hostid6.Derivation{hostid6.MAC(mac)}, nil
		case formBracket:
			return buildHostID6ValueList(tree.Args[0])
		}
	}

	panic("domainentry: invalid parsed hostid6 value tree; this should not happen; please report it")
}

func buildHostID6ValueList(tree syntax.Tree[formID]) ([]hostid6.Derivation, *Diagnostic) {
	if op, ok := tree.(syntax.Op[formID]); ok {
		//nolint:exhaustive // Only strict value-list forms are interpreted here.
		switch op.ID {
		case formComma:
			left, diagnostic := buildHostID6ValueList(op.Args[0])
			if diagnostic != nil {
				return nil, diagnostic
			}
			right, diagnostic := buildHostID6ValueList(op.Args[1])
			return append(left, right...), diagnostic
		case formTrailingComma:
			return buildHostID6ValueList(op.Args[0])
		}
	}
	return buildHostID6Values(tree)
}

func mustAtom(tree syntax.Tree[formID]) syntax.Atom[formID] {
	atom, ok := tree.(syntax.Atom[formID])
	if !ok {
		panic("domainentry: parsed entry tree requires an atom; this should not happen; please report it")
	}
	return atom
}

func mustOp(tree syntax.Tree[formID]) syntax.Op[formID] {
	op, ok := tree.(syntax.Op[formID])
	if !ok {
		panic("domainentry: parsed entry tree requires an operation; this should not happen; please report it")
	}
	return op
}
