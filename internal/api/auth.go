package api

import (
	"context"
	"time"

	"github.com/favonia/cloudflare-ddns-go/internal/pp"
)

type Auth interface {
	New(context.Context, pp.Indent, time.Duration) (*Handle, bool)
}

type TokenAuth struct {
	Token     string
	AccountID string
}

func (t *TokenAuth) New(ctx context.Context, indent pp.Indent, cacheExpiration time.Duration) (*Handle, bool) {
	return New(ctx, indent, t.Token, t.AccountID, cacheExpiration)
}
