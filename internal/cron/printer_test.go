package cron_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/favonia/cloudflare-ddns-go/internal/cron"
)

type descriptionTest struct {
	input  *time.Location
	output string
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return loc
}

func TestDescribeLocation(t *testing.T) {
	t.Parallel()

	for _, p := range [...]descriptionTest{
		{input: time.UTC, output: "UTC (UTC+00 now)"},
		{input: mustLoadLocation("Asia/Thimphu"), output: "Asia/Thimphu (UTC+06 now)"},
		{input: mustLoadLocation("Asia/Seoul"), output: "Asia/Seoul (UTC+09 now)"},
		{input: mustLoadLocation("Asia/Shanghai"), output: "Asia/Shanghai (UTC+08 now)"},
		{input: mustLoadLocation("Asia/Kolkata"), output: "Asia/Kolkata (UTC+05:30 now)"},
		{input: mustLoadLocation("America/Port_of_Spain"), output: "America/Port_of_Spain (UTC−04 now)"},
		{input: time.FixedZone("Dublin Mean Time", -1521), output: "Dublin Mean Time (UTC−00:25:21 now)"},
		{input: time.FixedZone("Bangkok Mean Time", 24124), output: "Bangkok Mean Time (UTC+06:42:04 now)"},
	} {
		assert.Equalf(t, p.output, cron.DescribeLocation(p.input), "Timezone descriptions should match.")
	}
}
