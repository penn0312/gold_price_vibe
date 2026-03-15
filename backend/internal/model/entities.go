package model

import (
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DataSource struct {
	ID              uint   `gorm:"primaryKey"`
	Code            string `gorm:"size:64;uniqueIndex"`
	Name            string `gorm:"size:128"`
	Category        string `gorm:"size:32"`
	BaseURL         string `gorm:"size:255"`
	IsEnabled       bool
	Priority        int
	RateLimitPerMin int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type GoldPriceTick struct {
	ID           uint   `gorm:"primaryKey"`
	SourceID     uint   `gorm:"index"`
	Symbol       string `gorm:"size:32;index"`
	Price        float64
	ChangeAmount float64
	ChangeRate   float64
	Currency     string    `gorm:"size:8"`
	Unit         string    `gorm:"size:8"`
	CapturedAt   time.Time `gorm:"index"`
	CreatedAt    time.Time
}

type GoldPriceCandle struct {
	ID          uint   `gorm:"primaryKey"`
	Symbol      string `gorm:"size:32;index:idx_gold_candle_symbol_interval_time"`
	Interval    string `gorm:"size:16;index:idx_gold_candle_symbol_interval_time"`
	OpenPrice   float64
	HighPrice   float64
	LowPrice    float64
	ClosePrice  float64
	AvgPrice    float64
	SampleCount int
	WindowStart time.Time `gorm:"index:idx_gold_candle_symbol_interval_time"`
	WindowEnd   time.Time
	CreatedAt   time.Time
}

type FactorDefinitionRecord struct {
	ID                  uint   `gorm:"primaryKey"`
	Code                string `gorm:"size:64;uniqueIndex"`
	Name                string `gorm:"size:64"`
	Category            string `gorm:"size:32"`
	Description         string
	ValueType           string `gorm:"size:32"`
	Unit                string `gorm:"size:32"`
	DefaultWeight       float64
	ImpactDirectionRule string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type NewsArticleRecord struct {
	ID                 uint   `gorm:"primaryKey"`
	SourceID           uint   `gorm:"index"`
	Title              string `gorm:"size:255;index"`
	Summary            string
	ContentHash        string    `gorm:"size:128;uniqueIndex"`
	URL                string    `gorm:"size:255"`
	PublishedAt        time.Time `gorm:"index"`
	CapturedAt         time.Time
	Region             string `gorm:"size:16"`
	Category           string `gorm:"size:32"`
	Sentiment          string `gorm:"size:16"`
	Importance         int
	ImpactScore        float64
	RelatedFactorsJSON string
	CreatedAt          time.Time
}

type AnalysisReportRecord struct {
	ID                uint   `gorm:"primaryKey"`
	ReportDate        string `gorm:"size:16;uniqueIndex"`
	Title             string `gorm:"size:255"`
	Trend             string `gorm:"size:32"`
	Confidence        float64
	Summary           string
	FullContent       string
	KeyDriversJSON    string
	RiskPointsJSON    string
	InputSnapshotJSON string
	AIProvider        string `gorm:"size:64"`
	ModelName         string `gorm:"size:64"`
	PromptVersion     string `gorm:"size:32"`
	GeneratedAt       time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type JobRunRecord struct {
	ID          uint   `gorm:"primaryKey"`
	JobName     string `gorm:"size:64;index"`
	JobType     string `gorm:"size:32"`
	Status      string `gorm:"size:16"`
	StartedAt   time.Time
	FinishedAt  time.Time
	DurationMS  int
	Message     string
	ErrorDetail string
	CreatedAt   time.Time
}

func OpenDatabase(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&DataSource{},
		&GoldPriceTick{},
		&GoldPriceCandle{},
		&FactorDefinitionRecord{},
		&NewsArticleRecord{},
		&AnalysisReportRecord{},
		&JobRunRecord{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
