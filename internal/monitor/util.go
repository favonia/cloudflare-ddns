package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func SuccessAll(ctx context.Context, ppfmt pp.PP, ms []Monitor) bool {
	ok := true
	for _, m := range ms {
		if !m.Success(ctx, ppfmt) {
			ok = false
		}
	}
	return ok
}

func StartAll(ctx context.Context, ppfmt pp.PP, ms []Monitor) bool {
	ok := true
	for _, m := range ms {
		if !m.Start(ctx, ppfmt) {
			ok = false
		}
	}
	return ok
}

func FailureAll(ctx context.Context, ppfmt pp.PP, ms []Monitor) bool {
	ok := true
	for _, m := range ms {
		if !m.Failure(ctx, ppfmt) {
			ok = false
		}
	}
	return ok
}

func ExitStatusAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, code int) bool {
	ok := true
	for _, m := range ms {
		if !m.ExitStatus(ctx, ppfmt, code) {
			ok = false
		}
	}
	return ok
}
