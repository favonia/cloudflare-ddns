package domainexp

// This file holds the advisory linter for PROXIED expressions. It walks the
// parsed Expr AST and emits warnings for suspicious-but-valid shapes. It never
// rejects an expression and never changes evaluation.
//
// The shape pass (R1, R2) below uses only Boolean structure and atom polarity;
// it carries no is/sub semantics. That is the seam a future shared linter would
// extract. The semantic pass (R3, R4) lives in lint_semantic.go.

import "github.com/favonia/cloudflare-ddns/internal/pp"

// finding is one advisory lint result. The set is closed to this package, so a
// finding reports itself rather than being matched with errors.As. message
// returns the full operator-facing line; LintExpression deduplicates on it.
type finding interface {
	message(key, input string) string
}

// LintExpression emits advisory warnings for suspicious PROXIED expressions.
// It assumes expr already parsed successfully. It is purely advisory: callers
// continue to build config and evaluate the expression unchanged.
func LintExpression(ppfmt pp.PP, key, input string, expr Expr) {
	shape := shapeFindings(expr)
	semantic := semanticFindings(expr)
	findings := make([]finding, 0, len(shape)+len(semantic))
	findings = append(findings, shape...)
	findings = append(findings, semantic...)

	seen := map[string]bool{}
	for _, f := range findings {
		msg := f.message(key, input)
		if seen[msg] {
			continue
		}
		seen[msg] = true
		// Pass msg as an argument, not as the format, so a % in user input is safe.
		ppfmt.Noticef(pp.EmojiUserWarning, "%s", msg)
	}
}

// redundantNegationFinding is R1: a ! applied to another ! or to a constant.
type redundantNegationFinding struct {
	suggestion string // canonical text of the equivalent simpler expression
	constant   bool   // true if the ! was applied to a Boolean constant
}

// exclusionOnlyDisjunctFinding is R2: an || branch with no positive atom.
type exclusionOnlyDisjunctFinding struct {
	branch string // canonical text of the offending branch
}

func (f exclusionOnlyDisjunctFinding) message(key, input string) string {
	return key + ` ("` + input + `") has an || branch "` + f.branch +
		`" with no included domain, only exclusions; it usually matches far more than intended`
}

// flatten returns the operands of the maximal chain of op rooted at e. For a
// non-op node it returns just that node.
func flatten(e Expr, op formID) []Expr {
	b, ok := e.(binaryExpr)
	if !ok || b.operator != op {
		return []Expr{e}
	}
	return append(flatten(b.left, op), flatten(b.right, op)...)
}

// hasAnyAtom reports whether e contains at least one is(...)/sub(...) atom.
// Boolean constants are not atoms.
func hasAnyAtom(e Expr, _ bool) bool {
	switch e := e.(type) {
	case callExpr:
		return true
	case unaryExpr:
		return hasAnyAtom(e.operand, false)
	case binaryExpr:
		return hasAnyAtom(e.left, false) || hasAnyAtom(e.right, false)
	default:
		return false
	}
}

// hasPositiveAtom reports whether e contains an is(...)/sub(...) atom under an
// even number of negations. neg tracks the parity so far. A Boolean constant is
// treated as positive so that R2 never fires on a branch containing a constant
// (those cases are R3/R4 instead).
func hasPositiveAtom(e Expr, neg bool) bool {
	switch e := e.(type) {
	case callExpr:
		return !neg
	case literalExpr:
		return true
	case unaryExpr:
		return hasPositiveAtom(e.operand, !neg)
	case binaryExpr:
		return hasPositiveAtom(e.left, neg) || hasPositiveAtom(e.right, neg)
	default:
		return false
	}
}

func (f redundantNegationFinding) message(key, input string) string {
	if f.constant {
		return key + ` ("` + input + `") negates a constant; "` + f.suggestion + `" means the same thing`
	}
	return key + ` ("` + input + `") negates a negation, which has no effect; "` + f.suggestion + `" means the same thing`
}

func shapeFindings(expr Expr) []finding {
	var findings []finding
	walk(expr, func(e Expr) {
		u, ok := e.(unaryExpr)
		if !ok {
			return
		}
		switch inner := u.operand.(type) {
		case unaryExpr:
			// !!X cancels to inner.operand.
			findings = append(findings, redundantNegationFinding{
				suggestion: exprString(inner.operand),
				constant:   false,
			})
		case literalExpr:
			// !true -> false, !false -> true.
			findings = append(findings, redundantNegationFinding{
				suggestion: exprString(literalExpr{value: !inner.value}),
				constant:   true,
			})
		}
	})

	// R2: every || branch should include at least one positive atom.
	walk(expr, func(e Expr) {
		b, ok := e.(binaryExpr)
		if !ok || b.operator != formOr {
			return
		}
		for _, branch := range flatten(e, formOr) {
			if hasAnyAtom(branch, false) && !hasPositiveAtom(branch, false) {
				findings = append(findings, exclusionOnlyDisjunctFinding{branch: exprString(branch)})
			}
		}
	})

	return findings
}

// walk visits every node of e in pre-order.
func walk(e Expr, visit func(Expr)) {
	visit(e)
	switch e := e.(type) {
	case unaryExpr:
		walk(e.operand, visit)
	case binaryExpr:
		walk(e.left, visit)
		walk(e.right, visit)
	}
}
