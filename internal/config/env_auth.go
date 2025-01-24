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
	TokenKey1     string = "CLOUDFLARE_API_TOKEN"
	TokenKey2     string = "CF_API_TOKEN"
	TokenFileKey1 string = "CLOUDFLARE_API_TOKEN_FILE"
	TokenFileKey2 string = "CF_API_TOKEN_FILE"
)

// HintAuthTokenNewPrefix contains the hint about the transition from
// CF_* to CLOUDFLARE_*.
const HintAuthTokenNewPrefix string = "Cloudflare is switching to the CLOUDFLARE_* prefix for its tools. Use CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_TOKEN_FILE instead of CF_* (fully supported until 2.0.0 and then minimally supported until 3.0.0)." //nolint:lll

func readPlainAuthTokens(ppfmt pp.PP) (string, string, bool) {
	token1 := Getenv(TokenKey1)
	token2 := Getenv(TokenKey2)

	var token, tokenKey string
	switch {
	case token1 == "" && token2 == "":
		return "", "", true
	case token1 != "" && token2 != "" && token1 != token2:
		ppfmt.Noticef(pp.EmojiUserError,
			"The values of %s and %s do not match; they must specify the same token", TokenKey1, TokenKey2)
		return "", "", false
	case token1 != "":
		token, tokenKey = token1, TokenKey1
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, HintAuthTokenNewPrefix)
		token, tokenKey = token2, TokenKey2
	}

	// foolproof check: the sample value in README
	if token == "YOUR-CLOUDFLARE-API-TOKEN" {
		ppfmt.Noticef(pp.EmojiUserError, "You need to provide a real API token as %s", tokenKey)
		return "", "", false
	}

	return token, tokenKey, true
}

func readAuthTokenFile(ppfmt pp.PP, key string) (string, bool) {
	tokenFile := Getenv(key)
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

	return token, true
}

func readAuthTokenFiles(ppfmt pp.PP) (string, string, bool) {
	token1, ok := readAuthTokenFile(ppfmt, TokenFileKey1)
	if !ok {
		return "", "", false
	}

	token2, ok := readAuthTokenFile(ppfmt, TokenFileKey2)
	if !ok {
		return "", "", false
	}

	switch {
	case token1 != "" && token2 != "" && token1 != token2:
		ppfmt.Noticef(pp.EmojiUserError,
			"The files specified by %s and %s have conflicting tokens; their content must match", TokenFileKey1, TokenFileKey2)
		return "", "", false
	case token1 != "":
		return token1, TokenFileKey1, true
	case token2 != "":
		ppfmt.NoticeOncef(pp.MessageAuthTokenNewPrefix, pp.EmojiHint, HintAuthTokenNewPrefix)
		return token2, TokenFileKey2, true
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
		ppfmt.Noticef(pp.EmojiUserError, "Needs either %s or %s", TokenKey1, TokenFileKey1)
		return "", false
	}

	if !oauthBearerRegex.MatchString(token) {
		ppfmt.Noticef(pp.EmojiUserWarning,
			"The API token appears to be invalid; it does not follow the OAuth2 bearer token format")
	}

	return token, true
}

// ReadAuth reads environment variables CLOUDFLARE_API_TOKEN, CLOUDFLARE_API_TOKEN_FILE,
// CF_API_TOKEN, CF_API_TOKEN_FILE, and CF_ACCOUNT_ID and creates an [api.CloudflareAuth].
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
