package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/favonia/cloudflare-ddns/internal/cron"
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return loc
}

func TestDescribeLocation(t *testing.T) {
	t.Parallel()
	for _, tc := range [...]struct {
		input  *time.Location
		output string
	}{
		{time.UTC, "UTC (UTC+00 now)"},
		{mustLoadLocation("Asia/Thimphu"), "Asia/Thimphu (UTC+06 now)"},
		{mustLoadLocation("Asia/Seoul"), "Asia/Seoul (UTC+09 now)"},
		{mustLoadLocation("Asia/Shanghai"), "Asia/Shanghai (UTC+08 now)"},
		{mustLoadLocation("Asia/Kolkata"), "Asia/Kolkata (UTC+05:30 now)"},
		{mustLoadLocation("America/Port_of_Spain"), "America/Port_of_Spain (UTC−04 now)"},
		{time.FixedZone("Dublin Mean Time", -1521), "Dublin Mean Time (UTC−00:25:21 now)"},
		{time.FixedZone("Bangkok Mean Time", 24124), "Bangkok Mean Time (UTC+06:42:04 now)"},
	} {
		tc := tc
		t.Run(tc.input.String(), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, cron.DescribeLocation(tc.input))
		})
	}
}
