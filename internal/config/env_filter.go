package config

import (
	"github.com/favonia/cloudflare-ddns/internal/ipfilter"
	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func readDetectionFilter(ppfmt pp.PP, key string, family ipnet.Family, field *ipfilter.Filter) bool {
	val := getenv(key)
	if val == "" {
		if !field.IsDefault() {
			ppfmt.Infof(pp.EmojiBullet, "Using default %s=%s", key, field.String())
		}
		return true
	}
	filter, ok := ipfilter.Parse(ppfmt, key, family, val)
	if !ok {
		return false
	}
	ppfmt.InfoOncef(pp.MessageExperimentalDetectionFilters, pp.EmojiExperimental,
		"You are using experimental detection filters (unreleased)")
	*field = filter
	return true
}
