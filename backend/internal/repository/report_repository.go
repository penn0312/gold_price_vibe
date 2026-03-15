package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
)

type ReportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) CountReports() (int64, error) {
	var count int64
	return count, r.db.Model(&model.AnalysisReportRecord{}).Count(&count).Error
}

func (r *ReportRepository) UpsertReport(record model.AnalysisReportRecord) (model.AnalysisReportRecord, error) {
	var existing model.AnalysisReportRecord
	err := r.db.Where("report_date = ?", record.ReportDate).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, r.db.Create(&record).Error
	}
	if err != nil {
		return model.AnalysisReportRecord{}, err
	}

	existing.Title = record.Title
	existing.Trend = record.Trend
	existing.Confidence = record.Confidence
	existing.Summary = record.Summary
	existing.FullContent = record.FullContent
	existing.KeyDriversJSON = record.KeyDriversJSON
	existing.RiskPointsJSON = record.RiskPointsJSON
	existing.InputSnapshotJSON = record.InputSnapshotJSON
	existing.AIProvider = record.AIProvider
	existing.ModelName = record.ModelName
	existing.PromptVersion = record.PromptVersion
	existing.GeneratedAt = record.GeneratedAt
	return existing, r.db.Save(&existing).Error
}

func (r *ReportRepository) ReplacePredictions(reportID uint, records []model.ReportPredictionRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("report_id = ?", reportID).Delete(&model.ReportPredictionRecord{}).Error; err != nil {
			return err
		}
		for _, item := range records {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ReportRepository) UpsertScore(record model.ReportScoreRecord) (model.ReportScoreRecord, error) {
	var existing model.ReportScoreRecord
	err := r.db.Where("report_id = ?", record.ReportID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, r.db.Create(&record).Error
	}
	if err != nil {
		return model.ReportScoreRecord{}, err
	}

	existing.ScoredDate = record.ScoredDate
	existing.DirectionScore = record.DirectionScore
	existing.RangeScore = record.RangeScore
	existing.FactorHitScore = record.FactorHitScore
	existing.RiskScore = record.RiskScore
	existing.TotalScore = record.TotalScore
	existing.ActualClose = record.ActualClose
	existing.ActualHigh = record.ActualHigh
	existing.ActualLow = record.ActualLow
	existing.ScoreExplanation = record.ScoreExplanation
	return existing, r.db.Save(&existing).Error
}

func (r *ReportRepository) GetLatestReport() (model.AnalysisReportRecord, error) {
	var record model.AnalysisReportRecord
	return record, r.db.Order("report_date desc").First(&record).Error
}

func (r *ReportRepository) GetReportByDate(reportDate string) (model.AnalysisReportRecord, error) {
	var record model.AnalysisReportRecord
	return record, r.db.Where("report_date = ?", reportDate).First(&record).Error
}

func (r *ReportRepository) GetReportByID(id int64) (model.AnalysisReportRecord, error) {
	var record model.AnalysisReportRecord
	return record, r.db.First(&record, id).Error
}

func (r *ReportRepository) ListReports(query model.ReportQuery) ([]model.AnalysisReportRecord, int64, error) {
	var total int64
	var records []model.AnalysisReportRecord

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}

	tx := r.db.Model(&model.AnalysisReportRecord{})
	if query.StartDate != "" {
		tx = tx.Where("report_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		tx = tx.Where("report_date <= ?", query.EndDate)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("report_date desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *ReportRepository) ListPredictions(reportID uint) ([]model.ReportPredictionRecord, error) {
	var records []model.ReportPredictionRecord
	err := r.db.Where("report_id = ?", reportID).Order("target_date asc").Find(&records).Error
	return records, err
}

func (r *ReportRepository) GetScoreByReportID(reportID uint) (model.ReportScoreRecord, error) {
	var record model.ReportScoreRecord
	return record, r.db.Where("report_id = ?", reportID).First(&record).Error
}

func (r *ReportRepository) ListScores(startDate, endDate string) ([]model.ReportScoreRecord, error) {
	var records []model.ReportScoreRecord
	tx := r.db.Model(&model.ReportScoreRecord{})
	if startDate != "" {
		tx = tx.Where("scored_date >= ?", startDate)
	}
	if endDate != "" {
		tx = tx.Where("scored_date <= ?", endDate)
	}
	err := tx.Order("scored_date asc").Find(&records).Error
	return records, err
}

func (r *ReportRepository) SaveJobRun(record model.JobRunRecord) error {
	return r.db.Create(&record).Error
}

func (r *ReportRepository) LatestGeneratedAtBefore(reportDate string) (time.Time, error) {
	record, err := r.GetReportByDate(reportDate)
	if err != nil {
		return time.Time{}, err
	}
	return record.GeneratedAt, nil
}
