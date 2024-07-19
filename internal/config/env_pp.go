package config

import (
	"strconv"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

// ReadEmoji reads an environment variable as emoji/no-emoji.
func ReadEmoji(key string, ppfmt *pp.PP) bool {
	valEmoji := Getenv(key)
	if valEmoji == "" {
		return true
	}

	emoji, err := strconv.ParseBool(valEmoji)
	if err != nil {
		(*ppfmt).Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valEmoji, err)
		return false
	}

	*ppfmt = (*ppfmt).SetEmoji(emoji)

	return true
}

// ReadQuiet reads an environment variable as quiet/verbose.
func ReadQuiet(key string, ppfmt *pp.PP) bool {
	valQuiet := Getenv(key)
	if valQuiet == "" {
		return true
	}

	quiet, err := strconv.ParseBool(valQuiet)
	if err != nil {
		(*ppfmt).Errorf(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valQuiet, err)
		return false
	}

	if quiet {
		*ppfmt = (*ppfmt).SetVerbosity(pp.Quiet)
	} else {
		*ppfmt = (*ppfmt).SetVerbosity(pp.Verbose)
	}

	return true
}
