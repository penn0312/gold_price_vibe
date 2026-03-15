package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/source"
)

type FactorRepository struct {
	db *gorm.DB
}

func NewFactorRepository(db *gorm.DB) *FactorRepository {
	return &FactorRepository{db: db}
}

func (r *FactorRepository) EnsureSource(meta source.SourceMeta) (model.DataSource, error) {
	record := model.DataSource{}
	err := r.db.Where("code = ?", meta.Code).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		priority := meta.Priority
		if priority == 0 {
			priority = 1
		}
		record = model.DataSource{
			Code:      meta.Code,
			Name:      meta.Name,
			Category:  meta.Category,
			BaseURL:   meta.BaseURL,
			IsEnabled: true,
			Priority:  priority,
		}
		return record, r.db.Create(&record).Error
	}
	if err != nil {
		return model.DataSource{}, err
	}

	record.Name = meta.Name
	record.Category = meta.Category
	record.BaseURL = meta.BaseURL
	record.IsEnabled = true
	if meta.Priority > 0 {
		record.Priority = meta.Priority
	}
	return record, r.db.Save(&record).Error
}

func (r *FactorRepository) CountDefinitions() (int64, error) {
	var count int64
	return count, r.db.Model(&model.FactorDefinitionRecord{}).Count(&count).Error
}

func (r *FactorRepository) CountSnapshots() (int64, error) {
	var count int64
	return count, r.db.Model(&model.FactorSnapshotRecord{}).Count(&count).Error
}

func (r *FactorRepository) UpsertDefinitions(records []model.FactorDefinitionRecord) error {
	for _, item := range records {
		var existing model.FactorDefinitionRecord
		err := r.db.Where("code = ?", item.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := r.db.Create(&item).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}

		existing.Name = item.Name
		existing.Category = item.Category
		existing.Description = item.Description
		existing.ValueType = item.ValueType
		existing.Unit = item.Unit
		existing.DefaultWeight = item.DefaultWeight
		existing.ImpactDirectionRule = item.ImpactDirectionRule
		if err := r.db.Save(&existing).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *FactorRepository) ListDefinitions() ([]model.FactorDefinitionRecord, error) {
	var records []model.FactorDefinitionRecord
	err := r.db.Order("id asc").Find(&records).Error
	return records, err
}

func (r *FactorRepository) GetDefinitionByCode(code string) (model.FactorDefinitionRecord, error) {
	var record model.FactorDefinitionRecord
	return record, r.db.Where("code = ?", code).First(&record).Error
}

func (r *FactorRepository) SaveSnapshots(records []model.FactorSnapshotRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range records {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *FactorRepository) GetLatestSnapshotByFactorID(factorID uint) (model.FactorSnapshotRecord, error) {
	var record model.FactorSnapshotRecord
	return record, r.db.
		Where("factor_id = ?", factorID).
		Order("captured_at desc").
		Order("id desc").
		First(&record).Error
}

func (r *FactorRepository) ListSnapshotsByFactorID(factorID uint, start, end time.Time) ([]model.FactorSnapshotRecord, error) {
	var records []model.FactorSnapshotRecord
	err := r.db.
		Where("factor_id = ? AND captured_at >= ? AND captured_at <= ?", factorID, start, end).
		Order("captured_at asc").
		Find(&records).Error
	return records, err
}

func (r *FactorRepository) SaveJobRun(record model.JobRunRecord) error {
	return r.db.Create(&record).Error
}
