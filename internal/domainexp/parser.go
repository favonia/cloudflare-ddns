// Package domainexp parses expressions containing domains.
package domainexp

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func scanList(ppfmt pp.PP, key string, input string, tokens []string) ([]string, []string) {
	var list []string
	readyForNext := true
	for len(tokens) > 0 {
		switch tokens[0] {
		case ",":
			readyForNext = true
		case ")":
			return list, tokens
		case "(", "&&", "||", "!":
			ppfmt.Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, tokens[0])
			return nil, nil
		default:
			if !readyForNext {
				ppfmt.Warningf(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, tokens[0])
			}
			list = append(list, tokens[0])
			readyForNext = false
		}

		tokens = tokens[1:]
	}
	return list, tokens
}

func scanASCIIDomainList(ppfmt pp.PP, key string, input string, tokens []string) ([]string, []string) {
	list, tokens := scanList(ppfmt, key, input, tokens)
	domains := make([]string, 0, len(list))
	for _, raw := range list {
		domains = append(domains, domain.StringToASCII(raw))
	}
	return domains, tokens
}

func scanDomainList(ppfmt pp.PP, key string, input string, tokens []string) ([]domain.Domain, []string) {
	list, tokens := scanList(ppfmt, key, input, tokens)
	domains := make([]domain.Domain, 0, len(list))
	for _, raw := range list {
		domain, err := domain.New(raw)
		if err != nil {
			ppfmt.Errorf(pp.EmojiUserError,
				"%s (%q) contains an ill-formed domain %q: %v",
				key, input, domain.Describe(), err)
			return nil, nil
		}
		domains = append(domains, domain)
	}
	return domains, tokens
}

func scanConstants(_ppfmt pp.PP, _key string, _input string, tokens []string, expected []string) (string, []string) {
	if len(tokens) == 0 {
		return "", nil
	}
	for _, expected := range expected {
		if expected == tokens[0] {
			return tokens[0], tokens[1:]
		}
	}
	return "", nil
}

