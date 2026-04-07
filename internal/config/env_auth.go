package config

import (
	"regexp"
	"strings"

	"github.com/favonia/cloudflare-ddns/internal/api"
	"github.com/favonia/cloudflare-ddns/internal/file"
	"github.com/favonia/cloudflare-ddns/internal/pp"
)

var oauthBearerRegex = regexp.MustCompile(`^[-a-zA-Z0-9._~+/]+=*$`)

// Keys of environment variables.
const (
	tokenKey1     string = "CLOUDFLARE_API_TOKEN"
	tokenKey2     string = "CF_API_TOKEN"
	tokenFileKey1 string = "CLOUDFLARE_API_TOKEN_FILE"
	tokenFileKey2 string = "CF_API_TOKEN_FILE"
)

// hintAuthTokenNewPrefix contains the hint about the transition from
// CF_* to CLOUDFLARE_*.
const hintAuthTokenNewPrefix string = "Cloudflare is switching to the CLOUDFLARE_* prefix for its tools. Use CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE instead of CF_* (fully supported until 2.0.0 and then minimally supported until 3.0.0)." //nolint:lll

func tokenHasMatchingQuotes(token string) bool {
	if len(token) < 2 {
		return false
	}

	switch {
	case token[0] == '"' && token[len(token)-1] == '"':
		return true
	case token[0] == '\'' && token[len(token)-1] == '\'':
		return true
	default:
		return false
	}
}

func sanityCheckToken(ppfmt pp.PP, tokenKey string, fromFile bool, token string) bool {
	// foolproof check: the sample value in README
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Noticef(pp.EmojiUserError, "You need to provide a real API token as the value of %s", tokenKey)
		return false
	}

	// Some setups, including NixOS modules, pass an environment file as the token file.
	if fromFile {
		switch {
		case strings.HasPrefix(token, "CLOUDFLARE_API_TOKEN="):
			ppfmt.Noticef(pp.EmojiUserError,
				`The token file appears to be an environment file with "CLOUDFLARE_API_TOKEN=..."; `+
					`the file should contain only the token itself`)
			return false
		case strings.HasPrefix(token, "CF_API_TOKEN="):
			ppfmt.Noticef(pp.EmojiUserError,
				`The token file appears to be an environment file with "CF_API_TOKEN=..."; `+
					`the file should contain only the token itself`)
			return false
		}
	}

	if tokenHasMatchingQuotes(token) {
		ppfmt.Noticef(pp.EmojiUserError,
			"The value of %s appears to include surrounding quotation marks; remove the extra quotes", tokenKey)
		return false
	}

	return true
}

func readPlainAuthTokens(ppfmt pp.PP) (tokenKey, token string, ok bool) {
	token1 := getenv(tokenKey1)
	token2 := getenv(tokenKey2)

	switch {
	case token1 == "" && token2 == "":
		return "", "", true
	case token1 != "" && token2 != "" && token1 != token2:
		ppfmt.Noticef(pp.EmojiUserError,
			"The values of %s and %s do not match; they must specify the same token", tokenKey1, tokenKey2)
		return "", "", false
	case token1 != "":
		tokenKey, token = tokenKey1, token1
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, hintAuthTokenNewPrefix)
		tokenKey, token = tokenKey2, token2
	}

	if !sanityCheckToken(ppfmt, tokenKey, false, token) {
		return "", "", false
	}

	return tokenKey, token, true
}

func readAuthTokenFile(ppfmt pp.PP, key string) (string, bool) {
	tokenFile := getenv(key)
	if tokenFile == "" {
		return "", true
	}

	token, ok := file.ReadString(ppfmt, tokenFile)
	if !ok {
		return "", false
	}

	if token == "" {
		ppfmt.Noticef(pp.EmojiUserError, "The file specified by %s does not contain an API token", key)
		return "", false
	}

	if !sanityCheckToken(ppfmt, key, true, token) {
		return "", false
	}

	return token, true
}

func readAuthTokenFiles(ppfmt pp.PP) (tokenKey, token string, ok bool) {
	token1, ok := readAuthTokenFile(ppfmt, tokenFileKey1)
	if !ok {
		return "", "", false
	}

	token2, ok := readAuthTokenFile(ppfmt, tokenFileKey2)
	if !ok {
		return "", "", false
	}

	switch {
	case token1 != "" && token2 != "" && token1 != token2:
		ppfmt.Noticef(pp.EmojiUserError,
			"The files specified by %s and %s have different tokens; their content must match", tokenFileKey1, tokenFileKey2)
		return "", "", false
	case token1 != "":
		return tokenFileKey1, token1, true
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, hintAuthTokenNewPrefix)
		return tokenFileKey2, token2, true
	default:
		return "", "", true
	}
}

func readAuthToken(ppfmt pp.PP) (string, bool) {
	tokenPlainKey, tokenPlain, ok := readPlainAuthTokens(ppfmt)
	if !ok {
		return "", false
	}

	tokenFromFileKey, tokenFromFile, ok := readAuthTokenFiles(ppfmt)
	if !ok {
		return "", false
	}

	var token string
	switch {
	case tokenPlain != "" && tokenFromFile != "" && tokenPlain != tokenFromFile:
		ppfmt.Noticef(pp.EmojiUserError,
			"The value of %s does not match the token found in the file specified by %s; they must specify the same token",
			tokenPlainKey, tokenFromFileKey)
		return "", false
	case tokenPlain != "":
		token = tokenPlain
	case tokenFromFile != "":
		token = tokenFromFile
	default:
		ppfmt.Noticef(pp.EmojiUserError, "Either %s or %s must be set", tokenKey1, tokenFileKey1)
		return "", false
	}

	if !oauthBearerRegex.MatchString(token) {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The API token appears to be invalid; it does not follow the OAuth2 bearer token format")
	}

	return token, true
}

// readAuth reads environment variables CLOUDFLARE_API_TOKEN, CLOUDFLARE_API_TOKEN_FILE,
// CF_API_TOKEN, CF_API_TOKEN_FILE, and CF_ACCOUNT_ID and creates an [api.CloudflareAuth].
func readAuth(ppfmt pp.PP, field *api.Auth) bool {
	token, ok := readAuthToken(ppfmt)
	if !ok {
		return false
	}

	if getenv("CF_ACCOUNT_ID") != "" {
		ppfmt.Noticef(pp.EmojiUserWarning, "CF_ACCOUNT_ID is ignored since 1.14.0")
	}

	*field = &api.CloudflareAuth{Token: token, BaseURL: ""}
	return true
}
