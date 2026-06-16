// Package domainentry parses structured domain declarations such as
// "example.com{hostid6=...}".
//
// Unlike the operator-facing parsers in internal/domainexp, which report
// problems directly through a pp.PP as they parse, this package returns
// structured Diagnostic values for the caller to render. It therefore does not
// depend on internal/pp.
package domainentry

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/hostid6"
	"github.com/favonia/cloudflare-ddns/internal/syntax"
)

var (
	// ErrInvalidHostID6 reports an invalid IPv6 host-ID literal.
	ErrInvalidHostID6 = errors.New("invalid IPv6 host ID")
	// ErrInvalidMAC reports an invalid 48-bit MAC address.
	ErrInvalidMAC = errors.New("invalid 48-bit MAC address")
	// ErrInvalidDomain reports a malformed or non-fully-qualified domain.
	ErrInvalidDomain = errors.New("invalid domain")
	// ErrUnknownDomainField reports an unsupported structured-domain field.
	ErrUnknownDomainField = errors.New("unknown domain field")
	// ErrExtraComma reports extra top-level commas accepted for compatibility.
	ErrExtraComma = errors.New("extra comma")
	// ErrMissingComma reports missing top-level commas accepted for compatibility.
	ErrMissingComma = errors.New("missing comma")
)

// Entry is one parsed domain declaration.
type Entry struct {
	Domain          domain.Domain
	HostID6Opinions []hostid6.Set
	Span            syntax.Span
}

// Diagnostic describes one semantic failure in a parsed domain entry.
type Diagnostic struct {
	Span  syntax.Span
	Cause error
}

// Description renders the source-specific semantic failure without setting context.
func (diagnostic Diagnostic) Description(input string) string {
	source := input[diagnostic.Span.Start:diagnostic.Span.End]
	detail := diagnostic.Cause
	if causes, ok := diagnostic.Cause.(interface{ Unwrap() []error }); ok {
		unwrapped := causes.Unwrap()
		if len(unwrapped) > 1 {
			detail = unwrapped[1]
		}
	}

	switch {
	case errors.Is(diagnostic.Cause, ErrInvalidDomain):
		return fmt.Sprintf("invalid domain %q: %v", source, detail)
	case errors.Is(diagnostic.Cause, ErrUnknownDomainField):
		return fmt.Sprintf("unknown domain field %q", source)
	case errors.Is(diagnostic.Cause, ErrInvalidHostID6):
		return fmt.Sprintf("invalid hostid6 value %q: %v", source, detail)
	case errors.Is(diagnostic.Cause, ErrInvalidMAC):
		return fmt.Sprintf("invalid hostid6 MAC address %q: %v", source, detail)
	default:
		return diagnostic.Cause.Error()
	}
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
	state.diagnostics = append(state.diagnostics, Diagnostic{Span: span, Cause: ErrExtraComma})
}

func (state *buildState) recordMissingComma(span syntax.Span) {
	if state.missingComma {
		return
	}
	state.missingComma = true
	state.diagnostics = append(state.diagnostics, Diagnostic{Span: span, Cause: ErrMissingComma})
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
			Span:  tree.Span(),
			Cause: fmt.Errorf("%w: %w", ErrInvalidHostID6, err),
		}
	case syntax.Op[formID]:
		//nolint:exhaustive // Only structured host-ID values are valid here.
		switch tree.ID {
		case formMAC:
			atom := mustAtom(tree.Args[0])
			mac, err := hostid6.ParseMAC(atom.Token.Text)
			if err != nil {
				return nil, &Diagnostic{
					Span:  atom.Span(),
					Cause: fmt.Errorf("%w: %w", ErrInvalidMAC, err),
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
