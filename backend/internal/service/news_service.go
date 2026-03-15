package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
)

type NewsIngestionService struct {
	repo     *repository.NewsRepository
	provider source.NewsProvider
}

func NewNewsIngestionService(repo *repository.NewsRepository, provider source.NewsProvider) *NewsIngestionService {
	return &NewsIngestionService{repo: repo, provider: provider}
}

func (s *NewsIngestionService) Bootstrap(ctx context.Context) error {
	count, err := s.repo.CountNews()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = s.FetchNow(ctx)
	return err
}

func (s *NewsIngestionService) FetchNow(ctx context.Context) (model.JobRun, error) {
	startedAt := time.Now()
	items, err := s.provider.Fetch(ctx)
	if err != nil {
		return s.failJob(startedAt, err)
	}

	sourceRecord, err := s.repo.EnsureSource(s.provider.Metadata())
	if err != nil {
		return s.failJob(startedAt, err)
	}

	savedCount := 0
	for _, item := range items {
		record := buildNewsRecord(sourceRecord.ID, item)
		_, created, saveErr := s.repo.SaveArticle(record)
		if saveErr != nil {
			return s.failJob(startedAt, saveErr)
		}
		if created {
			savedCount++
		}
	}

	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "fetch-news",
		JobType:    "collector",
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    fmt.Sprintf("news fetched: %d new article(s)", savedCount),
	}

	err = s.repo.SaveJobRun(model.JobRunRecord{
		JobName:    run.JobName,
		JobType:    run.JobType,
		Status:     run.Status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMS: int(finishedAt.Sub(startedAt).Milliseconds()),
		Message:    run.Message,
	})
	return run, err
}

func (s *NewsIngestionService) ListNews(query model.NewsQuery) model.NewsList {
	records, total, err := s.repo.ListNews(query)
	if err != nil || len(records) == 0 {
		mockItems := s.mockItems()
		return model.NewsList{
			Items:    toNewsArticles(mockNewsRecordsFrom(mockItems, false), false),
			Page:     normalizePage(query.Page),
			PageSize: normalizePageSize(query.PageSize),
			Total:    int64(len(mockItems)),
		}
	}

	return model.NewsList{
		Items:    toNewsArticles(records, false),
		Page:     normalizePage(query.Page),
		PageSize: normalizePageSize(query.PageSize),
		Total:    total,
	}
}

func (s *NewsIngestionService) LatestNews(limit int) []model.NewsArticle {
	records, err := s.repo.ListLatest(limit)
	if err != nil || len(records) == 0 {
		return toNewsArticles(mockNewsRecordsFrom(s.mockItems(), false), false)
	}

	return toNewsArticles(records, false)
}

func (s *NewsIngestionService) GetNewsDetail(id int64) (model.NewsArticle, bool) {
	record, err := s.repo.GetByID(id)
	if err == nil {
		return toNewsArticle(record, true), true
	}

	for _, item := range toNewsArticles(mockNewsRecordsFrom(s.mockItems(), true), true) {
		if item.ID == id {
			return item, true
		}
	}
	return model.NewsArticle{}, false
}

func (s *NewsIngestionService) failJob(startedAt time.Time, err error) (model.JobRun, error) {
	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "fetch-news",
		JobType:    "collector",
		Status:     "failed",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    err.Error(),
	}

	saveErr := s.repo.SaveJobRun(model.JobRunRecord{
		JobName:     run.JobName,
		JobType:     run.JobType,
		Status:      run.Status,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		DurationMS:  int(finishedAt.Sub(startedAt).Milliseconds()),
		Message:     "news ingestion failed",
		ErrorDetail: err.Error(),
	})
	if saveErr != nil {
		return run, errors.Join(err, saveErr)
	}

	return run, err
}

func buildNewsRecord(sourceID uint, item source.NewsItem) model.NewsArticleRecord {
	category := classifyNewsCategory(item.Title, item.Content)
	region := classifyNewsRegion(item.Title, item.Content)
	sentiment := classifyNewsSentiment(item.Title, item.Content)
	importance := classifyNewsImportance(item.Title, item.Content)
	relatedFactors := classifyRelatedFactors(item.Title, item.Content)
	impactScore := calculateImpactScore(importance, sentiment, relatedFactors)
	summary := summarizeNews(item.Title, item.Content)
	contentHash := buildContentHash(item)

	return repository.BuildNewsRecord(
		sourceID,
		item,
		summary,
		contentHash,
		category,
		region,
		sentiment,
		importance,
		impactScore,
		relatedFactors,
	)
}

func classifyNewsCategory(title, content string) string {
	text := strings.ToLower(title + " " + content)
	switch {
	case containsAny(text, "中东", "冲突", "制裁", "geopolit", "war", "risk"):
		return "geopolitics"
	case containsAny(text, "央行", "美联储", "利率", "cpi", "inflation", "政策"):
		return "policy"
	case containsAny(text, "美元", "汇率", "油价", "股市", "指数", "market"):
		return "market"
	case containsAny(text, "实物", "珠宝", "需求", "gold buying", "import"):
		return "industry"
	default:
		return "macro"
	}
}

