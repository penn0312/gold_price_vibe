package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/config"
	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
)

func TestJobRunnerEnsureDefinitions(t *testing.T) {
	t.Parallel()

	_, _, _, _, jobRepo := newJobSchedulerTestRepos(t)
	runner := NewJobRunner(config.Config{
		PriceCollectIntervalSec: 30,
		NewsFetchEnabled:        true,
		NewsFetchIntervalSec:    600,
		FactorUpdateEnabled:     true,
		FactorUpdateIntervalSec: 900,
		ReportGenerateEnabled:   true,
		ReportGenerateTime:      "09:00",
		ReportScoreEnabled:      true,
		ReportScoreTime:         "09:10",
		JobRetryLimit:           2,
		JobRetryBackoffSec:      1,
		JobTimeoutSec:           5,
	}, jobRepo, nil, nil, nil, nil)

	if err := runner.EnsureDefinitions(); err != nil {
		t.Fatalf("ensure definitions: %v", err)
	}

	definitions, err := jobRepo.ListDefinitions()
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) != 5 {
		t.Fatalf("expected 5 job definitions, got %d", len(definitions))
	}
}

func TestJobRunnerExecuteScheduledRetriesFailures(t *testing.T) {
	t.Parallel()

	priceRepo, _, _, _, jobRepo := newJobSchedulerTestRepos(t)
	collector := NewPriceCollector(priceRepo, collectorStubProvider{
		meta: source.SourceMeta{
			Code:     "failing_gold_feed",
			Name:     "Failing Gold Feed",
			Category: "gold",
			BaseURL:  "local://failing",
		},
		err: errors.New("upstream unavailable"),
	})

	runner := NewJobRunner(config.Config{
		JobRetryBackoffSec: 0,
		JobTimeoutSec:      5,
	}, jobRepo, collector, nil, nil, nil)

	if err := runner.EnsureDefinitions(); err != nil {
		t.Fatalf("ensure definitions: %v", err)
	}

	scheduledFor := time.Now().Add(time.Second)
	runner.ExecuteScheduled(context.Background(), model.JobDefinitionRecord{
		JobName:         "collect-price",
		JobType:         "collector",
		ScheduleSpec:    "@every:30s",
		IsEnabled:       true,
		RetryLimit:      1,
		RetryBackoffSec: 0,
		TimeoutSec:      5,
	}, scheduledFor)

	runs, err := priceRepo.ListJobRuns(10)
	if err != nil {
		t.Fatalf("list job runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 job runs after retry, got %d", len(runs))
	}
	if runs[0].Attempt != 2 {
		t.Fatalf("expected latest run attempt to be 2, got %d", runs[0].Attempt)
	}
	if runs[0].TriggerMode != "retry" {
		t.Fatalf("expected retry trigger mode, got %s", runs[0].TriggerMode)
	}
	if runs[1].TriggerMode != "scheduled" {
		t.Fatalf("expected first trigger mode to be scheduled, got %s", runs[1].TriggerMode)
	}

	definitions, err := jobRepo.ListDefinitions()
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	var collectDefinition model.JobDefinitionRecord
	for _, item := range definitions {
		if item.JobName == "collect-price" {
			collectDefinition = item
			break
		}
	}
	if collectDefinition.LastRunStatus != "failed" {
		t.Fatalf("expected collect-price last status failed, got %s", collectDefinition.LastRunStatus)
	}
}

func newJobSchedulerTestRepos(t *testing.T) (*repository.PriceRepository, *repository.NewsRepository, *repository.FactorRepository, *repository.ReportRepository, *repository.JobRepository) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "scheduler.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	return repository.NewPriceRepository(db),
		repository.NewNewsRepository(db),
		repository.NewFactorRepository(db),
		repository.NewReportRepository(db),
		repository.NewJobRepository(db)
}
