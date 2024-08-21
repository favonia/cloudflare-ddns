package config

import (
	"io"
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
		(*ppfmt).Noticef(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valEmoji, err)
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
		(*ppfmt).Noticef(pp.EmojiUserError, "%s (%q) is not a boolean: %v", key, valQuiet, err)
		return false
	}

	if quiet {
		*ppfmt = (*ppfmt).SetVerbosity(pp.Quiet)
	} else {
		*ppfmt = (*ppfmt).SetVerbosity(pp.Verbose)
	}

	return true
}

// SetupPP sets up a new PP according to the values of EMOJI and QUIET.
func SetupPP(output io.Writer) (pp.PP, bool) {
	ppfmt := pp.New(output)
	if !ReadEmoji("EMOJI", &ppfmt) || !ReadQuiet("QUIET", &ppfmt) {
		return nil, false
	}
	return ppfmt, true
}
