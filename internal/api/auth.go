package api

import (
	"fmt"

	"github.com/cloudflare/cloudflare-go"
)

type Auth interface {
	New() (*Handle, error)
}

type TokenAuth struct {
	Token     string
	AccountID string
}

func (t *TokenAuth) New() (*Handle, error) {
	handle, err := cloudflare.NewWithAPIToken(t.Token, cloudflare.UsingAccount(t.AccountID))
	if err != nil {
		return nil, fmt.Errorf("🤔 The token-based CloudFlare authentication failed: %v", err)
	}
	return &Handle{cf: handle}, nil
}
