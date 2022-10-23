package domainexp

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func scanList(ppfmt pp.PP, input string, tokens []string) ([]string, []string) {
	var list []string
	readyForNext := true
	for len(tokens) > 0 {
		switch tokens[0] {
		case ",":
			readyForNext = true
		case ")":
			return list, tokens
		case "(", "&&", "||", "!":
			ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: invalid token %q in a list`, input, tokens[0])
			return nil, nil
		default:
			if !readyForNext {
				ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted ","; got %q`, input, tokens[0])
			}
			list = append(list, tokens[0])
			readyForNext = false
		}

		tokens = tokens[1:]
	}
	return list, tokens
}

func scanASCIIDomainList(ppfmt pp.PP, input string, tokens []string) ([]string, []string) {
	list, tokens := scanList(ppfmt, input, tokens)
	domains := make([]string, 0, len(list))
	for _, raw := range list {
		domains = append(domains, domain.StringToASCII(raw))
	}
	return domains, tokens
}

func scanDomainList(ppfmt pp.PP, input string, tokens []string) ([]domain.Domain, []string) {
	list, tokens := scanList(ppfmt, input, tokens)
	domains := make([]domain.Domain, 0, len(list))
	for _, raw := range list {
		domain, err := domain.New(raw)
		if err != nil {
			ppfmt.Warningf(pp.EmojiUserError,
				"Domain %q was added but it is ill-formed: %v",
				domain.Describe(), err)
		}
		domains = append(domains, domain)
	}
	return domains, tokens
}

//nolint:unparam
func scanConstants(_ppfmt pp.PP, _input string, tokens []string, wanted []string) (string, []string) {
	if len(tokens) == 0 {
		return "", nil
	}
	for _, wanted := range wanted {
		if wanted == tokens[0] {
			return tokens[0], tokens[1:]
		}
	}
	return "", nil
}

func scanMustConstant(ppfmt pp.PP, input string, tokens []string, wanted string) []string {
	if len(tokens) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted ")"; got end-of-string`, input)
		return nil
	}
	if wanted == tokens[0] {
		return tokens[1:]
	}
	ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted ")"; got %q`, input, tokens[0])
	return nil
}

type predicate = func(domain.Domain) bool

func hasSuffix(s, suffix string) bool {
	return len(suffix) == 0 || (strings.HasSuffix(s, suffix) && (len(s) == len(suffix) || s[len(s)-len(suffix)-1] == '.'))
}

