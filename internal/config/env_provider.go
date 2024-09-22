package config

import (
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/ipnet"
	"github.com/favonia/cloudflare-ddns/internal/pp"
	"github.com/favonia/cloudflare-ddns/internal/provider"
)

// ReadProvider reads an environment variable and parses it as a provider.
//
// policyKey was the name of the deprecated parameters IP4/6_POLICY.
func ReadProvider(ppfmt pp.PP, key, keyDeprecated string, field *provider.Provider) bool {
	val := Getenv(key)

	if val == "" {
		// parsing of the deprecated parameter
		switch valDeprecated := Getenv(keyDeprecated); valDeprecated {
		case "":
			ppfmt.Infof(pp.EmojiBullet, "Use default %s=%s", key, provider.Name(*field))
			return true
		case "cloudflare":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s=cloudflare is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
				keyDeprecated, key, key,
			)
			*field = provider.NewCloudflareTrace()
			return true
		case "cloudflare.trace":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s is deprecated; use %s=%s`,
				keyDeprecated, key, valDeprecated,
			)
			*field = provider.NewCloudflareTrace()
			return true
		case "cloudflare.doh":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s is deprecated; use %s=%s`,
				keyDeprecated, key, valDeprecated,
			)
			*field = provider.NewCloudflareDOH()
			return true
		case "ipify":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
				keyDeprecated, key, key,
			)
			*field = provider.NewIpify()
			return true
		case "local":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s is deprecated; use %s=%s`,
				keyDeprecated, key, valDeprecated,
			)
			*field = provider.NewLocal()
			return true
		case "unmanaged":
			ppfmt.Noticef(
				pp.EmojiUserWarning,
				`%s is deprecated; use %s=none`,
				keyDeprecated, key,
			)
			*field = nil
			return true
		default:
			ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", keyDeprecated, valDeprecated)
			return false
		}
	}

	if Getenv(keyDeprecated) != "" {
		ppfmt.Noticef(
			pp.EmojiUserError,
			`Cannot have both %s and %s set`,
			key, keyDeprecated,
		)
		return false
	}

	switch val {
	case "cloudflare":
		ppfmt.Noticef(
			pp.EmojiUserError,
			`%s=cloudflare is invalid; use %s=cloudflare.trace or %s=cloudflare.doh`,
			key, key, key,
		)
		return false
	case "cloudflare.trace":
		*field = provider.NewCloudflareTrace()
		return true
	case "cloudflare.doh":
		*field = provider.NewCloudflareDOH()
		return true
	case "ipify":
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
			key, key, key,
		)
		*field = provider.NewIpify()
		return true
	case "local":
		*field = provider.NewLocal()
		return true
	case "none":
		*field = nil
		return true
	}

	if strings.HasPrefix(val, "url:") {
		url := strings.TrimSpace(strings.TrimPrefix(val, "url:"))
		p, ok := provider.NewCustomURL(ppfmt, url)
		if ok {
			*field = p
		}
		return ok
	}

	ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, val)
	return false
}

// ReadProviderMap reads the environment variables IP4_PROVIDER and IP6_PROVIDER,
// with support of deprecated environment variables IP4_POLICY and IP6_POLICY.
func ReadProviderMap(ppfmt pp.PP, field *map[ipnet.Type]provider.Provider) bool {
	ip4Provider := (*field)[ipnet.IP4]
	ip6Provider := (*field)[ipnet.IP6]

	if !ReadProvider(ppfmt, "IP4_PROVIDER", "IP4_POLICY", &ip4Provider) ||
		!ReadProvider(ppfmt, "IP6_PROVIDER", "IP6_POLICY", &ip6Provider) {
		return false
	}

	*field = map[ipnet.Type]provider.Provider{
		ipnet.IP4: ip4Provider,
		ipnet.IP6: ip6Provider,
	}
	return true
}
