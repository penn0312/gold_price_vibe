package repository

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/source"
)

type PriceRepository struct {
	db *gorm.DB
}

func NewPriceRepository(db *gorm.DB) *PriceRepository {
	return &PriceRepository{db: db}
}

func (r *PriceRepository) EnsureSource(meta source.SourceMeta) (model.DataSource, error) {
	record := model.DataSource{}
	err := r.db.Where("code = ?", meta.Code).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = model.DataSource{
			Code:      meta.Code,
			Name:      meta.Name,
			Category:  meta.Category,
			BaseURL:   meta.BaseURL,
			IsEnabled: true,
			Priority:  1,
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
	return record, r.db.Save(&record).Error
}

func (r *PriceRepository) SaveTick(sourceID uint, quote source.PriceQuote) (model.GoldPriceTick, error) {
	var latest model.GoldPriceTick
	err := r.db.Where("symbol = ?", quote.Symbol).Order("captured_at desc").First(&latest).Error
	changeAmount := 0.0
	changeRate := 0.0
	if err == nil {
		changeAmount = quote.Price - latest.Price
		if latest.Price != 0 {
			changeRate = changeAmount / latest.Price * 100
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.GoldPriceTick{}, err
	}

	record := model.GoldPriceTick{
		SourceID:     sourceID,
		Symbol:       quote.Symbol,
		Price:        quote.Price,
		ChangeAmount: round(changeAmount),
		ChangeRate:   round(changeRate),
		Currency:     quote.Currency,
		Unit:         quote.Unit,
		CapturedAt:   quote.CapturedAt,
	}

	return record, r.db.Create(&record).Error
}

func (r *PriceRepository) SaveTicks(sourceID uint, quotes []source.PriceQuote) error {
	for _, item := range quotes {
		if _, err := r.SaveTick(sourceID, item); err != nil {
			return err
		}
	}

	return nil
}

func (r *PriceRepository) CountTicks() (int64, error) {
	var count int64
	return count, r.db.Model(&model.GoldPriceTick{}).Count(&count).Error
}

func (r *PriceRepository) GetLatestTick() (model.GoldPriceTick, error) {
	var record model.GoldPriceTick
	return record, r.db.Order("captured_at desc").First(&record).Error
}

func (r *PriceRepository) ListTicks(symbol string, start, end time.Time) ([]model.GoldPriceTick, error) {
	var records []model.GoldPriceTick
	err := r.db.
		Where("symbol = ? AND captured_at >= ? AND captured_at < ?", symbol, start, end).
		Order("captured_at asc").
		Find(&records).Error
	return records, err
}

func (r *PriceRepository) UpsertCandle(candle model.GoldPriceCandle) error {
	var existing model.GoldPriceCandle
	err := r.db.Where(
		"symbol = ? AND interval = ? AND window_start = ?",
		candle.Symbol,
		candle.Interval,
		candle.WindowStart,
	).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.Create(&candle).Error
	}
	if err != nil {
		return err
	}

	existing.OpenPrice = candle.OpenPrice
	existing.HighPrice = candle.HighPrice
	existing.LowPrice = candle.LowPrice
	existing.ClosePrice = candle.ClosePrice
	existing.AvgPrice = candle.AvgPrice
	existing.SampleCount = candle.SampleCount
	existing.WindowEnd = candle.WindowEnd
	return r.db.Save(&existing).Error
}

func (r *PriceRepository) ListCandles(symbol, interval string, start, end time.Time) ([]model.GoldPriceCandle, error) {
	var records []model.GoldPriceCandle
	err := r.db.
		Where("symbol = ? AND interval = ? AND window_start >= ? AND window_start <= ?", symbol, interval, start, end).
		Order("window_start asc").
		Find(&records).Error
	return records, err
}

func (r *PriceRepository) SaveJobRun(record model.JobRunRecord) error {
	return r.db.Create(&record).Error
}

func (r *PriceRepository) ListJobRuns(limit int) ([]model.JobRunRecord, error) {
	var records []model.JobRunRecord
	err := r.db.Order("started_at desc").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})
	return records, nil
}

func round(value float64) float64 {
	return float64(int(value*1000+0.5)) / 1000
}
