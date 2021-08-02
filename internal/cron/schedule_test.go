package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/cron"
)

func TestMustNewSuccessful(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]string{
		"*/4 * * * *",
		"@every 5h0s",
		"@yearly",
	} {
		tc := tc // capture range variable
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc, cron.MustNew(tc).String())
		})
	}
}

func TestMustNewPanicking(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]string{
		"*/4 * * * * *",
		"@every 5ss",
		"@cool",
	} {
		tc := tc // capture range variable
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() { cron.MustNew(tc) })
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()
	const delta = time.Second * 5
	for _, tc := range [...]struct {
		spec     string
		interval time.Duration
	}{
		{"@every 1h", time.Hour},
		{"@every 4h", time.Hour * 4},
	} {
		tc := tc // capture range variable
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			assert.WithinDuration(t, time.Now().Add(tc.interval), cron.MustNew(tc.spec).Next(), delta)
		})
	}
}
