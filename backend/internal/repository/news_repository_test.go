package repository

import (
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/model"
)

func TestNewsRepositoryListNewsFiltersByFactorAndCategory(t *testing.T) {
	t.Parallel()

	repo := newTestNewsRepository(t)
	now := time.Now()

	first, _, err := repo.SaveArticle(model.NewsArticleRecord{
		SourceName:         "Test Source",
		Title:              "美元回落",
		Summary:            "summary",
		Content:            "content",
		ContentHash:        "hash-1",
		URL:                "https://example.com/1",
		PublishedAt:        now,
		CapturedAt:         now,
		Category:           "macro",
		Region:             "US",
		Sentiment:          "positive",
		Importance:         4,
		ImpactScore:        80,
		RelatedFactorsJSON: `["usd_index"]`,
	})
	if err != nil {
		t.Fatalf("save first article: %v", err)
	}

	_, _, err = repo.SaveArticle(model.NewsArticleRecord{
		SourceName:         "Test Source",
		Title:              "实物需求回暖",
		Summary:            "summary",
		Content:            "content",
		ContentHash:        "hash-2",
		URL:                "https://example.com/2",
		PublishedAt:        now.Add(-time.Hour),
		CapturedAt:         now.Add(-time.Hour),
		Category:           "industry",
		Region:             "CN",
		Sentiment:          "positive",
		Importance:         3,
		ImpactScore:        60,
		RelatedFactorsJSON: `["physical_demand"]`,
	})
	if err != nil {
		t.Fatalf("save second article: %v", err)
	}

	records, total, err := repo.ListNews(model.NewsQuery{Page: 1, PageSize: 10, Category: "macro", FactorCode: "usd_index"})
	if err != nil {
		t.Fatalf("list news: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one filtered article, total=%d len=%d", total, len(records))
	}
	if records[0].ID != first.ID {
		t.Fatalf("expected filtered article id %d, got %d", first.ID, records[0].ID)
	}
}

func newTestNewsRepository(t *testing.T) *NewsRepository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "news.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	return NewNewsRepository(db)
}
