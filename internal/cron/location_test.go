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
		{time.UTC, "UTC (currently UTC+00)"},
		{mustLoadLocation("Asia/Thimphu"), "Asia/Thimphu (currently UTC+06)"},
		{mustLoadLocation("Asia/Seoul"), "Asia/Seoul (currently UTC+09)"},
		{mustLoadLocation("Asia/Shanghai"), "Asia/Shanghai (currently UTC+08)"},
		{mustLoadLocation("Asia/Kolkata"), "Asia/Kolkata (currently UTC+05:30)"},
		{mustLoadLocation("America/Port_of_Spain"), "America/Port_of_Spain (currently UTC−04)"},
		{time.FixedZone("Dublin Mean Time", -1521), "Dublin Mean Time (currently UTC−00:25:21)"},
		{time.FixedZone("Bangkok Mean Time", 24124), "Bangkok Mean Time (currently UTC+06:42:04)"},
	} {
		t.Run(tc.input.String(), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.output, cron.DescribeLocation(tc.input))
		})
	}
}
