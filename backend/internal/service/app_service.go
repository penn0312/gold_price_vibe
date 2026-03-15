package service

import (
	"context"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
)

type AppService struct {
	priceRepo *repository.PriceRepository
	newsRepo  *repository.NewsRepository
	collector *PriceCollector
	news      *NewsIngestionService
	factors   *FactorService
	reports   *ReportService
	mock      *MockMarketService
}

func NewAppService(priceRepo *repository.PriceRepository, newsRepo *repository.NewsRepository, collector *PriceCollector, news *NewsIngestionService, factors *FactorService, reports *ReportService) *AppService {
	return &AppService{
		priceRepo: priceRepo,
		newsRepo:  newsRepo,
		collector: collector,
		news:      news,
		factors:   factors,
		reports:   reports,
		mock:      NewMockMarketService(),
	}
}

func (s *AppService) GetDashboardOverview() model.DashboardOverview {
	news := s.GetNewsList()
	if len(news) > 4 {
		news = news[:4]
	}

	factors := s.GetLatestFactors()
	if len(factors) > 6 {
		factors = factors[:6]
	}

	return model.DashboardOverview{
		RealtimePrice: s.GetRealtimePrice(),
		LatestReport:  s.GetLatestReport(),
		Factors:       factors,
		Headlines:     news,
	}
}

func (s *AppService) GetRealtimePrice() model.RealtimePrice {
	record, err := s.priceRepo.GetLatestTick()
	if err != nil {
		return s.mock.GetRealtimePrice()
	}

	return model.RealtimePrice{
		Symbol:       record.Symbol,
		Price:        record.Price,
		ChangeAmount: record.ChangeAmount,
		ChangeRate:   record.ChangeRate,
		Currency:     record.Currency,
		Unit:         record.Unit,
		CapturedAt:   record.CapturedAt.Format(time.RFC3339),
	}
}

func (s *AppService) GetPriceHistory(rangeValue, interval string) model.PriceHistory {
	if interval == "" {
		interval = defaultInterval(rangeValue)
	}

	start, end := timeWindow(rangeValue)
	candles, err := s.priceRepo.ListCandles("AU_CNY_G", interval, start, end)
	if err != nil || len(candles) == 0 {
		return s.mock.GetPriceHistory(rangeValue, interval)
	}

	items := make([]model.Candle, 0, len(candles))
	for _, item := range candles {
		items = append(items, model.Candle{
			Time:  item.WindowStart.Format(time.RFC3339),
			Open:  item.OpenPrice,
			High:  item.HighPrice,
			Low:   item.LowPrice,
			Close: item.ClosePrice,
		})
	}

	return model.PriceHistory{
		Symbol:   "AU_CNY_G",
		Interval: interval,
		Items:    items,
	}
}

func (s *AppService) GetNewsList() []model.NewsArticle {
	if s.news == nil {
		return s.mock.GetNewsList()
	}
	return s.news.LatestNews(20)
}

func (s *AppService) ListNews(query model.NewsQuery) model.NewsList {
	if s.news == nil {
		return s.mock.ListNews(query)
	}
	return s.news.ListNews(query)
}

func (s *AppService) GetNewsDetail(id int64) (model.NewsArticle, bool) {
	if s.news == nil {
		return s.mock.GetNewsDetail(id)
	}
	return s.news.GetNewsDetail(id)
}

func (s *AppService) GetLatestFactors() []model.FactorLatest {
	if s.factors != nil {
		return s.factors.GetLatestFactors()
	}
	return s.mock.GetLatestFactors()
}

func (s *AppService) GetFactorDefinitions() []model.FactorDefinition {
	if s.factors != nil {
		return s.factors.GetFactorDefinitions()
	}
	return s.mock.GetFactorDefinitions()
}

func (s *AppService) GetFactorHistory(code, rangeValue string) model.FactorHistory {
	if s.factors != nil {
		return s.factors.GetFactorHistory(code, rangeValue)
	}
	return s.mock.GetFactorHistory(code, rangeValue)
}

func (s *AppService) GetLatestReport() model.ReportSummary {
	if s.reports != nil {
		return s.reports.GetLatestReport()
	}
	return s.mock.GetLatestReport()
}

func (s *AppService) ListReports(query model.ReportQuery) model.ReportList {
	if s.reports != nil {
		return s.reports.ListReports(query)
	}
	return s.mock.ListReports(query)
}

func (s *AppService) GetReportDetail(id int64) (model.ReportDetail, bool) {
	if s.reports != nil {
		return s.reports.GetReportDetail(id)
	}
	return s.mock.GetReportDetail(id)
}

func (s *AppService) GetAccuracyCurve(rangeValue string) model.AccuracyCurve {
	if s.reports != nil {
		return s.reports.GetAccuracyCurve(rangeValue)
	}
	return s.mock.GetAccuracyCurve(rangeValue)
}

func (s *AppService) TriggerJob(jobName string) model.JobRun {
	if jobName == "collect-price" {
		if run, err := s.collector.CollectNow(context.Background()); err == nil {
			return run
		}
	}
	if jobName == "fetch-news" && s.news != nil {
		if run, err := s.news.FetchNow(context.Background()); err == nil {
			return run
		}
	}
	if jobName == "update-factors" && s.factors != nil {
		if run, err := s.factors.UpdateNow(context.Background()); err == nil {
			return run
		}
	}

	run := s.mock.TriggerJob(jobName)
	_ = s.priceRepo.SaveJobRun(model.JobRunRecord{
		JobName:    run.JobName,
		JobType:    run.JobType,
		Status:     run.Status,
		StartedAt:  parseRFC3339(run.StartedAt),
		FinishedAt: parseRFC3339(run.FinishedAt),
		Message:    run.Message,
	})
	return run
}

func (s *AppService) GenerateReport(reportDate string) model.JobRun {
	if s.reports != nil {
		if run, err := s.reports.GenerateNow(context.Background(), reportDate); err == nil {
			return run
		}
	}
	return s.mock.GenerateReport(reportDate)
}

func (s *AppService) ScoreReport(reportDate string) model.JobRun {
	if s.reports != nil {
		if run, err := s.reports.ScoreNow(context.Background(), reportDate); err == nil {
			return run
		}
	}
	return s.mock.ScoreReport(reportDate)
}

func (s *AppService) GetJobRuns() []model.JobRun {
	records, err := s.priceRepo.ListJobRuns(20)
	if err != nil || len(records) == 0 {
		return s.mock.GetJobRuns()
	}

	items := make([]model.JobRun, 0, len(records))
	for _, item := range records {
		items = append(items, model.JobRun{
			ID:         int64(item.ID),
			JobName:    item.JobName,
			JobType:    item.JobType,
			Status:     item.Status,
			StartedAt:  item.StartedAt.Format(time.RFC3339),
			FinishedAt: item.FinishedAt.Format(time.RFC3339),
			Message:    item.Message,
		})
	}
	return items
}

func timeWindow(rangeValue string) (time.Time, time.Time) {
	now := time.Now()
	switch rangeValue {
	case "1d":
		return now.Add(-24 * time.Hour), now
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now
	case "90d":
		return now.Add(-90 * 24 * time.Hour), now
	default:
		return now.Add(-365 * 24 * time.Hour), now
	}
}

func parseRFC3339(value string) time.Time {
	if value == "" {
		return time.Now()
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now()
	}
	return parsed
}
