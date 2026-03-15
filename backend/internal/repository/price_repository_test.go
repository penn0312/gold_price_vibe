package repository

import (
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/source"
)

func TestPriceRepositorySaveTickComputesDelta(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	repo := NewPriceRepository(db)
	src, err := repo.EnsureSource(source.SourceMeta{
		Code:     "test_source",
		Name:     "Test Source",
		BaseURL:  "local://test",
		Category: "gold",
	})
	if err != nil {
		t.Fatalf("ensure source: %v", err)
	}

	now := time.Now().Add(-time.Minute)
	if _, err := repo.SaveTick(src.ID, source.PriceQuote{
		Symbol: "AU_CNY_G", Price: 560.1, Currency: "CNY", Unit: "g", CapturedAt: now,
	}); err != nil {
		t.Fatalf("save first tick: %v", err)
	}

	latest, err := repo.SaveTick(src.ID, source.PriceQuote{
		Symbol: "AU_CNY_G", Price: 560.8, Currency: "CNY", Unit: "g", CapturedAt: now.Add(30 * time.Second),
	})
	if err != nil {
		t.Fatalf("save second tick: %v", err)
	}

	if latest.ChangeAmount == 0 {
		t.Fatalf("expected change amount to be computed")
	}
	if latest.ChangeRate == 0 {
		t.Fatalf("expected change rate to be computed")
	}
}
