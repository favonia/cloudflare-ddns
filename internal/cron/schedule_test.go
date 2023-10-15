package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/cron"
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
			require.Equal(t, tc, cron.DescribeSchedule(cron.MustNew(tc)))
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
			require.Panics(t, func() { cron.MustNew(tc) })
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()
	const delta = time.Second
	for _, tc := range [...]struct {
		spec     string
		interval time.Duration
	}{
		{"@every 1h1m", time.Hour + time.Minute},
		{"@every 4h", time.Hour * 4},
	} {
		tc := tc // capture range variable
		t.Run(tc.spec, func(t *testing.T) {
			t.Parallel()
			require.WithinDuration(t, time.Now().Add(tc.interval), cron.Next(cron.MustNew(tc.spec)), delta)
		})
	}
}

func TestNextNever(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]string{
		"* * 30 2 *",
	} {
		tc := tc // capture range variable
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			require.True(t, cron.Next(cron.MustNew(tc)).IsZero())
		})
	}
}

func TestNextNil(t *testing.T) {
	t.Parallel()
	require.True(t, cron.Next(nil).IsZero())
}
