package cron

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDescribeIntuitively(t *testing.T) {
	t.Parallel()

	now := time.Now().In(time.Local)
	nextYear := now.AddDate(1, 0, 0)
	diffDay := now.AddDate(0, 0, 1)
	if diffDay.Year() != now.Year() {
		diffDay = now.AddDate(0, 0, -1)
	}

	for name, tc := range map[string]struct {
		time   time.Time
		output string
	}{
		"now": {
			now,
			now.Format("15:04"),
		},
		"1day": {
			diffDay,
			diffDay.Format("02 Jan 15:04"),
		},
		"1year": {
			nextYear,
			nextYear.Format("02 Jan 15:04 2006"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, describeIntuitively(now, tc.time))
		})
	}
}