func scanMustConstant(ppfmt pp.PP, key string, input string, tokens []string, expected string) []string {
	if len(tokens) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, expected)
		return nil
	}
	if expected == tokens[0] {
		return tokens[1:]
	}
	ppfmt.Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q when %q is expected`, key, input, tokens[0], expected)
	return nil
}

type predicate = func(domain.Domain) bool

func hasStrictSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix) && (len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.')
}

// scanAtomic mimics ParseBool, call scanFunction, and then check parenthesized expressions.
//
// <factor> --> true | false | <fun> | ! <factor> | ( <expression> )
//
//nolint:funlen
func scanFactor(ppfmt pp.PP, key string, input string, tokens []string) (predicate, []string) {
	// fmt.Printf("scanFactor(tokens = %#v)\n", tokens)

	if _, newTokens := scanConstants(ppfmt, key, input, tokens,
		[]string{"1", "t", "T", "TRUE", "true", "True"}); newTokens != nil {
		return func(_ domain.Domain) bool { return true }, newTokens
	}

	if _, newTokens := scanConstants(ppfmt, key, input, tokens,
		[]string{"0", "f", "F", "FALSE", "false", "False"}); newTokens != nil {
		return func(_ domain.Domain) bool { return false }, newTokens
	}

	{
		//nolint:nestif
		if funName, newTokens := scanConstants(ppfmt, key, input, tokens, []string{"is", "sub"}); newTokens != nil {
			newTokens = scanMustConstant(ppfmt, key, input, newTokens, "(")
			if newTokens == nil {
				return nil, nil
			}
			ASCIIDomains, newTokens := scanASCIIDomainList(ppfmt, key, input, newTokens)
			if newTokens == nil {
				return nil, nil
			}
			newTokens = scanMustConstant(ppfmt, key, input, newTokens, ")")
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
						if hasStrictSuffix(asciiD, pat) {
							return true
						}
					}
					return false
				},
			}[funName], newTokens
		}
	}

	{
		_, newTokens := scanConstants(ppfmt, key, input, tokens, []string{"!"})
		if newTokens != nil {
			if pred, newTokens := scanFactor(ppfmt, key, input, newTokens); newTokens != nil {
				return func(d domain.Domain) bool { return !(pred(d)) }, newTokens
			}
			return nil, nil
		}
	}

	{
		_, newTokens := scanConstants(ppfmt, key, input, tokens, []string{"("})
		if newTokens != nil {
			pred, newTokens := scanExpression(ppfmt, key, input, newTokens)
			if newTokens == nil {
				return nil, nil
			}
			newTokens = scanMustConstant(ppfmt, key, input, newTokens, ")")
			if newTokens == nil {
				return nil, nil
			}
			return pred, newTokens
		}
	}

	if len(tokens) == 0 {
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, input)
	} else {
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) is not a boolean expression: got unexpected token %q", key, input, tokens[0])
	}
	return nil, nil
}

// scanTerm scans a term with this grammar:
//
//	<term> --> <factor> "&&" <term> | <factor>
func scanTerm(ppfmt pp.PP, key string, input string, tokens []string) (predicate, []string) {
	// fmt.Printf("scanTerm(tokens = %#v)\n", tokens)

	pred1, tokens := scanFactor(ppfmt, key, input, tokens)
	if tokens == nil {
		return nil, nil
	}

	_, newTokens := scanConstants(ppfmt, key, input, tokens, []string{"&&"})
	if newTokens == nil {
		return pred1, tokens
	}

	pred2, newTokens := scanTerm(ppfmt, key, input, newTokens)
	if newTokens != nil {
		return func(d domain.Domain) bool { return pred1(d) && pred2(d) }, newTokens
	}

	return nil, nil
}

// scanExpression scans an expression with this grammar:
//
//	<expression> --> <term> "||" <expression> | <term>
func scanExpression(ppfmt pp.PP, key string, input string, tokens []string) (predicate, []string) {
	pred1, tokens := scanTerm(ppfmt, key, input, tokens)
	if tokens == nil {
		return nil, nil
	}

	_, newTokens := scanConstants(ppfmt, key, input, tokens, []string{"||"})
	if newTokens == nil {
		return pred1, tokens
	}

	pred2, newTokens := scanExpression(ppfmt, key, input, newTokens)
	if newTokens != nil {
		return func(d domain.Domain) bool { return pred1(d) || pred2(d) }, newTokens
	}

	return nil, nil
}

// ParseList parses a list of comma-separated domains. Internationalized domain names are fully supported.
func ParseList(ppfmt pp.PP, key string, input string) ([]domain.Domain, bool) {
	tokens, ok := tokenize(ppfmt, key, input)
	if !ok {
		return nil, false
	}

	list, tokens := scanDomainList(ppfmt, key, input, tokens)
	if tokens == nil {
		return nil, false
	} else if len(tokens) > 0 {
		ppfmt.Errorf(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, tokens[0])
		return nil, false
	}

	return list, true
}

// ParseExpression parses a boolean expression containing domains. Internationalized domain names are fully supported.
// A boolean expression must have one of the following forms:
//
//   - A boolean value accepted by [strconv.ParseBool], such as t as true or FALSE as false.
//   - is(example.org), which matches the domain example.org. Note that is(*.example.org)
//     only matches the wildcard domain *.example.org; use sub(example.org) to match
//     all subdomains of example.org (including *.example.org).
//   - sub(example.org), which matches subdomains of example.org, such as www.example.org and *.example.org.
//     It does not match the domain example.org itself.
//   - ! exp, where exp is a boolean expression, representing logical negation of exp.
//   - exp1 || exp2, where exp1 and exp2 are boolean expressions, representing logical disjunction of exp1 and exp2.
//   - exp1 && exp2, where exp1 and exp2 are boolean expressions, representing logical conjunction of exp1 and exp2.
//
// One can use parentheses to group expressions, such as !(is(hello.org) && (is(hello.io) || is(hello.me))).
func ParseExpression(ppfmt pp.PP, key string, input string) (predicate, bool) {
	tokens, ok := tokenize(ppfmt, key, input)
	if !ok {
		return nil, false
	}

	pred, tokens := scanExpression(ppfmt, key, input, tokens)
	if tokens == nil {
		return nil, false
	} else if len(tokens) > 0 {
		ppfmt.Errorf(pp.EmojiUserError, "%s (%q) has unexpected token %q", key, input, tokens[0])
		return nil, false
	}

	return pred, true
}
