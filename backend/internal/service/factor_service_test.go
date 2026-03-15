package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
)

func TestFactorServiceBootstrapSeedsDefinitionsAndSnapshots(t *testing.T) {
	t.Parallel()

	factorService, factorRepo, _, _ := newTestFactorService(t)
	if err := factorService.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap factors: %v", err)
	}

	defCount, err := factorRepo.CountDefinitions()
	if err != nil {
		t.Fatalf("count definitions: %v", err)
	}
	if defCount != 10 {
		t.Fatalf("expected 10 factor definitions, got %d", defCount)
	}

	snapshotCount, err := factorRepo.CountSnapshots()
	if err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if snapshotCount < 900 {
		t.Fatalf("expected at least 900 snapshots, got %d", snapshotCount)
	}
}

func TestFactorServiceUpdateNowWritesLatestSnapshots(t *testing.T) {
	t.Parallel()

	factorService, factorRepo, priceRepo, newsRepo := newTestFactorService(t)
	if err := factorService.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap factors: %v", err)
	}

	sourceRecord, err := priceRepo.EnsureSource(source.SourceMeta{
		Code:     "test_gold_feed",
		Name:     "Test Gold Feed",
		Category: "gold",
		BaseURL:  "local://gold",
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("ensure price source: %v", err)
	}

	capturedAt := time.Now().Add(-time.Minute)
	if _, err := priceRepo.SaveTick(sourceRecord.ID, source.PriceQuote{
		Symbol:     source.DefaultSymbol,
		Price:      563.2,
		Currency:   source.DefaultCurrency,
		Unit:       source.DefaultUnit,
		CapturedAt: capturedAt,
	}); err != nil {
		t.Fatalf("save tick: %v", err)
	}

	if _, _, err := newsRepo.SaveArticle(model.NewsArticleRecord{
		SourceName:         "Test News",
		Title:              "中东局势升温，黄金避险需求上升",
		Summary:            "避险情绪升温",
		Content:            "地缘风险上行推动黄金关注度提升。",
		ContentHash:        "factor-test-news",
		URL:                "https://example.com/factor-news",
		PublishedAt:        capturedAt,
		CapturedAt:         capturedAt,
		Category:           "geopolitics",
		Region:             "Global",
		Sentiment:          "positive",
		Importance:         5,
		ImpactScore:        92,
		RelatedFactorsJSON: `["geopolitics","safe_haven_sentiment"]`,
	}); err != nil {
		t.Fatalf("save news article: %v", err)
	}

	beforeCount, err := factorRepo.CountSnapshots()
	if err != nil {
		t.Fatalf("count snapshots before update: %v", err)
	}

	run, err := factorService.UpdateNow(context.Background())
	if err != nil {
		t.Fatalf("update factors: %v", err)
	}
	if run.Status != "success" {
		t.Fatalf("expected success job run, got %s", run.Status)
	}

	afterCount, err := factorRepo.CountSnapshots()
	if err != nil {
		t.Fatalf("count snapshots after update: %v", err)
	}
	if afterCount != beforeCount+10 {
		t.Fatalf("expected 10 new snapshots, got before=%d after=%d", beforeCount, afterCount)
	}

	latest := factorService.GetLatestFactors()
	if len(latest) != 10 {
		t.Fatalf("expected 10 latest factors, got %d", len(latest))
	}

	foundGeopolitics := false
	for _, item := range latest {
		if item.Code == "geopolitics" {
			foundGeopolitics = true
			if item.Score <= 0 {
				t.Fatalf("expected geopolitics score to be positive, got %.2f", item.Score)
			}
		}
	}
	if !foundGeopolitics {
		t.Fatalf("expected geopolitics factor in latest list")
	}
}

func newTestFactorService(t *testing.T) (*FactorService, *repository.FactorRepository, *repository.PriceRepository, *repository.NewsRepository) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "factor-service.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	factorRepo := repository.NewFactorRepository(db)
	priceRepo := repository.NewPriceRepository(db)
	newsRepo := repository.NewNewsRepository(db)
	factorService := NewFactorService(factorRepo, priceRepo, newsRepo)
	return factorService, factorRepo, priceRepo, newsRepo
}
