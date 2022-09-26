package domain

import (
	"strings"
	"text/template"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func hasSuffix(s, suffix string) bool {
	return len(suffix) == 0 || (strings.HasSuffix(s, suffix) && (len(s) == len(suffix) || s[len(s)-len(suffix)-1] == '.'))
}

func ParseTemplate(ppfmt pp.PP, tmpl string) (func(target Domain) (string, bool), bool) {
	var targetASCII string
	funcMap := template.FuncMap{
		"domain": func(rawDomains ...string) bool {
			for _, rawDomain := range rawDomains {
				if targetASCII == toASCII(rawDomain) {
					return true
				}
			}
			return false
		},
		"suffix": func(rawSuffixes ...string) bool {
			for _, rawSuffix := range rawSuffixes {
				if hasSuffix(targetASCII, toASCII(rawSuffix)) {
					return true
				}
			}
			return false
		},
	}

	t, err := template.New("").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "%q is not a valid template: %v", tmpl, err)
		return nil, false
	}

	exec := func(target Domain) (string, bool) {
		targetASCII = target.DNSNameASCII()

		var output strings.Builder
		if err = t.Execute(&output, nil); err != nil {
			ppfmt.Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", tmpl, err)
			return "", false
		}
		return output.String(), true
	}

	return exec, true
}
