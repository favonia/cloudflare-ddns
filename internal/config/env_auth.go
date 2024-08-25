package config

import (
	"regexp"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var oauthBearerRegex = regexp.MustCompile(`^[-a-zA-Z0-9._~+/]+=*$`)

func readAuthToken(ppfmt pp.PP) (string, bool) {
	var (
		token     = Getenv("CF_API_TOKEN")
		tokenFile = Getenv("CF_API_TOKEN_FILE")
	)
	var ok bool

	// foolproof checks
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Noticef(pp.EmojiUserError, "You need to provide a real API token as CF_API_TOKEN")
		return "", false
	}

	switch {
	case token != "" && tokenFile != "":
		ppfmt.Noticef(pp.EmojiUserError, "Cannot have both CF_API_TOKEN and CF_API_TOKEN_FILE set")
		return "", false
	case token != "":
	case tokenFile != "":
		token, ok = file.ReadString(ppfmt, tokenFile)
		if !ok {
			return "", false
		}

		if token == "" {
			ppfmt.Noticef(pp.EmojiUserError, "The token in the file specified by CF_API_TOKEN_FILE is empty")
			return "", false
		}
	default:
		ppfmt.Noticef(pp.EmojiUserError, "Needs either CF_API_TOKEN or CF_API_TOKEN_FILE")
		return "", false
	}

	if !oauthBearerRegex.MatchString(token) {
		ppfmt.Noticef(pp.EmojiUserWarning, "The API token does not look like a valid OAuth2 bearer token")
	}

	return token, true
}

// ReadAuth reads environment variables CF_API_TOKEN, CF_API_TOKEN_FILE, and CF_ACCOUNT_ID
// and creates an [api.CloudflareAuth].
func ReadAuth(ppfmt pp.PP, field *api.Auth) bool {
	token, ok := readAuthToken(ppfmt)
	if !ok {
		return false
	}

	if Getenv("CF_ACCOUNT_ID") != "" {
		ppfmt.Noticef(pp.EmojiUserWarning, "CF_ACCOUNT_ID is ignored since 1.14.0")
	}

	*field = &api.CloudflareAuth{Token: token, BaseURL: ""}
	return true
}
