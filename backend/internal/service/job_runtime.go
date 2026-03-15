package service

import (
	"time"

	"gold_price/backend/internal/model"
)

type JobRunOptions struct {
	TriggerMode  string
	Attempt      int
	MaxAttempts  int
	ScheduledFor *time.Time
}

func manualJobRunOptions() JobRunOptions {
	return JobRunOptions{
		TriggerMode: "manual",
		Attempt:     1,
		MaxAttempts: 1,
	}
}

func bootstrapJobRunOptions() JobRunOptions {
	return JobRunOptions{
		TriggerMode: "bootstrap",
		Attempt:     1,
		MaxAttempts: 1,
	}
}

func normalizeJobRunOptions(options JobRunOptions, fallbackTrigger string) JobRunOptions {
	if options.TriggerMode == "" {
		options.TriggerMode = fallbackTrigger
	}
	if options.Attempt <= 0 {
		options.Attempt = 1
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = options.Attempt
	}
	if options.MaxAttempts < options.Attempt {
		options.MaxAttempts = options.Attempt
	}
	return options
}

func fillJobRunMeta(run *model.JobRun, options JobRunOptions, durationMS int, err error) {
	run.TriggerMode = options.TriggerMode
	run.Attempt = options.Attempt
	run.MaxAttempts = options.MaxAttempts
	run.DurationMS = durationMS
	if options.ScheduledFor != nil && !options.ScheduledFor.IsZero() {
		run.ScheduledFor = options.ScheduledFor.Format(time.RFC3339)
	}
	if err != nil {
		run.ErrorDetail = err.Error()
	}
}
