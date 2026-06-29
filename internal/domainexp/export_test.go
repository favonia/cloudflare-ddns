package domainexp

// ExprString exposes the unexported exprString canonical renderer to black-box
// tests in package domainexp_test. Those tests drive the exported ParseExpression
// contract and only need exprString as an inspection hook to assert the parsed
// AST round-trips to its canonical text; per docs/designs/guides/testing-boundaries.markdown
// that is the export_test.go case, not a reason to keep the tests in package domainexp.
// The direct unit tests of exprString itself stay in exprstring_internal_test.go.
func ExprString(e Expr) string { return exprString(e) }
