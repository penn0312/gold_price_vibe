package service

import (
	"context"
	"path/filepath"
	"testing"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
)

func TestReportServiceGenerateScoreAndDetail(t *testing.T) {
	t.Parallel()

	reportService := newTestReportService(t)
	reportDate := "2026-03-10"

	run, err := reportService.GenerateNow(context.Background(), reportDate)
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}
	if run.Status != "success" {
		t.Fatalf("expected generate success, got %s", run.Status)
	}

	scoreRun, err := reportService.ScoreNow(context.Background(), reportDate)
	if err != nil {
		t.Fatalf("score report: %v", err)
	}
	if scoreRun.Status != "success" {
		t.Fatalf("expected score success, got %s", scoreRun.Status)
	}

	list := reportService.ListReports(model.ReportQuery{Page: 1, PageSize: 10})
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("expected 1 report in list, got total=%d len=%d", list.Total, len(list.Items))
	}
	if list.Items[0].AccuracyScore <= 0 {
		t.Fatalf("expected accuracy score to be populated, got %.2f", list.Items[0].AccuracyScore)
	}

	detail, ok := reportService.GetReportDetail(list.Items[0].ID)
	if !ok {
		t.Fatalf("expected report detail to exist")
	}
	if len(detail.Predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(detail.Predictions))
	}
	if detail.Score == nil || detail.Score.TotalScore <= 0 {
		t.Fatalf("expected detail score to be populated")
	}

	curve := reportService.GetAccuracyCurve("30d")
	if len(curve.Items) != 1 {
		t.Fatalf("expected one accuracy item, got %d", len(curve.Items))
	}
}

func TestReportServiceBootstrapSeedsHistoricalReports(t *testing.T) {
	t.Parallel()

	reportService := newTestReportService(t)
	if err := reportService.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap reports: %v", err)
	}

	list := reportService.ListReports(model.ReportQuery{Page: 1, PageSize: 50})
	if list.Total < 30 {
		t.Fatalf("expected at least 30 reports after bootstrap, got %d", list.Total)
	}

	curve := reportService.GetAccuracyCurve("30d")
	if len(curve.Items) < 20 {
		t.Fatalf("expected historical accuracy items after bootstrap, got %d", len(curve.Items))
	}
}

func newTestReportService(t *testing.T) *ReportService {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "report-service.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	priceRepo := repository.NewPriceRepository(db)
	factorRepo := repository.NewFactorRepository(db)
	newsRepo := repository.NewNewsRepository(db)
	reportRepo := repository.NewReportRepository(db)
	return NewReportService(reportRepo, priceRepo, factorRepo, newsRepo)
}
