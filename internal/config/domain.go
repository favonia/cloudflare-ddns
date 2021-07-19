package config

import (
	"strings"

	"golang.org/x/net/idna"
)

func normalizeDomain(domain string) string {
	domain = strings.TrimSuffix(domain, ".")

	normalized, err := idna.ToUnicode(domain)
	if err != nil {
		return domain
	}

	return normalized
}
