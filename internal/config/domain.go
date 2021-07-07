package config

import (
	"strings"

	"golang.org/x/net/idna"
)

func normalizeDomain(domain string) string {
	domain = strings.TrimSuffix(domain, ".")
	if normalized, err := idna.ToUnicode(domain); err == nil {
		return normalized
	}
	return domain
}
