package repository

import (
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/model"
)

func TestReportRepositoryUpsertAndScoreFlow(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "report-repository.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	repo := NewReportRepository(db)
	report, err := repo.UpsertReport(model.AnalysisReportRecord{
		ReportDate:        "2026-03-15",
		Title:             "黄金短线偏强",
		Trend:             "bullish",
		Confidence:        81,
		Summary:           "summary",
		FullContent:       "full content",
		KeyDriversJSON:    `["避险情绪"]`,
		RiskPointsJSON:    `["美元反弹"]`,
		InputSnapshotJSON: `{"price":562.3}`,
		AIProvider:        "rule-engine",
		ModelName:         "local-phase5-v1",
		PromptVersion:     "phase5-rule-v1",
		GeneratedAt:       time.Now(),
	})
	if err != nil {
		t.Fatalf("upsert report: %v", err)
	}

	if err := repo.ReplacePredictions(report.ID, []model.ReportPredictionRecord{
		{
			ReportID:           report.ID,
			TargetDate:         "2026-03-16",
			PredictedDirection: "up",
			PredictedLow:       560.1,
			PredictedHigh:      565.8,
			PredictedClose:     563.4,
			FactorFocusJSON:    `["避险情绪"]`,
		},
	}); err != nil {
		t.Fatalf("replace predictions: %v", err)
	}

	if _, err := repo.UpsertScore(model.ReportScoreRecord{
		ReportID:         report.ID,
		ScoredDate:       "2026-03-16",
		DirectionScore:   35,
		RangeScore:       28,
		FactorHitScore:   16,
		RiskScore:        10,
		TotalScore:       89,
		ActualClose:      563.2,
		ActualHigh:       565.1,
		ActualLow:        560.8,
		ScoreExplanation: "score explanation",
	}); err != nil {
		t.Fatalf("upsert score: %v", err)
	}

	list, total, err := repo.ListReports(model.ReportQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected 1 report, got total=%d len=%d", total, len(list))
	}

	predictions, err := repo.ListPredictions(report.ID)
	if err != nil {
		t.Fatalf("list predictions: %v", err)
	}
	if len(predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(predictions))
	}

	score, err := repo.GetScoreByReportID(report.ID)
	if err != nil {
		t.Fatalf("get score: %v", err)
	}
	if score.TotalScore != 89 {
		t.Fatalf("expected total score 89, got %.2f", score.TotalScore)
	}
}
