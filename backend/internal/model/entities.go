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

func (FactorDefinitionRecord) TableName() string {
	return "factor_definitions"
}

type FactorSnapshotRecord struct {
	ID              uint `gorm:"primaryKey"`
	FactorID        uint `gorm:"index:idx_factor_snapshot_factor_time"`
	SourceID        uint `gorm:"index"`
	ValueNum        float64
	ValueText       string
	Score           float64
	ImpactDirection string `gorm:"size:16"`
	ImpactStrength  float64
	Confidence      float64
	Summary         string
	CapturedAt      time.Time `gorm:"index:idx_factor_snapshot_factor_time"`
	CreatedAt       time.Time
}

func (FactorSnapshotRecord) TableName() string {
	return "factor_snapshots"
}

type NewsArticleRecord struct {
	ID                 uint   `gorm:"primaryKey"`
	SourceID           uint   `gorm:"index"`
	SourceName         string `gorm:"size:128"`
	Title              string `gorm:"size:255;index"`
	Summary            string
	Content            string
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

func (NewsArticleRecord) TableName() string {
	return "news_articles"
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

func (AnalysisReportRecord) TableName() string {
	return "analysis_reports"
}

type ReportPredictionRecord struct {
	ID                 uint   `gorm:"primaryKey"`
	ReportID           uint   `gorm:"index"`
	TargetDate         string `gorm:"size:16;index"`
	PredictedDirection string `gorm:"size:16"`
	PredictedLow       float64
	PredictedHigh      float64
	PredictedClose     float64
	FactorFocusJSON    string
	CreatedAt          time.Time
}

func (ReportPredictionRecord) TableName() string {
	return "report_predictions"
}

type ReportScoreRecord struct {
	ID               uint   `gorm:"primaryKey"`
	ReportID         uint   `gorm:"uniqueIndex"`
	ScoredDate       string `gorm:"size:16;index"`
	DirectionScore   float64
	RangeScore       float64
	FactorHitScore   float64
	RiskScore        float64
	TotalScore       float64
	ActualClose      float64
	ActualHigh       float64
	ActualLow        float64
	ScoreExplanation string
	CreatedAt        time.Time
}

func (ReportScoreRecord) TableName() string {
	return "report_scores"
}

type JobDefinitionRecord struct {
	ID              uint   `gorm:"primaryKey"`
	JobName         string `gorm:"size:64;uniqueIndex"`
	JobType         string `gorm:"size:32"`
	ScheduleSpec    string `gorm:"size:64"`
	IsEnabled       bool
	RetryLimit      int
	RetryBackoffSec int
	TimeoutSec      int
	LastRunStatus   string `gorm:"size:16"`
	LastRunAt       *time.Time
	LastFinishedAt  *time.Time
	LastDurationMS  int
	LastMessage     string
	LastErrorDetail string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (JobDefinitionRecord) TableName() string {
	return "job_definitions"
}

type JobRunRecord struct {
	ID           uint   `gorm:"primaryKey"`
	JobName      string `gorm:"size:64;index"`
	JobType      string `gorm:"size:32"`
	Status       string `gorm:"size:16"`
	TriggerMode  string `gorm:"size:16"`
	Attempt      int
	MaxAttempts  int
	ScheduledFor *time.Time
	StartedAt    time.Time
	FinishedAt   time.Time
	DurationMS   int
	Message      string
	ErrorDetail  string
	CreatedAt    time.Time
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
		&FactorSnapshotRecord{},
		&NewsArticleRecord{},
		&AnalysisReportRecord{},
		&ReportPredictionRecord{},
		&ReportScoreRecord{},
		&JobDefinitionRecord{},
		&JobRunRecord{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
