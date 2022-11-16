package monitor

import (
	"context"

	"github.com/favonia/cloudflare-ddns/internal/pp"
)

func SuccessAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Success(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

func StartAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Start(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

func FailureAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Failure(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

func LogAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.Log(ctx, ppfmt, message) {
			ok = false
		}
	}
	return ok
}

func ExitStatusAll(ctx context.Context, ppfmt pp.PP, ms []Monitor, code int, message string) bool {
	ok := true
	for _, m := range ms {
		if !m.ExitStatus(ctx, ppfmt, code, message) {
			ok = false
		}
	}
	return ok
}
