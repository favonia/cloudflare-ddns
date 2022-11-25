// Package cron handles anything related to time.
package cron

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// cronSchedule holds a parsed cron expression and its original input.
type cronSchedule struct {
	spec     string
	schedule cron.Schedule
}

// New creates a new Schedule.
func New(spec string) (Schedule, error) {
	sche, err := cron.ParseStandard(spec)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", spec, err)
	}

	return &cronSchedule{
		spec:     spec,
		schedule: sche,
	}, nil
}

// MustNew creates a new Schedule, and panics if it fails to parse the input.
func MustNew(spec string) Schedule {
	cron, err := New(spec)
	if err != nil {
		panic(fmt.Errorf(`schedule.MustNew failed: %w`, err))
	}

	return cron
}

// Next tells the next scheduled time.
func (s *cronSchedule) Next() time.Time {
	return s.schedule.Next(time.Now())
}

// String gives back the original cron string.
func (s *cronSchedule) String() string {
	return s.spec
}
