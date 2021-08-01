package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/cron"
)

func TestMustNewSuccessful(t *testing.T) {
	t.Parallel()

	for _, s := range [...]string{
		"*/4 * * * *",
		"@every 5h0s",
		"@yearly",
	} {
		assert.Equal(t, s, cron.MustNew(s).String(), "Cron.String() should return the original spec.")
	}
}

func TestMustNewPanicking(t *testing.T) {
	t.Parallel()

	for _, s := range [...]string{
		"*/4 * * * * *",
		"@every 5ss",
		"@cool",
	} {
		s := s // redefine s to avoid capturing of the same variable
		assert.Panics(t, func() { cron.MustNew(s) })
	}
}

func TestNext(t *testing.T) {
	t.Parallel()

	const delta = time.Second * 5
	for _, c := range [...]struct {
		spec     string
		interval time.Duration
	}{
		{"@every 1h", time.Hour},
		{"@every 4h", time.Hour * 4},
	} {
		assert.WithinDuration(t, time.Now().Add(c.interval), cron.MustNew(c.spec).Next(), delta)
	}
}
