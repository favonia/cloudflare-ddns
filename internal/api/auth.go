package api

import (
	"context"
	"time"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

type Auth interface {
	New(context.Context, pp.PP, time.Duration) (Handle, bool)
}
