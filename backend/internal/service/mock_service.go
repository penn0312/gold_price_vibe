package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gold_price/backend/internal/model"
)

type MarketService interface {
	GetDashboardOverview() model.DashboardOverview
	GetRealtimePrice() model.RealtimePrice
	GetPriceHistory(rangeValue, interval string) model.PriceHistory
	GetNewsList() []model.NewsArticle
	GetNewsDetail(id int64) (model.NewsArticle, bool)
	GetLatestFactors() []model.FactorLatest
	GetFactorDefinitions() []model.FactorDefinition
	GetFactorHistory(code, rangeValue string) model.FactorHistory
	GetLatestReport() model.ReportSummary
	GetReports() []model.ReportSummary
	GetReportDetail(id int64) (model.ReportDetail, bool)
	GetAccuracyCurve(rangeValue string) model.AccuracyCurve
	TriggerJob(jobName string) model.JobRun
	GetJobRuns() []model.JobRun
}

type MockMarketService struct {
	jobCounter atomic.Int64
}

func NewMockMarketService() *MockMarketService {
	return &MockMarketService{}
}

func (s *MockMarketService) GetDashboardOverview() model.DashboardOverview {
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

func (s *MockMarketService) GetRealtimePrice() model.RealtimePrice {
	now := time.Now()
	seconds := float64(now.Unix() % 3600)
	price := 562.2 + math.Sin(seconds/240.0)*3.8 + math.Cos(seconds/90.0)*0.6
	changeAmount := math.Sin(seconds/120.0) * 1.18
	changeRate := changeAmount / price * 100

	return model.RealtimePrice{
		Symbol:       "AU_CNY_G",
		Price:        round(price),
		ChangeAmount: round(changeAmount),
		ChangeRate:   round(changeRate),
		Currency:     "CNY",
		Unit:         "g",
		CapturedAt:   now.Format(time.RFC3339),
	}
}

func (s *MockMarketService) GetPriceHistory(rangeValue, interval string) model.PriceHistory {
	if interval == "" {
		interval = defaultInterval(rangeValue)
	}

	count, step := historyConfig(rangeValue, interval)
	now := time.Now()
	items := make([]model.Candle, 0, count)
	base := 558.0

	for i := count - 1; i >= 0; i-- {
		pointTime := now.Add(-time.Duration(i) * step)
		angle := float64(i) / 4.0
		center := base + math.Sin(angle)*4.2 + math.Cos(angle/2.5)*1.3
		open := center - math.Sin(angle/3.0)*0.8
		closePrice := center + math.Cos(angle/2.0)*0.7
		high := math.Max(open, closePrice) + 0.9 + math.Abs(math.Sin(angle))*0.5
		low := math.Min(open, closePrice) - 0.8 - math.Abs(math.Cos(angle))*0.4

		items = append(items, model.Candle{
			Time:  pointTime.Format(time.RFC3339),
			Open:  round(open),
			High:  round(high),
			Low:   round(low),
			Close: round(closePrice),
		})
	}

	return model.PriceHistory{
		Symbol:   "AU_CNY_G",
		Interval: interval,
		Items:    items,
	}
}

func (s *MockMarketService) GetNewsList() []model.NewsArticle {
	now := time.Now()
	return []model.NewsArticle{
		{
			ID:             1,
			Title:          "美元指数回落，黄金短线获得支撑",
			Summary:        "美元回调压低持有黄金的机会成本，短线对金价形成偏多支撑。",
			URL:            "https://example.com/news/usd-gold",
			Region:         "US",
			Category:       "macro",
			Sentiment:      "positive",
			Importance:     5,
			ImpactScore:    82,
			RelatedFactors: []string{"usd_index", "fed_rate"},
			PublishedAt:    now.Add(-35 * time.Minute).Format(time.RFC3339),
		},
		{
			ID:             2,
			Title:          "中东局势升温，避险情绪推动贵金属关注度上升",
			Summary:        "地缘政治风险抬升，避险资产需求增加，黄金情绪分值上行。",
			URL:            "https://example.com/news/geopolitics",
			Region:         "Global",
			Category:       "geopolitics",
			Sentiment:      "positive",
			Importance:     5,
			ImpactScore:    88,
			RelatedFactors: []string{"geopolitics", "safe_haven_sentiment"},
			PublishedAt:    now.Add(-2 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:             3,
			Title:          "原油价格反弹，通胀预期边际回升",
			Summary:        "油价反弹可能抬升通胀预期，进而提高黄金的长期配置吸引力。",
			URL:            "https://example.com/news/oil-inflation",
			Region:         "Global",
			Category:       "market",
			Sentiment:      "neutral",
			Importance:     4,
			ImpactScore:    63,
			RelatedFactors: []string{"oil_price", "inflation"},
			PublishedAt:    now.Add(-4 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:             4,
			Title:          "亚洲实物金需求回暖，金店终端成交改善",
			Summary:        "节后实物需求回暖带来价格底部支撑，但持续性仍需观察。",
			URL:            "https://example.com/news/physical-demand",
			Region:         "CN",
			Category:       "industry",
			Sentiment:      "positive",
			Importance:     3,
			ImpactScore:    58,
			RelatedFactors: []string{"physical_demand"},
			PublishedAt:    now.Add(-7 * time.Hour).Format(time.RFC3339),
		},
	}
}

func (s *MockMarketService) GetNewsDetail(id int64) (model.NewsArticle, bool) {
	for _, item := range s.GetNewsList() {
		if item.ID == id {
			return item, true
		}
	}

	return model.NewsArticle{}, false
}

func (s *MockMarketService) GetLatestFactors() []model.FactorLatest {
	now := time.Now().Format(time.RFC3339)
	return []model.FactorLatest{
		{Code: "usd_index", Name: "美元指数", Value: 103.82, Unit: "", Score: -61.5, ImpactDirection: "bearish", ImpactStrength: 79.0, Confidence: 86.0, CapturedAt: now},
		{Code: "fed_rate", Name: "美联储利率", Value: 5.25, Unit: "%", Score: -54.0, ImpactDirection: "bearish", ImpactStrength: 73.0, Confidence: 82.0, CapturedAt: now},
		{Code: "inflation", Name: "通胀", Value: 2.9, Unit: "%", Score: 41.0, ImpactDirection: "bullish", ImpactStrength: 62.0, Confidence: 76.0, CapturedAt: now},
		{Code: "cny_fx", Name: "人民币汇率", Value: 7.18, Unit: "", Score: 28.0, ImpactDirection: "bullish", ImpactStrength: 48.0, Confidence: 71.0, CapturedAt: now},
		{Code: "safe_haven_sentiment", Name: "避险情绪", Value: 74.0, Unit: "score", Score: 69.0, ImpactDirection: "bullish", ImpactStrength: 84.0, Confidence: 88.0, CapturedAt: now},
		{Code: "central_bank_gold_buying", Name: "央行购金", Value: 63.0, Unit: "ton", Score: 52.0, ImpactDirection: "bullish", ImpactStrength: 68.0, Confidence: 75.0, CapturedAt: now},
		{Code: "oil_price", Name: "石油", Value: 81.4, Unit: "USD", Score: 24.0, ImpactDirection: "bullish", ImpactStrength: 39.0, Confidence: 65.0, CapturedAt: now},
		{Code: "equity_market", Name: "股市", Value: 4920.0, Unit: "pt", Score: -22.0, ImpactDirection: "bearish", ImpactStrength: 34.0, Confidence: 60.0, CapturedAt: now},
		{Code: "geopolitics", Name: "地缘政治", Value: 78.0, Unit: "score", Score: 72.0, ImpactDirection: "bullish", ImpactStrength: 87.0, Confidence: 91.0, CapturedAt: now},
		{Code: "physical_demand", Name: "实物需求", Value: 66.0, Unit: "score", Score: 37.0, ImpactDirection: "bullish", ImpactStrength: 51.0, Confidence: 70.0, CapturedAt: now},
	}
}

func (s *MockMarketService) GetFactorDefinitions() []model.FactorDefinition {
	return []model.FactorDefinition{
		{Code: "usd_index", Name: "美元指数", Category: "macro", Description: "美元走强通常压制黄金表现。", Unit: ""},
		{Code: "fed_rate", Name: "美联储利率", Category: "macro", Description: "实际利率上行通常利空黄金。", Unit: "%"},
		{Code: "inflation", Name: "通胀", Category: "macro", Description: "通胀升温提升黄金抗通胀属性。", Unit: "%"},
		{Code: "cny_fx", Name: "人民币汇率", Category: "macro", Description: "人民币汇率波动会影响人民币计价金价。", Unit: ""},
		{Code: "safe_haven_sentiment", Name: "避险情绪", Category: "event", Description: "风险事件上升时黄金需求增强。", Unit: "score"},
		{Code: "central_bank_gold_buying", Name: "央行购金", Category: "demand", Description: "央行持续增持为长期支撑因子。", Unit: "ton"},
		{Code: "oil_price", Name: "石油", Category: "market", Description: "油价变动会影响通胀预期。", Unit: "USD"},
		{Code: "equity_market", Name: "股市", Category: "market", Description: "风险偏好上升时可能分流黄金资金。", Unit: "pt"},
		{Code: "geopolitics", Name: "地缘政治", Category: "event", Description: "地缘冲突通常抬升避险资产需求。", Unit: "score"},
		{Code: "physical_demand", Name: "实物需求", Category: "demand", Description: "终端消费与珠宝需求影响价格支撑。", Unit: "score"},
	}
}

func (s *MockMarketService) GetFactorHistory(code, rangeValue string) model.FactorHistory {
	count := factorPointCount(rangeValue)
	now := time.Now()
	items := make([]model.FactorPoint, 0, count)
	base := float64(len(code))*2 + 20

	for i := count - 1; i >= 0; i-- {
		pointTime := now.Add(-time.Duration(i) * 24 * time.Hour)
		angle := float64(i) / 3.6
		value := base + math.Sin(angle)*6 + math.Cos(angle/2.0)*2
		score := math.Sin(angle/2.4) * 75

		items = append(items, model.FactorPoint{
			Time:  pointTime.Format(time.RFC3339),
			Value: round(value),
			Score: round(score),
		})
	}

	return model.FactorHistory{
		Code:  code,
		Range: rangeValue,
		Items: items,
	}
}

func (s *MockMarketService) GetLatestReport() model.ReportSummary {
	reports := s.GetReports()
	if len(reports) == 0 {
		return model.ReportSummary{}
	}

	return reports[0]
}

func (s *MockMarketService) GetReports() []model.ReportSummary {
	now := time.Now()
	reports := []model.ReportSummary{
		{
			ID:            3,
			ReportDate:    now.Format("2006-01-02"),
			Title:         "黄金短线偏强震荡，关注避险情绪与美元回落",
			Trend:         "bullish",
			Confidence:    82,
			Summary:       "避险情绪与美元偏弱形成共振，黄金短线维持偏强震荡格局，但需关注利率预期回摆。",
			KeyDrivers:    []string{"避险情绪升温", "美元指数回落", "央行购金延续"},
			RiskPoints:    []string{"美联储官员鹰派发言", "美元突然反弹"},
			AccuracyScore: 83,
			GeneratedAt:   now.Add(-20 * time.Minute).Format(time.RFC3339),
		},
		{
			ID:            2,
			ReportDate:    now.Add(-24 * time.Hour).Format("2006-01-02"),
			Title:         "黄金高位整理，等待宏观事件确认方向",
			Trend:         "range",
			Confidence:    74,
			Summary:       "市场在关键宏观数据前趋于谨慎，金价预计以区间震荡为主。",
			KeyDrivers:    []string{"宏观数据等待期", "美元横盘", "股市风险偏好回升"},
			RiskPoints:    []string{"突发地缘政治风险"},
			AccuracyScore: 76,
			GeneratedAt:   now.Add(-24*time.Hour + 15*time.Minute).Format(time.RFC3339),
		},
		{
			ID:            1,
			ReportDate:    now.Add(-48 * time.Hour).Format("2006-01-02"),
			Title:         "黄金或迎来温和反弹窗口",
			Trend:         "bullish",
			Confidence:    68,
			Summary:       "油价与通胀预期抬升，黄金反弹概率增加。",
			KeyDrivers:    []string{"通胀预期回升", "石油价格走高"},
			RiskPoints:    []string{"风险资产继续走强"},
			AccuracyScore: 71,
			GeneratedAt:   now.Add(-48*time.Hour + 10*time.Minute).Format(time.RFC3339),
		},
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ReportDate > reports[j].ReportDate
	})

	return reports
}

func (s *MockMarketService) GetReportDetail(id int64) (model.ReportDetail, bool) {
	for _, item := range s.GetReports() {
		if item.ID == id {
			return model.ReportDetail{
				ReportSummary: item,
				FullContent: strings.Join([]string{
					"一、行情判断：黄金当前维持偏强震荡，人民币计价金价在避险情绪推动下仍有上探动能。",
					"二、核心驱动：美元指数边际回落、地缘政治升温、央行购金延续，构成当前主要支撑。",
					"三、风险提示：若美联储释放更鹰派表态，或美元快速反弹，金价可能回吐短线涨幅。",
					"四、策略观察：关注夜盘波动和次日宏观事件窗口，重点观察 560-566 元/克区域表现。",
				}, "\n\n"),
			}, true
		}
	}

	return model.ReportDetail{}, false
}

func (s *MockMarketService) GetAccuracyCurve(rangeValue string) model.AccuracyCurve {
	count := factorPointCount(rangeValue)
	now := time.Now()
	items := make([]model.AccuracyItem, 0, count)
	total := 0.0

	for i := count - 1; i >= 0; i-- {
		angle := float64(i) / 2.8
		score := 76 + math.Sin(angle)*10 + math.Cos(angle/1.9)*4
		score = math.Max(52, math.Min(95, score))
		total += score

		items = append(items, model.AccuracyItem{
			ReportDate: now.Add(-time.Duration(i) * 24 * time.Hour).Format("2006-01-02"),
			Score:      round(score),
		})
	}

	return model.AccuracyCurve{
		AvgScore: round(total / float64(len(items))),
		Items:    items,
	}
}

func (s *MockMarketService) TriggerJob(jobName string) model.JobRun {
	id := s.jobCounter.Add(1)
	startedAt := time.Now().Add(-2 * time.Second)
	finishedAt := time.Now()

	return model.JobRun{
		ID:         id,
		JobName:    jobName,
		JobType:    jobType(jobName),
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    fmt.Sprintf("%s executed with mock pipeline", jobName),
	}
}

func (s *MockMarketService) GetJobRuns() []model.JobRun {
	names := []string{"collect-price", "fetch-news", "update-factors", "generate-report", "score-report"}
	runs := make([]model.JobRun, 0, len(names))

	for index, name := range names {
		startedAt := time.Now().Add(-time.Duration((index+1)*15) * time.Minute)
		runs = append(runs, model.JobRun{
			ID:         int64(index + 1),
			JobName:    name,
			JobType:    jobType(name),
			Status:     "success",
			StartedAt:  startedAt.Format(time.RFC3339),
			FinishedAt: startedAt.Add(2 * time.Second).Format(time.RFC3339),
			Message:    "completed",
		})
	}

	return runs
}

func defaultInterval(rangeValue string) string {
	switch rangeValue {
	case "1d":
		return "1m"
	case "7d", "30d":
		return "1h"
	default:
		return "1d"
	}
}

func historyConfig(rangeValue, interval string) (int, time.Duration) {
	switch {
	case rangeValue == "1d" && interval == "1m":
		return 240, time.Minute
	case rangeValue == "7d":
		return 168, time.Hour
	case rangeValue == "30d":
		return 180, 4 * time.Hour
	case rangeValue == "90d":
		return 90, 24 * time.Hour
	default:
		return 365, 24 * time.Hour
	}
}

func factorPointCount(rangeValue string) int {
	switch rangeValue {
	case "30d":
		return 30
	case "90d":
		return 90
	case "180d":
		return 180
	case "1y":
		return 365
	default:
		return 30
	}
}

func jobType(jobName string) string {
	switch jobName {
	case "collect-price", "fetch-news", "update-factors":
		return "collector"
	case "generate-report":
		return "report"
	case "score-report":
		return "scoring"
	default:
		return "collector"
	}
}

func round(value float64) float64 {
	return math.Round(value*1000) / 1000
}
