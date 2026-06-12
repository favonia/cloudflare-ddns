package syntax

// Tree is a parse-tree node. It is an [EmptyTree], [Atom], or [Op].
type Tree[ID any] interface {
	// tree is unexported to seal the interface.
	tree(_ ID)
	// Span returns the byte range of this node in the source input.
	Span() Span
}

// EmptyTree represents successfully parsed empty input.
type EmptyTree[ID any] struct{}

// Span returns the empty source range.
func (EmptyTree[ID]) Span() Span {
	return Span{Start: 0, End: 0}
}

func (EmptyTree[ID]) tree(_ ID) {}

// Atom is a leaf node representing a single token that was not matched by any [Rule].
type Atom[ID any] struct {
	// Token is the source token for this leaf.
	Token Token
}

// Span returns the source range of the atom's token.
func (atom Atom[ID]) Span() Span {
	return atom.Token.Span
}

func (Atom[ID]) tree(_ ID) {}

// Op is an operator node produced by matching a [Form].
type Op[ID any] struct {
	// ID is the identifier of the matched [Rule].
	ID ID
	// Tokens contains all literal (non-hole) tokens matched by the form's pattern, in pattern order.
	Tokens []Token
	// Args contains the sub-expressions produced by the holes in the form's pattern, in pattern order.
	Args []Tree[ID]
	span Span
}

// newOp constructs an operator node from tokens and arguments already collected
// in pattern order.
func newOp[ID any](id ID, tokens []Token, span Span, args ...Tree[ID]) Op[ID] {
	return Op[ID]{
		ID:     id,
		Tokens: tokens,
		Args:   args,
		span:   span,
	}
}

// Span returns the full source range matched by the operation.
func (op Op[ID]) Span() Span {
	return op.span
}

func (Op[ID]) tree(_ ID) {}
