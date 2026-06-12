package syntax

type prattParser[ID any] struct {
	tokens []Token
	// pos indexes the next token to read. It always equals the number of
	// consumed real tokens and never goes past the trailing EOF token, so
	// comparing positions compares how much input was matched.
	pos   int
	pratt Pratt[ID]
}

// Parse parses input into a [Tree].
func (pratt Pratt[ID]) Parse(input string) (Tree[ID], *ParseError) {
	tokens, err := pratt.tokenizer.tokenize(input)
	if err != nil {
		return nil, err
	}
	if tokens[0].isEOF() {
		if pratt.empty {
			return EmptyTree[ID]{}, nil
		}
		return nil, newParseError(tokens[0].Span, ErrUnexpectedEOF)
	}
	parser := prattParser[ID]{tokens: tokens, pos: 0, pratt: pratt}
	tree, err := parser.parseExpression(0)
	if err != nil {
		return nil, err
	}
	if !parser.peek().isEOF() {
		return nil, parser.newParseError(parser.peek().Span, ErrUnexpectedToken)
	}
	return tree, nil
}

func (parser *prattParser[ID]) peek() Token {
	return parser.tokens[parser.pos]
}

// consume returns the current token and advances past it, except that EOF is
// never consumed, maintaining the invariant documented on pos.
func (parser *prattParser[ID]) consume() Token {
	token := parser.peek()
	if !token.isEOF() {
		parser.pos++
	}
	return token
}

func (parser *prattParser[ID]) newParseError(span Span, cause error) *ParseError {
	err := newParseError(span, cause)
	err.progress = parser.pos
	return err
}

// parseExpression parses one null form followed by every eligible left form.
func (parser *prattParser[ID]) parseExpression(minBindingPower int) (Tree[ID], *ParseError) {
	token := parser.consume()
	left, err := parser.parseNull(token)
	if err != nil {
		return nil, err
	}
	for {
		forms := parser.selectLeftForms(minBindingPower)
		if len(forms) == 0 {
			return left, nil
		}
		var next Tree[ID]
		matched := false
		var farthestErr *ParseError
		for _, form := range forms {
			// Forms sharing a leading token are alternatives in declaration order.
			checkpoint := parser.pos
			next, err = parser.parseLeftForm(form, left)
			if err == nil {
				left = next
				matched = true
				break
			}
			farthestErr = fartherError(farthestErr, err)
			parser.pos = checkpoint
		}
		if !matched {
			return nil, farthestErr
		}
	}
}

// parseNull parses an expression beginning with token, trying alternative forms
// in declaration order and preserving the failure that reached farthest.
func (parser *prattParser[ID]) parseNull(token Token) (Tree[ID], *ParseError) {
	candidates := parser.pratt.nullRules[keyForToken(token)]
	if len(candidates) != 0 {
		var farthestErr *ParseError
		for _, form := range candidates {
			// Preserve the error from the alternative that consumed the most input.
			checkpoint := parser.pos
			tree, err := parser.parseNullForm(form, token)
			if err == nil {
				return tree, nil
			}
			farthestErr = fartherError(farthestErr, err)
			parser.pos = checkpoint
		}
		return nil, farthestErr
	}
	if token.isAtom() {
		return Atom[ID]{Token: token}, nil
	}
	if token.isEOF() {
		return nil, parser.newParseError(token.Span, ErrUnexpectedEOF)
	}
	return nil, parser.newParseError(token.Span, ErrUnexpectedToken)
}

// fartherError preserves the failure farthest into the input, using declaration
// order to break ties. Recorded progress survives nested backtracking.
func fartherError(farthest, candidate *ParseError) *ParseError {
	if farthest == nil || candidate.progress > farthest.progress {
		return candidate
	}
	return farthest
}

// selectLeftForms returns eligible explicit forms, or implicit forms only when
// no explicit form matches and the next token can start an expression.
func (parser *prattParser[ID]) selectLeftForms(minBindingPower int) []Rule[ID] {
	candidates := parser.pratt.leftRules[keyForToken(parser.peek())]
	forms := make([]Rule[ID], 0, len(candidates))
	for _, form := range candidates {
		if form.pattern[0].bindingPower >= minBindingPower {
			forms = append(forms, form)
		}
	}
	if len(forms) != 0 || !parser.canStartExpression(parser.peek()) {
		return forms
	}
	for _, form := range parser.pratt.implicitRules {
		if form.pattern[0].bindingPower >= minBindingPower {
			forms = append(forms, form)
		}
	}
	return forms
}

// canStartExpression reports whether token can begin an atom or null form.
func (parser *prattParser[ID]) canStartExpression(token Token) bool {
	return token.isAtom() || len(parser.pratt.nullRules[keyForToken(token)]) != 0
}

// parseFormTail parses form.pattern[1:], extending the leading tokens, span,
// and arguments contributed by the form's first part.
func (parser *prattParser[ID]) parseFormTail(
	form Rule[ID], tokens []Token, span Span, args []Tree[ID],
) (Tree[ID], *ParseError) {
	for _, part := range form.pattern[1:] {
		if part.kind == partHole {
			child, err := parser.parseExpression(part.bindingPower)
			if err != nil {
				return nil, err
			}
			args = append(args, child)
			span = fromTo(span, child.Span())
			continue
		}
		token, err := parser.expect(part)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		span = fromTo(span, token.Span)
	}
	return newOp(form.id, tokens, span, args...), nil
}

// parseNullForm parses a form whose first literal token has already been consumed.
func (parser *prattParser[ID]) parseNullForm(form Rule[ID], first Token) (Tree[ID], *ParseError) {
	return parser.parseFormTail(form, []Token{first}, first.Span, nil)
}

// parseLeftForm parses a form whose first hole is the already parsed left expression.
func (parser *prattParser[ID]) parseLeftForm(form Rule[ID], left Tree[ID]) (Tree[ID], *ParseError) {
	return parser.parseFormTail(form, nil, left.Span(), []Tree[ID]{left})
}

// expect consumes a literal part or reports whether the expected token was
// missing at EOF or replaced by another token.
func (parser *prattParser[ID]) expect(part Part) (Token, *ParseError) {
	token := parser.peek()
	if keyForToken(token) == keyForPart(part) {
		return parser.consume(), nil
	}
	if token.isEOF() {
		return Token{}, parser.newParseError(token.Span, &MissingTokenError{Expected: part.text})
	}
	return Token{}, parser.newParseError(token.Span, &ExpectedTokenError{Got: token.Text, Expected: part.text})
}
