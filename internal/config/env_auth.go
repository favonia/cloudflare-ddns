package config

import (
	"regexp"

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

func ensureTokenNotQuoted(ppfmt pp.PP, token, tokenKey string) bool {
	if !tokenHasMatchingQuotes(token) {
		return true
	}

	ppfmt.Noticef(pp.EmojiUserError,
		"The token provided by %s appears to include surrounding quotation marks; remove the extra quotes", tokenKey)
	return false
}

func readPlainAuthTokens(ppfmt pp.PP) (string, string, bool) {
	token1 := getenv(tokenKey1)
	token2 := getenv(tokenKey2)

	var token, tokenKey string
	switch {
	case token1 == "" && token2 == "":
		return "", "", true
	case token1 != "" && token2 != "" && token1 != token2:
		ppfmt.Noticef(pp.EmojiUserError,
			"The values of %s and %s do not match; they must specify the same token", tokenKey1, tokenKey2)
		return "", "", false
	case token1 != "":
		token, tokenKey = token1, tokenKey1
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, hintAuthTokenNewPrefix)
		token, tokenKey = token2, tokenKey2
	}

	// foolproof check: the sample value in README
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Noticef(pp.EmojiUserError, "You need to provide a real API token as %s", tokenKey)
		return "", "", false
	}

	if !ensureTokenNotQuoted(ppfmt, token, tokenKey) {
		return "", "", false
	}

	return token, tokenKey, true
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

	if !ensureTokenNotQuoted(ppfmt, token, key) {
		return "", false
	}

	return token, true
}

func readAuthTokenFiles(ppfmt pp.PP) (string, string, bool) {
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
			"The files specified by %s and %s have conflicting tokens; their content must match", tokenFileKey1, tokenFileKey2)
		return "", "", false
	case token1 != "":
		return token1, tokenFileKey1, true
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, hintAuthTokenNewPrefix)
		return token2, tokenFileKey2, true
	default:
		return "", "", true
	}
}

func readAuthToken(ppfmt pp.PP) (string, bool) {
	tokenPlain, tokenPlainKey, ok := readPlainAuthTokens(ppfmt)
	if !ok {
		return "", false
	}

	tokenFile, tokenFileKey, ok := readAuthTokenFiles(ppfmt)
	if !ok {
		return "", false
	}

	var token string
	switch {
	case tokenPlain != "" && tokenFile != "" && tokenPlain != tokenFile:
		ppfmt.Noticef(pp.EmojiUserError,
			"The value of %s does not match the token found in the file specified by %s; they must specify the same token",
			tokenPlainKey, tokenFileKey)
		return "", false
	case tokenPlain != "":
		token = tokenPlain
	case tokenFile != "":
		token = tokenFile
	default:
		ppfmt.Noticef(pp.EmojiUserError, "Requires either %s or %s", tokenKey1, tokenFileKey1)
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
