package cron

import (
	"fmt"
	"time"
)

func absInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func describeOffset(offset int) string {
	sign := '+'
	if offset < 0 {
		sign = '−' // ISO 8601 says we should use '−' instead of '-' when possible
		offset = -offset
	}

	hours := offset / 60 / 60
	minutes := (offset / 60) % 60
	seconds := offset % 60

	switch {
	case minutes == 0 && seconds == 0:
		return fmt.Sprintf("UTC%c%02d", sign, hours)
	case seconds == 0:
		return fmt.Sprintf("UTC%c%02d:%02d", sign, hours, minutes)
	default: // this should not happen, but we can deal with it
		return fmt.Sprintf("UTC%c%02d:%02d:%02d", sign, hours, minutes, seconds)
	}
}

func DescribeTimezone() string {
	zone, offset := time.Now().Zone()

	return fmt.Sprintf("%s (%s)", zone, describeOffset(offset))
}
