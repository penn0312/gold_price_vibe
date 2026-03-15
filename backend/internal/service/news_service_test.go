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

type stubNewsProvider struct {
	meta  source.SourceMeta
	items []source.NewsItem
	err   error
}

func (p stubNewsProvider) Metadata() source.SourceMeta {
	return p.meta
}

func (p stubNewsProvider) Fetch(context.Context) ([]source.NewsItem, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.items, nil
}

func TestNewsIngestionServiceFetchNowDedupes(t *testing.T) {
	t.Parallel()

	repo := newTestNewsServiceRepo(t)
	now := time.Now()
	service := NewNewsIngestionService(repo, stubNewsProvider{
		meta: source.SourceMeta{
			Code:     "test_news_feed",
			Name:     "Test News Feed",
			Category: "news",
			BaseURL:  "https://example.com/feed",
			Priority: 1,
		},
		items: []source.NewsItem{
			{
				Title:       "美元指数回落，黄金短线获得支撑",
				Content:     "美元指数回调压低持有黄金的机会成本。",
				URL:         "https://example.com/news/1",
				SourceName:  "Feed A",
				PublishedAt: now,
				CapturedAt:  now,
			},
			{
				Title:       "美元指数回落，黄金短线获得支撑",
				Content:     "美元指数回调压低持有黄金的机会成本。",
				URL:         "https://example.com/news/1",
				SourceName:  "Feed A",
				PublishedAt: now,
				CapturedAt:  now,
			},
		},
	})

	run, err := service.FetchNow(context.Background())
	if err != nil {
		t.Fatalf("fetch news: %v", err)
	}
	if run.Status != "success" {
		t.Fatalf("expected success, got %s", run.Status)
	}

	count, err := repo.CountNews()
	if err != nil {
		t.Fatalf("count news: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected deduped news count 1, got %d", count)
	}
}

func TestNewsIngestionServiceGetNewsDetailIncludesContent(t *testing.T) {
	t.Parallel()

	repo := newTestNewsServiceRepo(t)
	now := time.Now()
	record, _, err := repo.SaveArticle(model.NewsArticleRecord{
		SourceName:         "Test Source",
		Title:              "中东局势升温",
		Summary:            "summary",
		Content:            "full content",
		ContentHash:        "news-detail-hash",
		URL:                "https://example.com/detail",
		PublishedAt:        now,
		CapturedAt:         now,
		Category:           "geopolitics",
		Region:             "Global",
		Sentiment:          "positive",
		Importance:         5,
		ImpactScore:        90,
		RelatedFactorsJSON: `["geopolitics"]`,
	})
	if err != nil {
		t.Fatalf("save article: %v", err)
	}

	service := NewNewsIngestionService(repo, &source.MockNewsProvider{})
	item, ok := service.GetNewsDetail(int64(record.ID))
	if !ok {
		t.Fatalf("expected article to exist")
	}
	if item.Content != "full content" {
		t.Fatalf("expected full content in detail response, got %q", item.Content)
	}
}

func newTestNewsServiceRepo(t *testing.T) *repository.NewsRepository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "service-news.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	return repository.NewNewsRepository(db)
}
