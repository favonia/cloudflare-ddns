package domain

import (
	"reflect"
	"strings"

	jet "github.com/CloudyKit/jet/v6"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func hasSuffix(s, suffix string) bool {
	return len(suffix) == 0 || (strings.HasSuffix(s, suffix) && (len(s) == len(suffix) || s[len(s)-len(suffix)-1] == '.'))
}

func ParseTemplate(ppfmt pp.PP, tmpl string) (func(target Domain) (string, bool), bool) {
	loader := jet.NewInMemLoader()
	loader.Set("self", tmpl)

	set := jet.NewSet(loader)

	var targetASCII string

	set.AddGlobalFunc("inDomains", func(args jet.Arguments) reflect.Value {
		for i := 0; i < args.NumOfArguments(); i++ {
			rawDomain := args.Get(i)

			if rawDomain.Kind() != reflect.String {
				ppfmt.Errorf(pp.EmojiUserError, "Value %v is not a string", rawDomain)
				args.Panicf("Value %v is not a string", rawDomain)
			}

			if targetASCII == toASCII(rawDomain.String()) {
				return reflect.ValueOf(true)
			}
		}
		return reflect.ValueOf(false)
	})

	set.AddGlobalFunc("hasSuffix", func(args jet.Arguments) reflect.Value {
		for i := 0; i < args.NumOfArguments(); i++ {
			rawSuffix := args.Get(i)

			if rawSuffix.Kind() != reflect.String {
				ppfmt.Errorf(pp.EmojiUserError, "Value %v is not a string", rawSuffix)
				args.Panicf("Value %v is not a string", rawSuffix)
			}

			if hasSuffix(targetASCII, toASCII(rawSuffix.String())) {
				return reflect.ValueOf(true)
			}
		}
		return reflect.ValueOf(false)
	})

	t, err := set.GetTemplate("self")
	if err != nil {
		ppfmt.Errorf(pp.EmojiUserError, "Could not parse the template %q: %v", tmpl, err)
		return nil, false
	}

	exec := func(target Domain) (string, bool) {
		targetASCII = target.DNSNameASCII()

		var output strings.Builder
		if err = t.Execute(&output, jet.VarMap{}, nil); err != nil {
			ppfmt.Errorf(pp.EmojiUserError, "Could not execute the template %q: %v", tmpl, err)
			return "", false
		}
		return output.String(), true
	}

	return exec, true
}
