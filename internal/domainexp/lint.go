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

// semanticFindings runs the is/sub pass (R3, R4). Filled in by later tasks.
func semanticFindings(Expr) []finding { return nil }
