package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
)

type JobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) UpsertDefinitions(records []model.JobDefinitionRecord) error {
	for _, item := range records {
		var existing model.JobDefinitionRecord
		err := r.db.Where("job_name = ?", item.JobName).First(&existing).Error
		if err == nil {
			existing.JobType = item.JobType
			existing.ScheduleSpec = item.ScheduleSpec
			existing.IsEnabled = item.IsEnabled
			existing.RetryLimit = item.RetryLimit
			existing.RetryBackoffSec = item.RetryBackoffSec
			existing.TimeoutSec = item.TimeoutSec
			if err := r.db.Save(&existing).Error; err != nil {
				return err
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := r.db.Create(&item).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *JobRepository) ListDefinitions() ([]model.JobDefinitionRecord, error) {
	var records []model.JobDefinitionRecord
	err := r.db.Order("job_name asc").Find(&records).Error
	return records, err
}

func (r *JobRepository) UpdateDefinitionRun(jobName string, run model.JobRun) error {
	updates := map[string]interface{}{
		"last_run_status":   run.Status,
		"last_duration_ms":  run.DurationMS,
		"last_message":      run.Message,
		"last_error_detail": run.ErrorDetail,
	}

	if startedAt, err := time.Parse(time.RFC3339, run.StartedAt); err == nil {
		updates["last_run_at"] = &startedAt
	}
	if finishedAt, err := time.Parse(time.RFC3339, run.FinishedAt); err == nil {
		updates["last_finished_at"] = &finishedAt
	}

	return r.db.Model(&model.JobDefinitionRecord{}).Where("job_name = ?", jobName).Updates(updates).Error
}
