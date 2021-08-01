package cron

import (
	"fmt"
	"time"
)

const (
	minutesPerHour   = 60
	secondsPerMinute = 60
)

func describeOffset(offset int) string {
	sign := '+'
	if offset < 0 {
		sign = '−' // ISO 8601 says we should use '−' instead of '-' when possible
		offset = -offset
	}

	hours := offset / secondsPerMinute / minutesPerHour
	minutes := (offset / secondsPerMinute) % minutesPerHour
	seconds := offset % secondsPerMinute

	switch {
	case minutes == 0 && seconds == 0:
		return fmt.Sprintf("UTC%c%02d", sign, hours)
	case seconds == 0:
		return fmt.Sprintf("UTC%c%02d:%02d", sign, hours, minutes)
	default:
		// This should not happen in reality because the UTC offsets of
		// all current timezones on the Earth are multiples of 15 minutes.
		// However, we can still deal with it in a reasonable way.
		return fmt.Sprintf("UTC%c%02d:%02d:%02d", sign, hours, minutes, seconds)
	}
}

// DescribeLocation gives a description of loc that combines loc.String() and time offset.
func DescribeLocation(loc *time.Location) string {
	_, offset := time.Now().In(loc).Zone()

	return fmt.Sprintf("%s (%s now)", loc.String(), describeOffset(offset))
}
