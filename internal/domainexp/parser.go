package domainexp

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func scanDomain(ppfmt pp.PP, input string, tokens []string) (domain.Domain, []string) {
	if len(tokens) == 0 {
		return nil, nil
	}
	switch tokens[0] {
	case "(", "&&", "||", "!", ",":
		ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted a domain; got %q`, input, tokens[0])
		return nil, nil
	case ")":
		return nil, tokens
	default:
		domain, err := domain.New(tokens[0])
		if err != nil {
			ppfmt.Warningf(pp.EmojiUserError,
				"Parsing %q: domain %q was added but it is ill-formed: %v",
				input, domain.Describe(), err)
		}
		return domain, tokens[1:]
	}
}

func scanDomainList(ppfmt pp.PP, input string, tokens []string) ([]domain.Domain, []string) {
	var list []domain.Domain
	for {
		var domain domain.Domain // to avoid := in the next line that would shadow token
		domain, tokens = scanDomain(ppfmt, input, tokens)
		if tokens == nil {
			return nil, nil
		}
		if domain != nil {
			list = append(list, domain)
		}

		if len(tokens) == 0 {
			return list, tokens
		}
		switch tokens[0] {
		case ",":
			tokens = tokens[1:]
			continue
		case ")":
			return list, tokens
		default:
			ppfmt.Errorf(pp.EmojiUserError, `Failed to parse %q: wanted ","; got %q`, input, tokens[0])
			return nil, nil
		}
	}
}

func scanDomainListInASCII(ppfmt pp.PP, input string, tokens []string) ([]string, []string) {
	domains, tokens := scanDomainList(ppfmt, input, tokens)
	if tokens == nil {
		return nil, nil
	}

	ASCIIDomains := make([]string, 0, len(domains))
	for _, domain := range domains {
		ASCIIDomains = append(ASCIIDomains, domain.DNSNameASCII())
	}

	return ASCIIDomains, tokens
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
			ASCIIDomains, newTokens := scanDomainListInASCII(ppfmt, input, newTokens)
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
		ppfmt.Errorf(pp.EmojiUserError, "Parsing %q: unexpected  %q", input, tokens[0])
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
		ppfmt.Errorf(pp.EmojiUserError, "Parsing %q: unexpected  %q", input, tokens[0])
	}

	return pred, true
}