func classifyNewsRegion(title, content string) string {
	text := strings.ToLower(title + " " + content)
	switch {
	case containsAny(text, "中国", "人民币", "亚洲", "金店", "cn"):
		return "CN"
	case containsAny(text, "美元", "美联储", "us", "fed", "washington"):
		return "US"
	case containsAny(text, "欧元", "欧洲", "ecb", "eu"):
		return "EU"
	default:
		return "Global"
	}
}

func classifyNewsSentiment(title, content string) string {
	text := strings.ToLower(title + " " + content)
	switch {
	case containsAny(text, "支撑", "升温", "避险", "回暖", "增持", "上涨", "bullish"):
		return "positive"
	case containsAny(text, "压制", "回落", "承压", "鹰派", "下跌", "bearish"):
		return "negative"
	default:
		return "neutral"
	}
}

func classifyNewsImportance(title, content string) int {
	text := strings.ToLower(title + " " + content)
	switch {
	case containsAny(text, "美联储", "央行", "地缘", "中东", "战争", "利率"):
		return 5
	case containsAny(text, "美元", "通胀", "油价", "汇率"):
		return 4
	case containsAny(text, "需求", "珠宝", "股市"):
		return 3
	default:
		return 2
	}
}

func classifyRelatedFactors(title, content string) []string {
	text := strings.ToLower(title + " " + content)
	factors := make([]string, 0, 4)
	appendIf := func(code string, keywords ...string) {
		if containsAny(text, keywords...) {
			factors = append(factors, code)
		}
	}

	appendIf("usd_index", "美元", "usd", "dollar")
	appendIf("fed_rate", "美联储", "利率", "fed", "rate")
	appendIf("inflation", "通胀", "cpi", "inflation")
	appendIf("cny_fx", "人民币", "汇率", "fx")
	appendIf("safe_haven_sentiment", "避险", "risk-off", "避险情绪")
	appendIf("central_bank_gold_buying", "央行购金", "央行", "gold buying")
	appendIf("oil_price", "油价", "原油", "oil")
	appendIf("equity_market", "股市", "指数", "equity")
	appendIf("geopolitics", "地缘", "战争", "冲突", "中东")
	appendIf("physical_demand", "实物", "珠宝", "需求", "金店")

	if len(factors) == 0 {
		return []string{"safe_haven_sentiment"}
	}
	return uniqueStrings(factors)
}

func calculateImpactScore(importance int, sentiment string, factors []string) float64 {
	score := float64(importance * 12)
	switch sentiment {
	case "positive":
		score += 18
	case "negative":
		score += 12
	default:
		score += 8
	}
	score += float64(len(factors)) * 4
	if score > 100 {
		return 100
	}
	return score
}

func summarizeNews(title, content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return strings.TrimSpace(title)
	}
	runes := []rune(text)
	if len(runes) > 56 {
		return string(runes[:56]) + "..."
	}
	return text
}

func buildContentHash(item source.NewsItem) string {
	input := strings.Join([]string{
		strings.TrimSpace(strings.ToLower(item.Title)),
		strings.TrimSpace(strings.ToLower(item.URL)),
		strings.TrimSpace(strings.ToLower(item.Content)),
		item.PublishedAt.UTC().Format(time.RFC3339),
	}, "|")
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func toNewsArticles(records []model.NewsArticleRecord, withContent bool) []model.NewsArticle {
	items := make([]model.NewsArticle, 0, len(records))
	for _, record := range records {
		items = append(items, toNewsArticle(record, withContent))
	}
	return items
}

func toNewsArticle(record model.NewsArticleRecord, withContent bool) model.NewsArticle {
	relatedFactors := make([]string, 0)
	_ = json.Unmarshal([]byte(record.RelatedFactorsJSON), &relatedFactors)

	content := ""
	if withContent {
		content = record.Content
	}

	return model.NewsArticle{
		ID:             int64(record.ID),
		SourceName:     record.SourceName,
		Title:          record.Title,
		Summary:        record.Summary,
		Content:        content,
		URL:            record.URL,
		Region:         record.Region,
		Category:       record.Category,
		Sentiment:      record.Sentiment,
		Importance:     record.Importance,
		ImpactScore:    record.ImpactScore,
		RelatedFactors: relatedFactors,
		PublishedAt:    record.PublishedAt.Format(time.RFC3339),
		CapturedAt:     record.CapturedAt.Format(time.RFC3339),
	}
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	switch {
	case pageSize <= 0:
		return 10
	case pageSize > 50:
		return 50
	default:
		return pageSize
	}
}

func (s *NewsIngestionService) mockItems() []source.NewsItem {
	items, _ := (&source.MockNewsProvider{}).Fetch(context.Background())
	return items
}

func mockNewsRecordsFrom(items []source.NewsItem, withContent bool) []model.NewsArticleRecord {
	records := make([]model.NewsArticleRecord, 0, len(items))
	for index, item := range items {
		record := buildNewsRecord(0, item)
		record.ID = uint(index + 1)
		if !withContent {
			record.Content = ""
		}
		records = append(records, record)
	}
	return records
}
