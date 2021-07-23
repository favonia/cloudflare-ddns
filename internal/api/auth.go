package api

import (
	"context"
	"time"
)

type Auth interface {
	New(context.Context, time.Duration) (*Handle, bool)
}

type TokenAuth struct {
	Token     string
	AccountID string
}

func (t *TokenAuth) New(ctx context.Context, cacheExpiration time.Duration) (*Handle, bool) {
	return New(ctx, t.Token, t.AccountID, cacheExpiration)
}
