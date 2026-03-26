package types

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronManager interface {
	Add(jobName, spec string, job func()) error
}

type JobEntry struct {
	ID            cron.EntryID
	Name          string
	Spec          string
	Job           func()
	Timeout       time.Duration
	AddedAt       time.Time
	LastRun       time.Time
	NextRun       time.Time
	LastDuration  time.Duration
	TotalDuration time.Duration
	AvgDuration   time.Duration
	RunCount      int64
	Error         error
}
