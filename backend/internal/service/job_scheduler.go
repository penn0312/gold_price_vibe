package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gold_price/backend/internal/config"
	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
)

type JobRunner struct {
	cfg       config.Config
	jobRepo   *repository.JobRepository
	collector *PriceCollector
	news      *NewsIngestionService
	factors   *FactorService
	reports   *ReportService
}

type JobScheduler struct {
	cfg     config.Config
	jobRepo *repository.JobRepository
	runner  *JobRunner
}

func NewJobRunner(cfg config.Config, jobRepo *repository.JobRepository, collector *PriceCollector, news *NewsIngestionService, factors *FactorService, reports *ReportService) *JobRunner {
	return &JobRunner{
		cfg:       cfg,
		jobRepo:   jobRepo,
		collector: collector,
		news:      news,
		factors:   factors,
		reports:   reports,
	}
}

func NewJobScheduler(cfg config.Config, jobRepo *repository.JobRepository, runner *JobRunner) *JobScheduler {
	return &JobScheduler{
		cfg:     cfg,
		jobRepo: jobRepo,
		runner:  runner,
	}
}

func (r *JobRunner) EnsureDefinitions() error {
	return r.jobRepo.UpsertDefinitions([]model.JobDefinitionRecord{
		{
			JobName:         "collect-price",
			JobType:         jobType("collect-price"),
			ScheduleSpec:    fmt.Sprintf("@every:%ds", r.cfg.PriceCollectIntervalSec),
			IsEnabled:       true,
			RetryLimit:      r.cfg.JobRetryLimit,
			RetryBackoffSec: r.cfg.JobRetryBackoffSec,
			TimeoutSec:      r.cfg.JobTimeoutSec,
		},
		{
			JobName:         "fetch-news",
			JobType:         jobType("fetch-news"),
			ScheduleSpec:    fmt.Sprintf("@every:%ds", r.cfg.NewsFetchIntervalSec),
			IsEnabled:       r.cfg.NewsFetchEnabled,
			RetryLimit:      r.cfg.JobRetryLimit,
			RetryBackoffSec: r.cfg.JobRetryBackoffSec,
			TimeoutSec:      r.cfg.JobTimeoutSec,
		},
		{
			JobName:         "update-factors",
			JobType:         jobType("update-factors"),
			ScheduleSpec:    fmt.Sprintf("@every:%ds", r.cfg.FactorUpdateIntervalSec),
			IsEnabled:       r.cfg.FactorUpdateEnabled,
			RetryLimit:      r.cfg.JobRetryLimit,
			RetryBackoffSec: r.cfg.JobRetryBackoffSec,
			TimeoutSec:      r.cfg.JobTimeoutSec,
		},
		{
			JobName:         "generate-report",
			JobType:         jobType("generate-report"),
			ScheduleSpec:    "daily:" + r.cfg.ReportGenerateTime,
			IsEnabled:       r.cfg.ReportGenerateEnabled,
			RetryLimit:      r.cfg.JobRetryLimit,
			RetryBackoffSec: r.cfg.JobRetryBackoffSec,
			TimeoutSec:      r.cfg.JobTimeoutSec,
		},
		{
			JobName:         "score-report",
			JobType:         jobType("score-report"),
			ScheduleSpec:    "daily:" + r.cfg.ReportScoreTime,
			IsEnabled:       r.cfg.ReportScoreEnabled,
			RetryLimit:      r.cfg.JobRetryLimit,
			RetryBackoffSec: r.cfg.JobRetryBackoffSec,
			TimeoutSec:      r.cfg.JobTimeoutSec,
		},
	})
}

func (r *JobRunner) ListJobDefinitions() []model.JobDefinition {
	records, err := r.jobRepo.ListDefinitions()
	if err != nil {
		return nil
	}

	items := make([]model.JobDefinition, 0, len(records))
	for _, item := range records {
		items = append(items, model.JobDefinition{
			JobName:         item.JobName,
			JobType:         item.JobType,
			ScheduleSpec:    item.ScheduleSpec,
			IsEnabled:       item.IsEnabled,
			RetryLimit:      item.RetryLimit,
			RetryBackoffSec: item.RetryBackoffSec,
			TimeoutSec:      item.TimeoutSec,
			LastRunStatus:   item.LastRunStatus,
			LastRunAt:       formatTimePointer(item.LastRunAt),
			LastFinishedAt:  formatTimePointer(item.LastFinishedAt),
			LastDurationMS:  item.LastDurationMS,
			LastMessage:     item.LastMessage,
			LastErrorDetail: item.LastErrorDetail,
		})
	}
	return items
}

func (r *JobRunner) RunManual(ctx context.Context, jobName, reportDate string) model.JobRun {
	run, err := r.runOnce(ctx, jobName, reportDate, manualJobRunOptions())
	if err != nil {
		return run
	}
	return run
}

func (r *JobRunner) ExecuteScheduled(ctx context.Context, definition model.JobDefinitionRecord, scheduledFor time.Time) {
	maxAttempts := definition.RetryLimit + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastRun model.JobRun
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		triggerMode := "scheduled"
		if attempt > 1 {
			triggerMode = "retry"
		}

		options := normalizeJobRunOptions(JobRunOptions{
			TriggerMode:  triggerMode,
			Attempt:      attempt,
			MaxAttempts:  maxAttempts,
			ScheduledFor: &scheduledFor,
		}, triggerMode)

		timeout := time.Duration(definition.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = time.Duration(r.cfg.JobTimeoutSec) * time.Second
		}
		runCtx := ctx
		cancel := func() {}
		if timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		lastRun, lastErr = r.runOnce(runCtx, definition.JobName, "", options)
		cancel()
		if lastErr == nil {
			return
		}

		if attempt == maxAttempts {
			break
		}

		backoff := time.Duration(definition.RetryBackoffSec) * time.Second
		if backoff <= 0 {
			backoff = time.Duration(r.cfg.JobRetryBackoffSec) * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}

	r.sendAlert(definition, lastRun, lastErr)
}

