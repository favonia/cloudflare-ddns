package domain

import (
	"strings"
	"text/template"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func hasSuffix(s, suffix string) bool {
	return len(suffix) == 0 || (strings.HasSuffix(s, suffix) && (len(s) == len(suffix) || s[len(s)-len(suffix)-1] == '.'))
}

// templateFuncs returns the function maps for running the template.
// "domains" to match a domain in a list, and "suffix" to match a domain with a suffix in a list.
func templateFuncs(target Domain) template.FuncMap {
	targetASCII := target.DNSNameASCII()

	return template.FuncMap{
		"domain": func(rawDomains ...string) (bool, error) {
			for _, rawDomain := range rawDomains {
				if targetASCII == toASCII(rawDomain) {
					return true, nil
				}
			}
			return false, nil
		},
		"suffix": func(rawSuffixes ...string) (bool, error) {
			for _, rawSuffix := range rawSuffixes {
				if hasSuffix(targetASCII, toASCII(rawSuffix)) {
					return true, nil
				}
			}
			return false, nil
		},
	}
}

func ExecTemplate(ppfmt pp.PP, tmpl string, target Domain) (string, bool) {
	t, err := template.New("").Funcs(templateFuncs(target)).Parse(tmpl)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "%q is not a valid template: %v", tmpl, err)
		return "", false
	}

	var output strings.Builder
	if err = t.Execute(&output, nil); err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", tmpl, err)
		return "", false
	}

	return output.String(), true
}
