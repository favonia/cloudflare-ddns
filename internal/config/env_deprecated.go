package config

import "github.com/favonia/cloudflare-ddns/internal/pp"

// CheckDeprecatedLinuxID reads an environment variable as a user or group ID.
func CheckDeprecatedLinuxID(ppfmt pp.PP, key string, class string) {
	if val := Getenv(key); val != "" {
		ppfmt.Warningf(pp.EmojiUserError, "%s=%s is deprecated; use Docker's built-in mechanism to set %s ID",
			key, val, class)
		ppfmt.Warningf(pp.EmojiHint,
			"See https://github.com/favonia/cloudflare-ddns for the new recommended template")
	}
}
