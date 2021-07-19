package api

import (
	"log"

	"github.com/cloudflare/cloudflare-go"
)

type Auth interface {
	New() (*Handle, bool)
}

type TokenAuth struct {
	Token     string
	AccountID string
}

func (t *TokenAuth) New() (*Handle, bool) {
	handle, err := cloudflare.NewWithAPIToken(t.Token, cloudflare.UsingAccount(t.AccountID))
	if err != nil {
		log.Printf("🤔 The token-based CloudFlare authentication failed: %v", err)
		return nil, false //nolint:nlreturn
	}

	return &Handle{cf: handle}, true
}
