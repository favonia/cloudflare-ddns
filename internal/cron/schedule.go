package cron

import (
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

type Schedule = interface {
	Next() time.Time
	String() string
}

type Cron struct {
	spec     string
	schedule cron.Schedule
}

func New(spec string) (*Cron, error) {
	sche, err := cron.ParseStandard(spec)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", spec, err)
	}

	return &Cron{
		spec:     spec,
		schedule: sche,
	}, nil
}

func MustNew(spec string) *Cron {
	cron, err := New(spec)
	if err != nil {
		log.Fatalf(`🤯 schedule.MustNew failed: %v`, err)
	}

	return cron
}

func (s *Cron) Next() time.Time {
	return s.schedule.Next(time.Now())
}

func (s *Cron) String() string {
	return s.spec
}
