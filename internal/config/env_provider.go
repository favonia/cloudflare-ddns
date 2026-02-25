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

	parts := strings.SplitN(val, ":", 2) // len(parts) >= 1 because val is not empty
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch {
	case len(parts) == 1 && parts[0] == "cloudflare":
		ppfmt.Noticef(
			pp.EmojiUserError,
			`%s=cloudflare is invalid; use %s=cloudflare.trace or %s=cloudflare.doh`,
			key, key, key,
		)
		return false
	case len(parts) == 1 && parts[0] == "cloudflare.trace":
		*field = provider.NewCloudflareTrace()
		return true
	case len(parts) == 2 && parts[0] == "cloudflare.trace":
		ppfmt.InfoOncef(pp.MessageUndocumentedCustomCloudflareTraceProvider, pp.EmojiHint,
			`You are using the undocumented "cloudflare.trace" provider with custom URL`)
		if parts[1] == "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`%s=cloudflare.trace: must be followed by a URL`,
				key,
			)
			return false
		}
		*field = provider.NewCloudflareTraceCustom(parts[1])
		return true
	case len(parts) == 1 && parts[0] == "cloudflare.doh":
		*field = provider.NewCloudflareDOH()
		return true
	case len(parts) == 1 && parts[0] == "ipify":
		ppfmt.Noticef(
			pp.EmojiUserWarning,
			`%s=ipify is deprecated; use %s=cloudflare.trace or %s=cloudflare.doh`,
			key, key, key,
		)
		*field = provider.NewIpify()
		return true
	case len(parts) == 1 && parts[0] == "local":
		*field = provider.NewLocal()
		return true
	case len(parts) == 2 && parts[0] == "local.iface":
		if parts[1] == "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`%s=local.iface: must be followed by a network interface name`,
				key,
			)
			return false
		}
		ppfmt.InfoOncef(pp.MessageExperimentalLocalWithInterface, pp.EmojiHint,
			`You are using the experimental "local.iface" provider added in version 1.15.0`)
		*field = provider.NewLocalWithInterface(parts[1])
		return true
	case len(parts) == 2 && parts[0] == "url":
		p, ok := provider.NewCustomURL(ppfmt, parts[1])
		if ok {
			*field = p
		}
		return ok
	case len(parts) == 2 && parts[0] == "literal":
		if parts[1] == "" {
			ppfmt.Noticef(
				pp.EmojiUserError,
				`%s=literal: must be followed by at least one IP address`,
				key,
			)
			return false
		}
		p, ok := provider.NewLiteral(ppfmt, parts[1])
		if ok {
			*field = p
		}
		return ok
	case len(parts) == 1 && parts[0] == "none":
		*field = nil
		return true
	default:
		ppfmt.Noticef(pp.EmojiUserError, "%s (%q) is not a valid provider", key, val)
		return false
	}
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
