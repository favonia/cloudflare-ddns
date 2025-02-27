// Package domainexp parses expressions containing domains.
package domainexp

import (
	"errors"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/domain"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func scanList(ppfmt pp.PP, key string, input string, tokens []string) ([]string, []string) {
	var list []string
	expectingElement := true
	for len(tokens) > 0 {
		switch tokens[0] {
		case ",":
			expectingElement = true
			tokens = tokens[1:]
		case ")":
			return list, tokens
		case "[", "]", "(", "&&", "||", "!":
			ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, tokens[0])
			return nil, nil
		default:
			if !expectingElement {
				ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, tokens[0])
			}
			list = append(list, tokens[0])
			expectingElement = false
			tokens = tokens[1:]
		}
	}
	return list, tokens
}

type taggedItem struct {
	Element string
	Tag     string
}

func scanTaggedList(ppfmt pp.PP, key string, input string, tokens []string) ([]taggedItem, []string) {
	var list []taggedItem
	expectingElement := true
	for len(tokens) > 0 {
		switch tokens[0] {
		case ",":
			expectingElement = true
			tokens = tokens[1:]
		case ")":
			return list, tokens
		case "[":
			ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing a domain before the opening bracket %q`, key, input, tokens[0])
			return nil, nil
		case "]", "(", "&&", "||", "!":
			ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, tokens[0])
			return nil, nil
		default:
			if !expectingElement {
				ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing a comma "," before %q`, key, input, tokens[0])
			}
			domain := tokens[0]

			host := ""
			switch {
			case len(tokens) == 1, tokens[1] != "[":
				tokens = tokens[1:]
			case len(tokens) == 2: // 'domain', '['
				ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unclosed "[" at the end`, key, input)
				return nil, nil
			default: // 'domain', '[', ?
				switch tokens[2] {
				case "]", ",", "(", ")", "&&", "||", "!":
					ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q when a host ID is expected`,
						key, input, tokens[0])
					return nil, nil
				default:
					switch {
					case len(tokens) == 3: // 'domain', '[', 'host'
						ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unclosed "[" at the end`, key, input)
						return nil, nil
					case tokens[3] != "]":
						ppfmt.Noticef(pp.EmojiUserError,
							`%s (%q) has unexpected token %q when %q is expected`,
							key, input, tokens[2], "]")
						return nil, nil
					default: // 'domain', '[', 'host', ']'
						host = tokens[2]
						tokens = tokens[4:]
					}
				}
			}
			list = append(list, taggedItem{Element: domain, Tag: host})

			expectingElement = false
		}
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

func parseDomain(ppfmt pp.PP, key string, input string, s string) (domain.Domain, bool) {
	d, err := domain.New(s)
	if err != nil {
		if errors.Is(err, domain.ErrNotFQDN) {
			ppfmt.Noticef(pp.EmojiUserError,
				`%s (%q) contains a domain %q that is probably not fully qualified; a fully qualified domain name (FQDN) would look like "*.example.org" or "sub.example.org"`, //nolint:lll
				key, input, d.Describe())
			return nil, false
		}
		ppfmt.Noticef(pp.EmojiUserError,
			"%s (%q) contains an ill-formed domain %q: %v",
			key, input, d.Describe(), err)
		return nil, false
	}
	return d, true
}

func parseHost(ppfmt pp.PP, key string, input string, s string) (ipnet.HostID, bool) {
	if s == "" {
		return nil, true
	}

	h, err := ipnet.ParseHost(s)
	if err != nil {
		ppfmt.Noticef(pp.EmojiUserError,
			"%s (%q) contains an ill-formed host ID %q: %v",
			key, input, s, err)
		return nil, false
	}

	return h, true
}

func scanDomainList(ppfmt pp.PP, key string, input string, tokens []string) ([]domain.Domain, []string) {
	list, tokens := scanList(ppfmt, key, input, tokens)
	domains := make([]domain.Domain, 0, len(list))
	for _, raw := range list {
		d, ok := parseDomain(ppfmt, key, input, raw)
		if !ok {
			return nil, nil
		}
		domains = append(domains, d)
	}
	return domains, tokens
}

func scanDomainHostIDList(ppfmt pp.PP, key string, input string, tokens []string) (
	[]DomainHostID, []string,
) {
	list, tokens := scanTaggedList(ppfmt, key, input, tokens)
	domains := make([]DomainHostID, 0, len(list))
	for _, raw := range list {
		d, ok := parseDomain(ppfmt, key, input, raw.Element)
		if !ok {
			return nil, nil
		}
		h, ok := parseHost(ppfmt, key, input, raw.Tag)
		if !ok {
			return nil, nil
		}
		domains = append(domains, DomainHostID{Domain: d, HostID: h})
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
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) is missing %q at the end`, key, input, expected)
		return nil
	}
	if expected == tokens[0] {
		return tokens[1:]
	}
	ppfmt.Noticef(pp.EmojiUserError,
		`%s (%q) has unexpected token %q when %q is expected`, key, input, tokens[0], expected)
	return nil
}

type predicate = func(domain.Domain) bool

func hasStrictSuffix(s, suffix string) bool {
	return strings.HasSuffix(s, suffix) && (len(s) > len(suffix) && s[len(s)-len(suffix)-1] == '.')
}

// scanFactor mimics ParseBool, call scanFunction, and then check parenthesized expressions.
//
//	<factor> --> true | false | <fun> | ! <factor> | ( <expression> )
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
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a boolean expression", key, input)
	} else {
		ppfmt.Noticef(pp.EmojiUserError,
			"%s (%q) is not a boolean expression: got unexpected token %q", key, input, tokens[0])
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

// Parse takes a scanner and return the result.
func Parse[T any](ppfmt pp.PP, key string, input string,
	scan func(pp.PP, string, string, []string) (T, []string),
) (T, bool) {
	var zero T

	tokens, ok := tokenize(ppfmt, key, input)
	if !ok {
		return zero, false
	}

	result, tokens := scan(ppfmt, key, input, tokens)
	if tokens == nil {
		return zero, false
	} else if len(tokens) > 0 {
		ppfmt.Noticef(pp.EmojiUserError, `%s (%q) has unexpected token %q`, key, input, tokens[0])
		return zero, false
	}

	return result, true
}

// ParseDomainHostIDList parses a list of comma-separated domains. Internationalized domain names are fully supported.
func ParseDomainHostIDList(ppfmt pp.PP, key string, input string) ([]DomainHostID, bool) {
	return Parse(ppfmt, key, input, scanDomainHostIDList)
}

// ParseDomainList parses a list of comma-separated domains. Internationalized domain names are fully supported.
func ParseDomainList(ppfmt pp.PP, key string, input string) ([]domain.Domain, bool) {
	return Parse(ppfmt, key, input, scanDomainList)
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
	return Parse(ppfmt, key, input, scanExpression)
}
