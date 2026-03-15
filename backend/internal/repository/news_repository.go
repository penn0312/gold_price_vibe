package repository

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/source"
)

type NewsRepository struct {
	db *gorm.DB
}

func NewNewsRepository(db *gorm.DB) *NewsRepository {
	return &NewsRepository{db: db}
}

func (r *NewsRepository) EnsureSource(meta source.SourceMeta) (model.DataSource, error) {
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

func (r *NewsRepository) CountNews() (int64, error) {
	var count int64
	return count, r.db.Model(&model.NewsArticleRecord{}).Count(&count).Error
}

func (r *NewsRepository) SaveArticle(record model.NewsArticleRecord) (model.NewsArticleRecord, bool, error) {
	var existing model.NewsArticleRecord
	err := r.db.Where("content_hash = ?", record.ContentHash).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, true, r.db.Create(&record).Error
	}
	if err != nil {
		return model.NewsArticleRecord{}, false, err
	}

	return existing, false, nil
}

func (r *NewsRepository) ListNews(query model.NewsQuery) ([]model.NewsArticleRecord, int64, error) {
	var total int64
	var records []model.NewsArticleRecord

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

	tx := r.db.Model(&model.NewsArticleRecord{})
	if query.Category != "" {
		tx = tx.Where("category = ?", query.Category)
	}
	if query.Region != "" {
		tx = tx.Where("region = ?", query.Region)
	}
	if query.Importance > 0 {
		tx = tx.Where("importance = ?", query.Importance)
	}
	if query.FactorCode != "" {
		tx = tx.Where("related_factors_json LIKE ?", "%"+query.FactorCode+"%")
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("published_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].PublishedAt.After(records[j].PublishedAt)
	})
	return records, total, nil
}

func (r *NewsRepository) ListLatest(limit int) ([]model.NewsArticleRecord, error) {
	var records []model.NewsArticleRecord
	if limit <= 0 {
		limit = 4
	}
	err := r.db.Order("published_at desc").Limit(limit).Find(&records).Error
	return records, err
}

func (r *NewsRepository) GetByID(id int64) (model.NewsArticleRecord, error) {
	var record model.NewsArticleRecord
	return record, r.db.First(&record, id).Error
}

func (r *NewsRepository) SaveJobRun(record model.JobRunRecord) error {
	return r.db.Create(&record).Error
}

func BuildNewsRecord(sourceID uint, item source.NewsItem, summary string, contentHash string, category string, region string, sentiment string, importance int, impactScore float64, relatedFactors []string) model.NewsArticleRecord {
	relatedJSON, _ := json.Marshal(relatedFactors)

	return model.NewsArticleRecord{
		SourceID:           sourceID,
		SourceName:         item.SourceName,
		Title:              strings.TrimSpace(item.Title),
		Summary:            strings.TrimSpace(summary),
		Content:            strings.TrimSpace(item.Content),
		ContentHash:        contentHash,
		URL:                strings.TrimSpace(item.URL),
		PublishedAt:        item.PublishedAt,
		CapturedAt:         item.CapturedAt,
		Region:             region,
		Category:           category,
		Sentiment:          sentiment,
		Importance:         importance,
		ImpactScore:        impactScore,
		RelatedFactorsJSON: string(relatedJSON),
		CreatedAt:          time.Now(),
	}
}