func (r *JobRunner) runOnce(ctx context.Context, jobName, reportDate string, options JobRunOptions) (model.JobRun, error) {
	if err := r.EnsureDefinitions(); err != nil {
		return model.JobRun{
			JobName: jobName,
			JobType: jobType(jobName),
			Status:  "failed",
			Message: err.Error(),
		}, err
	}

	var (
		run model.JobRun
		err error
	)
	switch jobName {
	case "collect-price":
		run, err = r.collector.CollectWithOptions(ctx, options)
	case "fetch-news":
		if r.news == nil {
			err = fmt.Errorf("%s service is not configured", jobName)
			break
		}
		run, err = r.news.FetchWithOptions(ctx, options)
	case "update-factors":
		if r.factors == nil {
			err = fmt.Errorf("%s service is not configured", jobName)
			break
		}
		run, err = r.factors.UpdateWithOptions(ctx, options)
	case "generate-report":
		if r.reports == nil {
			err = fmt.Errorf("%s service is not configured", jobName)
			break
		}
		run, err = r.reports.GenerateWithOptions(ctx, reportDate, options)
	case "score-report":
		if r.reports == nil {
			err = fmt.Errorf("%s service is not configured", jobName)
			break
		}
		run, err = r.reports.ScoreWithOptions(ctx, reportDate, options)
	default:
		err = fmt.Errorf("unknown job: %s", jobName)
	}
	if err != nil && run.JobName == "" {
		run = model.JobRun{
			JobName:      jobName,
			JobType:      jobType(jobName),
			Status:       "failed",
			TriggerMode:  options.TriggerMode,
			Attempt:      options.Attempt,
			MaxAttempts:  options.MaxAttempts,
			ScheduledFor: formatTimePointer(options.ScheduledFor),
			Message:      err.Error(),
			ErrorDetail:  err.Error(),
		}
	}
	if updateErr := r.jobRepo.UpdateDefinitionRun(jobName, run); updateErr != nil {
		log.Printf("job definition update failed for %s: %v", jobName, updateErr)
	}
	return run, err
}

func (s *JobScheduler) Start(ctx context.Context) {
	if err := s.runner.EnsureDefinitions(); err != nil {
		log.Printf("job definition bootstrap failed: %v", err)
		return
	}

	definitions, err := s.jobRepo.ListDefinitions()
	if err != nil {
		log.Printf("job definition list failed: %v", err)
		return
	}

	for _, definition := range definitions {
		if !definition.IsEnabled || definition.JobName == "collect-price" {
			continue
		}
		if strings.HasPrefix(definition.ScheduleSpec, "@every:") {
			go s.runIntervalLoop(ctx, definition)
			continue
		}
		if strings.HasPrefix(definition.ScheduleSpec, "daily:") {
			go s.runDailyLoop(ctx, definition)
		}
	}
}

func (s *JobScheduler) runIntervalLoop(ctx context.Context, definition model.JobDefinitionRecord) {
	interval, err := parseEverySpec(definition.ScheduleSpec)
	if err != nil {
		log.Printf("invalid interval schedule for %s: %v", definition.JobName, err)
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tickAt := <-ticker.C:
			s.runner.ExecuteScheduled(ctx, definition, tickAt)
		}
	}
}

func (s *JobScheduler) runDailyLoop(ctx context.Context, definition model.JobDefinitionRecord) {
	for {
		nextRun, err := nextDailyRun(definition.ScheduleSpec, time.Now())
		if err != nil {
			log.Printf("invalid daily schedule for %s: %v", definition.JobName, err)
			return
		}

		timer := time.NewTimer(time.Until(nextRun))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.runner.ExecuteScheduled(ctx, definition, nextRun)
		}
	}
}

func parseEverySpec(spec string) (time.Duration, error) {
	value := strings.TrimPrefix(spec, "@every:")
	return time.ParseDuration(value)
}

func nextDailyRun(spec string, now time.Time) (time.Time, error) {
	raw := strings.TrimPrefix(spec, "daily:")
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid daily spec: %s", spec)
	}

	hour, err := time.Parse("15:04", raw)
	if err != nil {
		return time.Time{}, err
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), hour.Hour(), hour.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next, nil
}

func formatTimePointer(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func (r *JobRunner) sendAlert(definition model.JobDefinitionRecord, run model.JobRun, err error) {
	if strings.TrimSpace(r.cfg.JobAlertWebhook) == "" || err == nil {
		return
	}

	req, reqErr := http.NewRequest(http.MethodPost, r.cfg.JobAlertWebhook, strings.NewReader(fmt.Sprintf(
		`{"job_name":"%s","status":"%s","attempt":%d,"max_attempts":%d,"message":"%s"}`,
		run.JobName,
		run.Status,
		run.Attempt,
		run.MaxAttempts,
		strings.ReplaceAll(run.Message, `"`, `'`),
	)))
	if reqErr != nil {
		log.Printf("alert request build failed for %s: %v", definition.JobName, reqErr)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		log.Printf("alert request failed for %s: %v", definition.JobName, doErr)
		return
	}
	_ = resp.Body.Close()
}