// scanAtomic mimics ParseBool, call scanFunction, and then check parenthesized expressions.
//
// <factor> --> true | false | <fun> | ! <factor> | ( <expression> )
//
//nolint:funlen
func scanFactor(ppfmt pp.PP, input string, tokens []string) (predicate, []string) {
	// fmt.Printf("scanFactor(tokens = %#v)\n", tokens)

	if _, newTokens := scanConstants(ppfmt, input, tokens,
		[]string{"1", "t", "T", "TRUE", "true", "True"}); newTokens != nil {
		return func(_ domain.Domain) bool { return true }, newTokens
	}

	if _, newTokens := scanConstants(ppfmt, input, tokens,
		[]string{"0", "f", "F", "FALSE", "false", "False"}); newTokens != nil {
		return func(_ domain.Domain) bool { return false }, newTokens
	}

	{
		//nolint:nestif
		if funName, newTokens := scanConstants(ppfmt, input, tokens, []string{"is", "sub"}); newTokens != nil {
			newTokens = scanMustConstant(ppfmt, input, newTokens, "(")
			if newTokens == nil {
				return nil, nil
			}
			ASCIIDomains, newTokens := scanASCIIDomainList(ppfmt, input, newTokens)
			if newTokens == nil {
				return nil, nil
			}
			newTokens = scanMustConstant(ppfmt, input, newTokens, ")")
			if newTokens == nil {
				return nil, nil
			}

			return map[string]predicate{
				"is": func(d domain.Domain) bool {
					asciiD := d.DNSNameASCII()
					for _, pat := range ASCIIDomains {
						if pat == asciiD {
							return true
						}
					}
					return false
				},
				"sub": func(d domain.Domain) bool {
					asciiD := d.DNSNameASCII()
					for _, pat := range ASCIIDomains {
						if hasSuffix(asciiD, pat) {
							return true
						}
					}
					return false
				},
			}[funName], newTokens
		}
	}

	{
		_, newTokens := scanConstants(ppfmt, input, tokens, []string{"!"})
		if newTokens != nil {
			if pred, newTokens := scanFactor(ppfmt, input, newTokens); newTokens != nil {
				return func(d domain.Domain) bool { return !(pred(d)) }, newTokens
			}
			return nil, nil
		}
	}

	{
		_, newTokens := scanConstants(ppfmt, input, tokens, []string{"("})
		if newTokens != nil {
			pred, newTokens := scanExpression(ppfmt, input, newTokens)
			if newTokens == nil {
				return nil, nil
			}
			newTokens = scanMustConstant(ppfmt, input, newTokens, ")")
			if newTokens == nil {
				ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted ')'`, input)
				return nil, nil
			}
			return pred, newTokens
		}
	}

	if len(tokens) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: wanted boolean expression; got end-of-string", input)
	} else {
		ppfmt.Errorf(pp.EmojiUserError, "Failed to parse %q: wanted boolean expression; got %q", input, tokens[0])
	}
	return nil, nil
}

// scanTerm scans a term with this grammar:
//
//	<term> --> <factor> "&&" <term> | <factor>
func scanTerm(ppfmt pp.PP, input string, tokens []string) (predicate, []string) {
	// fmt.Printf("scanTerm(tokens = %#v)\n", tokens)

	pred1, tokens := scanFactor(ppfmt, input, tokens)
	if tokens == nil {
		return nil, nil
	}

	_, newTokens := scanConstants(ppfmt, input, tokens, []string{"&&"})
	if newTokens == nil {
		return pred1, tokens
	}

	pred2, newTokens := scanTerm(ppfmt, input, newTokens)
	if newTokens != nil {
		return func(d domain.Domain) bool { return pred1(d) && pred2(d) }, newTokens
	}

	return nil, nil
}

// scanExpression scans an expression with this grammar:
//
//	<expression> --> <term> "||" <expression> | <term>
func scanExpression(ppfmt pp.PP, input string, tokens []string) (predicate, []string) {
	pred1, tokens := scanTerm(ppfmt, input, tokens)
	if tokens == nil {
		return nil, nil
	}

	_, newTokens := scanConstants(ppfmt, input, tokens, []string{"||"})
	if newTokens == nil {
		return pred1, tokens
	}

	pred2, newTokens := scanExpression(ppfmt, input, newTokens)
	if newTokens != nil {
		return func(d domain.Domain) bool { return pred1(d) || pred2(d) }, newTokens
	}

	return nil, nil
}

func ParseList(ppfmt pp.PP, input string) ([]domain.Domain, bool) {
	tokens, ok := tokenize(ppfmt, input)
	if !ok {
		return nil, false
	}

	list, tokens := scanDomainList(ppfmt, input, tokens)
	if tokens == nil {
		return nil, false
	} else if len(tokens) > 0 {
		ppfmt.Errorf(pp.EmojiUserError, "Parsing %q: unexpected %q", input, tokens[0])
	}

	return list, true
}

func ParseExpression(ppfmt pp.PP, input string) (predicate, bool) {
	tokens, ok := tokenize(ppfmt, input)
	if !ok {
		return nil, false
	}

	pred, tokens := scanExpression(ppfmt, input, tokens)
	if tokens == nil {
		return nil, false
	} else if len(tokens) > 0 {
		ppfmt.Errorf(pp.EmojiUserError, "Parsing %q: unexpected %q", input, tokens[0])
	}

	return pred, true
}
