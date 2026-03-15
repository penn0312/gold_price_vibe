package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
)

type collectorStubProvider struct {
	meta       source.SourceMeta
	current    source.PriceQuote
	historical []source.PriceQuote
	err        error
}

func (p collectorStubProvider) Metadata() source.SourceMeta {
	return p.meta
}

func (p collectorStubProvider) CurrentPrice(context.Context) (source.PriceQuote, error) {
	if p.err != nil {
		return source.PriceQuote{}, p.err
	}

	return p.current, nil
}

func (p collectorStubProvider) HistoricalTicks(context.Context, int, time.Duration) ([]source.PriceQuote, error) {
	if p.err != nil {
		return nil, p.err
	}

	return p.historical, nil
}

func TestPriceCollectorRejectsAbnormalJump(t *testing.T) {
	t.Parallel()

	repo := newTestPriceRepository(t)
	src, err := repo.EnsureSource(source.SourceMeta{
		Code:     "existing_source",
		Name:     "Existing Source",
		Category: "gold",
		BaseURL:  "local://existing",
	})
	if err != nil {
		t.Fatalf("ensure source: %v", err)
	}

	capturedAt := time.Now().Add(-time.Minute)
	if _, err := repo.SaveTick(src.ID, source.PriceQuote{
		Symbol:     source.DefaultSymbol,
		Price:      560,
		Currency:   source.DefaultCurrency,
		Unit:       source.DefaultUnit,
		CapturedAt: capturedAt,
	}); err != nil {
		t.Fatalf("save baseline tick: %v", err)
	}

	collector := NewPriceCollector(repo, collectorStubProvider{
		meta: source.SourceMeta{
			Code:     "remote_gold_feed",
			Name:     "Remote Gold Feed",
			Category: "gold",
			BaseURL:  "https://example.com",
			Priority: 1,
		},
		current: source.PriceQuote{
			Symbol:     source.DefaultSymbol,
			Price:      700,
			Currency:   source.DefaultCurrency,
			Unit:       source.DefaultUnit,
			CapturedAt: capturedAt.Add(30 * time.Second),
		},
	})

	_, err = collector.CollectNow(context.Background())
	if err == nil {
		t.Fatalf("expected abnormal jump to be rejected")
	}
	if !strings.Contains(err.Error(), "abnormal jump") {
		t.Fatalf("expected abnormal jump error, got %v", err)
	}

	count, err := repo.CountTicks()
	if err != nil {
		t.Fatalf("count ticks: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected rejected tick not to be saved, got %d ticks", count)
	}
}

func TestBootstrapHistoryFiltersOutliers(t *testing.T) {
	t.Parallel()

	repo := newTestPriceRepository(t)
	start := time.Now().Add(-3 * time.Minute)
	collector := NewPriceCollector(repo, collectorStubProvider{
		meta: source.SourceMeta{
			Code:     "remote_gold_feed",
			Name:     "Remote Gold Feed",
			Category: "gold",
			BaseURL:  "https://example.com",
			Priority: 1,
		},
		historical: []source.PriceQuote{
			{Symbol: source.DefaultSymbol, Price: 560, Currency: source.DefaultCurrency, Unit: source.DefaultUnit, CapturedAt: start},
			{Symbol: source.DefaultSymbol, Price: 980, Currency: source.DefaultCurrency, Unit: source.DefaultUnit, CapturedAt: start.Add(time.Minute)},
			{Symbol: source.DefaultSymbol, Price: 561, Currency: source.DefaultCurrency, Unit: source.DefaultUnit, CapturedAt: start.Add(2 * time.Minute)},
		},
	})

	if err := collector.BootstrapHistory(context.Background()); err != nil {
		t.Fatalf("bootstrap history: %v", err)
	}

	count, err := repo.CountTicks()
	if err != nil {
		t.Fatalf("count ticks: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected bootstrap to keep 2 valid ticks, got %d", count)
	}
}

func newTestPriceRepository(t *testing.T) *repository.PriceRepository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	return repository.NewPriceRepository(db)
}
