package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
)

const (
	reportAIProvider    = "rule-engine"
	reportModelName     = "local-phase5-v1"
	reportPromptVersion = "phase5-rule-v1"
)

type ReportService struct {
	repo       *repository.ReportRepository
	priceRepo  *repository.PriceRepository
	factorRepo *repository.FactorRepository
	newsRepo   *repository.NewsRepository
}

type reportFactorSignal struct {
	Code  string
	Name  string
	Value float64
	Unit  string
	Score float64
}

type reportBuildResult struct {
	record      model.AnalysisReportRecord
	predictions []model.ReportPredictionRecord
}

type marketOutcome struct {
	Date            time.Time
	Open            float64
	Close           float64
	High            float64
	Low             float64
	Direction       string
	DominantFactors []string
	RiskTags        []string
}

func NewReportService(repo *repository.ReportRepository, priceRepo *repository.PriceRepository, factorRepo *repository.FactorRepository, newsRepo *repository.NewsRepository) *ReportService {
	return &ReportService{
		repo:       repo,
		priceRepo:  priceRepo,
		factorRepo: factorRepo,
		newsRepo:   newsRepo,
	}
}

func (s *ReportService) Bootstrap(ctx context.Context) error {
	count, err := s.repo.CountReports()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for dayOffset := 29; dayOffset >= 0; dayOffset-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reportDate := time.Now().AddDate(0, 0, -dayOffset)
		if _, err := s.GenerateNow(ctx, reportDate.Format("2006-01-02")); err != nil {
			return err
		}
		if dayOffset > 0 {
			if _, err := s.ScoreNow(ctx, reportDate.Format("2006-01-02")); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ReportService) GenerateNow(ctx context.Context, reportDate string) (model.JobRun, error) {
	startedAt := time.Now()
	date, err := resolveReportDate(reportDate, time.Now())
	if err != nil {
		return s.failJob("generate-report", "report", startedAt, err)
	}

	build, err := s.buildReport(ctx, date)
	if err != nil {
		return s.failJob("generate-report", "report", startedAt, err)
	}

	record, err := s.repo.UpsertReport(build.record)
	if err != nil {
		return s.failJob("generate-report", "report", startedAt, err)
	}

	predictions := make([]model.ReportPredictionRecord, 0, len(build.predictions))
	for _, item := range build.predictions {
		item.ReportID = record.ID
		predictions = append(predictions, item)
	}
	if err := s.repo.ReplacePredictions(record.ID, predictions); err != nil {
		return s.failJob("generate-report", "report", startedAt, err)
	}

	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "generate-report",
		JobType:    "report",
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    fmt.Sprintf("report generated for %s", date.Format("2006-01-02")),
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

func (s *ReportService) ScoreNow(ctx context.Context, reportDate string) (model.JobRun, error) {
	startedAt := time.Now()
	defaultDate := time.Now().AddDate(0, 0, -1)
	date, err := resolveReportDate(reportDate, defaultDate)
	if err != nil {
		return s.failJob("score-report", "scoring", startedAt, err)
	}

	report, err := s.repo.GetReportByDate(date.Format("2006-01-02"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := s.GenerateNow(ctx, date.Format("2006-01-02")); err != nil {
			return s.failJob("score-report", "scoring", startedAt, err)
		}
		report, err = s.repo.GetReportByDate(date.Format("2006-01-02"))
	}
	if err != nil {
		return s.failJob("score-report", "scoring", startedAt, err)
	}

	predictions, err := s.repo.ListPredictions(report.ID)
	if err != nil {
		return s.failJob("score-report", "scoring", startedAt, err)
	}
	if len(predictions) == 0 {
		return s.failJob("score-report", "scoring", startedAt, errors.New("no predictions found for report"))
	}

	scoreRecord := s.buildScore(report, predictions[0])
	if _, err := s.repo.UpsertScore(scoreRecord); err != nil {
		return s.failJob("score-report", "scoring", startedAt, err)
	}

	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "score-report",
		JobType:    "scoring",
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    fmt.Sprintf("report scored for %s", date.Format("2006-01-02")),
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

func (s *ReportService) GetLatestReport() model.ReportSummary {
	if err := s.ensureReady(); err != nil {
		return NewMockMarketService().GetLatestReport()
	}

	record, err := s.repo.GetLatestReport()
	if err != nil {
		return NewMockMarketService().GetLatestReport()
	}
	return s.toReportSummary(record)
}

func (s *ReportService) ListReports(query model.ReportQuery) model.ReportList {
	if err := s.ensureReady(); err != nil {
		mock := NewMockMarketService().ListReports(query)
		page := normalizePage(query.Page)
		pageSize := normalizePageSize(query.PageSize)
		return model.ReportList{Items: mock.Items, Page: page, PageSize: pageSize, Total: mock.Total}
	}

	records, total, err := s.repo.ListReports(query)
	if err != nil {
		mock := NewMockMarketService().ListReports(query)
		page := normalizePage(query.Page)
		pageSize := normalizePageSize(query.PageSize)
		return model.ReportList{Items: mock.Items, Page: page, PageSize: pageSize, Total: mock.Total}
	}

	items := make([]model.ReportSummary, 0, len(records))
	for _, item := range records {
		items = append(items, s.toReportSummary(item))
	}

	return model.ReportList{
		Items:    items,
		Page:     normalizePage(query.Page),
		PageSize: normalizePageSize(query.PageSize),
		Total:    total,
	}
}

func (s *ReportService) GetReportDetail(id int64) (model.ReportDetail, bool) {
	if err := s.ensureReady(); err != nil {
		return NewMockMarketService().GetReportDetail(id)
	}

	record, err := s.repo.GetReportByID(id)
	if err != nil {
		return NewMockMarketService().GetReportDetail(id)
	}

	predictions, err := s.repo.ListPredictions(record.ID)
	if err != nil {
		return NewMockMarketService().GetReportDetail(id)
	}

	scoreRecord, scoreErr := s.repo.GetScoreByReportID(record.ID)
	var score *model.ReportScoreDetail
	if scoreErr == nil {
		score = &model.ReportScoreDetail{
			ScoredDate:       scoreRecord.ScoredDate,
			DirectionScore:   roundTo(scoreRecord.DirectionScore, 2),
			RangeScore:       roundTo(scoreRecord.RangeScore, 2),
			FactorHitScore:   roundTo(scoreRecord.FactorHitScore, 2),
			RiskScore:        roundTo(scoreRecord.RiskScore, 2),
			TotalScore:       roundTo(scoreRecord.TotalScore, 2),
			ActualClose:      roundTo(scoreRecord.ActualClose, 3),
			ActualHigh:       roundTo(scoreRecord.ActualHigh, 3),
			ActualLow:        roundTo(scoreRecord.ActualLow, 3),
			ScoreExplanation: scoreRecord.ScoreExplanation,
		}
	}

	keyDrivers := decodeJSONStringArray(record.KeyDriversJSON)
	riskPoints := decodeJSONStringArray(record.RiskPointsJSON)
	return model.ReportDetail{
		ReportSummary: model.ReportSummary{
			ID:            int64(record.ID),
			ReportDate:    record.ReportDate,
			Title:         record.Title,
			Trend:         record.Trend,
			Confidence:    roundTo(record.Confidence, 2),
			Summary:       record.Summary,
			KeyDrivers:    keyDrivers,
			RiskPoints:    riskPoints,
			AccuracyScore: s.lookupAccuracyScore(record.ID),
			GeneratedAt:   record.GeneratedAt.Format(time.RFC3339),
		},
		FullContent:   record.FullContent,
		AIProvider:    record.AIProvider,
		ModelName:     record.ModelName,
		PromptVersion: record.PromptVersion,
		Predictions:   toReportPredictions(predictions),
		Score:         score,
	}, true
}

func (s *ReportService) GetAccuracyCurve(rangeValue string) model.AccuracyCurve {
	if err := s.ensureReady(); err != nil {
		return NewMockMarketService().GetAccuracyCurve(rangeValue)
	}

	startDate, endDate := reportDateWindow(rangeValue)
	records, err := s.repo.ListScores(startDate, endDate)
	if err != nil || len(records) == 0 {
		return NewMockMarketService().GetAccuracyCurve(rangeValue)
	}

	items := make([]model.AccuracyItem, 0, len(records))
	total := 0.0
	for _, item := range records {
		total += item.TotalScore
		items = append(items, model.AccuracyItem{
			ReportDate: item.ScoredDate,
			Score:      roundTo(item.TotalScore, 2),
		})
	}

	return model.AccuracyCurve{
		AvgScore: roundTo(total/float64(len(items)), 2),
		Items:    items,
	}
}

func (s *ReportService) ensureReady() error {
	count, err := s.repo.CountReports()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.Bootstrap(context.Background())
}

func (s *ReportService) buildReport(ctx context.Context, reportDate time.Time) (reportBuildResult, error) {
	select {
	case <-ctx.Done():
		return reportBuildResult{}, ctx.Err()
	default:
	}

	factors := s.collectFactorSignals(reportDate)
	if len(factors) == 0 {
		return reportBuildResult{}, errors.New("no factor signals available")
	}

	sort.Slice(factors, func(i, j int) bool {
		return math.Abs(factors[i].Score) > math.Abs(factors[j].Score)
	})

	currentOutcome := s.syntheticMarketOutcome(reportDate)
	previousOutcome := s.syntheticMarketOutcome(reportDate.AddDate(0, 0, -1))
	priceMomentum := currentOutcome.Close - previousOutcome.Close
	compositeScore := averageFactorScore(factors[:minInt(5, len(factors))])
	topBullish, topBearish := splitTopFactors(factors)
	headlines := s.latestHeadlineTitles()

	trend := reportTrend(compositeScore, priceMomentum)
	confidence := clamp(64+math.Abs(compositeScore)*0.45+math.Abs(priceMomentum)*3, 52, 92)
	predictedClose := currentOutcome.Close + compositeScore*0.045 + priceMomentum*0.35
	rangeWidth := 2.4 + math.Abs(compositeScore)*0.018 + math.Abs(priceMomentum)*0.4
	predictedLow := predictedClose - rangeWidth
	predictedHigh := predictedClose + rangeWidth
	predictedDirection := closeDirection(currentOutcome.Close, predictedClose)

	keyDrivers := topBullish
	if len(keyDrivers) == 0 {
		keyDrivers = []string{"价格动能平稳", "市场等待新增催化"}
	}
	if len(keyDrivers) > 3 {
		keyDrivers = keyDrivers[:3]
	}

	riskPoints := topBearish
	if len(riskPoints) == 0 {
		riskPoints = []string{"美元与利率波动反复", "市场情绪可能快速切换"}
	}
	if len(riskPoints) > 3 {
		riskPoints = riskPoints[:3]
	}

	title := buildReportTitle(trend, keyDrivers)
	summary := buildReportSummary(trend, keyDrivers, riskPoints)
	fullContent := buildReportContent(reportDate, currentOutcome.Close, predictedLow, predictedHigh, predictedClose, keyDrivers, riskPoints, headlines)

	inputSnapshot := map[string]any{
		"reference_price": roundTo(currentOutcome.Close, 3),
		"price_momentum":  roundTo(priceMomentum, 3),
		"composite_score": roundTo(compositeScore, 2),
		"factor_count":    len(factors),
		"headlines":       headlines,
	}
	inputSnapshotJSON, _ := json.Marshal(inputSnapshot)
	keyDriversJSON, _ := json.Marshal(keyDrivers)
	riskPointsJSON, _ := json.Marshal(riskPoints)
	factorFocusJSON, _ := json.Marshal(limitStrings(keyDrivers, 3))

	record := model.AnalysisReportRecord{
		ReportDate:        reportDate.Format("2006-01-02"),
		Title:             title,
		Trend:             trend,
		Confidence:        roundTo(confidence, 2),
		Summary:           summary,
		FullContent:       fullContent,
		KeyDriversJSON:    string(keyDriversJSON),
		RiskPointsJSON:    string(riskPointsJSON),
		InputSnapshotJSON: string(inputSnapshotJSON),
		AIProvider:        reportAIProvider,
		ModelName:         reportModelName,
		PromptVersion:     reportPromptVersion,
		GeneratedAt:       time.Now(),
	}

	prediction := model.ReportPredictionRecord{
		TargetDate:         reportDate.AddDate(0, 0, 1).Format("2006-01-02"),
		PredictedDirection: predictedDirection,
		PredictedLow:       roundTo(predictedLow, 3),
		PredictedHigh:      roundTo(predictedHigh, 3),
		PredictedClose:     roundTo(predictedClose, 3),
		FactorFocusJSON:    string(factorFocusJSON),
	}

	return reportBuildResult{record: record, predictions: []model.ReportPredictionRecord{prediction}}, nil
}

func (s *ReportService) buildScore(report model.AnalysisReportRecord, prediction model.ReportPredictionRecord) model.ReportScoreRecord {
	targetDate, _ := time.Parse("2006-01-02", prediction.TargetDate)
	targetOutcome := s.syntheticMarketOutcome(targetDate)
	previousOutcome := s.syntheticMarketOutcome(targetDate.AddDate(0, 0, -1))

	directionScore := 8.0
	actualDirection := closeDirection(previousOutcome.Close, targetOutcome.Close)
	if prediction.PredictedDirection == actualDirection {
		directionScore = 35
	} else if prediction.PredictedDirection == "flat" && math.Abs(targetOutcome.Close-previousOutcome.Close) <= 0.55 {
		directionScore = 24
	}

	rangeScore := 0.0
	if targetOutcome.Close >= prediction.PredictedLow && targetOutcome.Close <= prediction.PredictedHigh {
		rangeScore = 30
	} else {
		distance := math.Min(math.Abs(targetOutcome.Close-prediction.PredictedLow), math.Abs(targetOutcome.Close-prediction.PredictedHigh))
		rangeScore = clamp(30-distance*8, 0, 30)
	}

	predictedFactors := decodeJSONStringArray(prediction.FactorFocusJSON)
	factorHits := 0
	for _, item := range predictedFactors {
		if containsAny(strings.ToLower(strings.Join(targetOutcome.DominantFactors, " ")), strings.ToLower(item)) {
			factorHits++
		}
	}
	factorHitScore := clamp(float64(factorHits)*6.5, 0, 20)

	reportRisks := decodeJSONStringArray(report.RiskPointsJSON)
	riskHits := 0
	for _, risk := range reportRisks {
		lowerRisk := strings.ToLower(risk)
		for _, tag := range targetOutcome.RiskTags {
			if strings.Contains(lowerRisk, strings.ToLower(tag)) || strings.Contains(strings.ToLower(tag), lowerRisk) {
				riskHits++
				break
			}
		}
	}
	riskScore := clamp(float64(riskHits)*5, 0, 15)
	totalScore := clamp(directionScore+rangeScore+factorHitScore+riskScore, 0, 100)

	explanation := fmt.Sprintf(
		"方向得分 %.1f，区间得分 %.1f，因子命中 %.1f，风险提示 %.1f。目标日实际区间 %.3f-%.3f，收盘 %.3f。",
		directionScore,
		rangeScore,
		factorHitScore,
		riskScore,
		targetOutcome.Low,
		targetOutcome.High,
		targetOutcome.Close,
	)

	return model.ReportScoreRecord{
		ReportID:         report.ID,
		ScoredDate:       prediction.TargetDate,
		DirectionScore:   roundTo(directionScore, 2),
		RangeScore:       roundTo(rangeScore, 2),
		FactorHitScore:   roundTo(factorHitScore, 2),
		RiskScore:        roundTo(riskScore, 2),
		TotalScore:       roundTo(totalScore, 2),
		ActualClose:      roundTo(targetOutcome.Close, 3),
		ActualHigh:       roundTo(targetOutcome.High, 3),
		ActualLow:        roundTo(targetOutcome.Low, 3),
		ScoreExplanation: explanation,
	}
}

func (s *ReportService) collectFactorSignals(reportDate time.Time) []reportFactorSignal {
	if s.factorRepo != nil && sameDay(reportDate, time.Now()) {
		definitions, err := s.factorRepo.ListDefinitions()
		if err == nil && len(definitions) > 0 {
			items := make([]reportFactorSignal, 0, len(definitions))
			for _, definition := range definitions {
				snapshot, err := s.factorRepo.GetLatestSnapshotByFactorID(definition.ID)
				if err != nil {
					continue
				}
				items = append(items, reportFactorSignal{
					Code:  definition.Code,
					Name:  definition.Name,
					Value: snapshot.ValueNum,
					Unit:  definition.Unit,
					Score: snapshot.Score,
				})
			}
			if len(items) > 0 {
				return items
			}
		}
	}

	definitions := fallbackFactorDefinitions()
	items := make([]reportFactorSignal, 0, len(definitions))
	for _, item := range definitions {
		value, score, _, _ := calculateFactorMetrics(item.Code, reportDate, factorContext{})
		items = append(items, reportFactorSignal{
			Code:  item.Code,
			Name:  item.Name,
			Value: value,
			Unit:  item.Unit,
			Score: score,
		})
	}
	return items
}

func (s *ReportService) latestHeadlineTitles() []string {
	if s.newsRepo == nil {
		return nil
	}

	records, err := s.newsRepo.ListLatest(3)
	if err != nil {
		return nil
	}

	items := make([]string, 0, len(records))
	for _, item := range records {
		items = append(items, item.Title)
	}
	return items
}

func (s *ReportService) syntheticMarketOutcome(date time.Time) marketOutcome {
	factors := s.collectFactorSignals(date)
	sort.Slice(factors, func(i, j int) bool {
		return math.Abs(factors[i].Score) > math.Abs(factors[j].Score)
	})

	positiveScore := 0.0
	negativeScore := 0.0
	for _, item := range factors {
		if item.Score > 0 {
			positiveScore += item.Score
		} else {
			negativeScore += math.Abs(item.Score)
		}
	}
	compositeScore := averageFactorScore(factors[:minInt(5, len(factors))])
	day := float64(date.Unix()) / 86400
	base := 558 + math.Sin(day/8.0)*6.8 + math.Cos(day/3.4)*1.7
	open := base + math.Sin(day/5.3)*0.9
	closePrice := base + compositeScore*0.045 + (positiveScore-negativeScore)*0.004
	high := math.Max(open, closePrice) + 1.9 + positiveScore*0.006
	low := math.Min(open, closePrice) - 1.7 - negativeScore*0.006

	topFactors := factorNamesBySign(factors, true)
	riskTags := factorNamesBySign(factors, false)
	if len(riskTags) == 0 {
		riskTags = []string{"波动放大"}
	}

	return marketOutcome{
		Date:            date,
		Open:            roundTo(open, 3),
		Close:           roundTo(closePrice, 3),
		High:            roundTo(high, 3),
		Low:             roundTo(low, 3),
		Direction:       closeDirection(open, closePrice),
		DominantFactors: limitStrings(topFactors, 3),
		RiskTags:        limitStrings(riskTags, 3),
	}
}

func (s *ReportService) toReportSummary(record model.AnalysisReportRecord) model.ReportSummary {
	return model.ReportSummary{
		ID:            int64(record.ID),
		ReportDate:    record.ReportDate,
		Title:         record.Title,
		Trend:         record.Trend,
		Confidence:    roundTo(record.Confidence, 2),
		Summary:       record.Summary,
		KeyDrivers:    decodeJSONStringArray(record.KeyDriversJSON),
		RiskPoints:    decodeJSONStringArray(record.RiskPointsJSON),
		AccuracyScore: s.lookupAccuracyScore(record.ID),
		GeneratedAt:   record.GeneratedAt.Format(time.RFC3339),
	}
}

func (s *ReportService) lookupAccuracyScore(reportID uint) float64 {
	scoreRecord, err := s.repo.GetScoreByReportID(reportID)
	if err != nil {
		return 0
	}
	return roundTo(scoreRecord.TotalScore, 2)
}

func (s *ReportService) failJob(jobName, jobType string, startedAt time.Time, err error) (model.JobRun, error) {
	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    jobName,
		JobType:    jobType,
		Status:     "failed",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    err.Error(),
	}

	saveErr := s.repo.SaveJobRun(model.JobRunRecord{
		JobName:     jobName,
		JobType:     jobType,
		Status:      "failed",
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		DurationMS:  int(finishedAt.Sub(startedAt).Milliseconds()),
		Message:     fmt.Sprintf("%s failed", jobName),
		ErrorDetail: err.Error(),
	})
	if saveErr != nil {
		return run, errors.Join(err, saveErr)
	}
	return run, err
}

func resolveReportDate(value string, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), 0, 0, 0, 0, fallback.Location()), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, errors.New("invalid report_date, expected YYYY-MM-DD")
	}
	return parsed, nil
}

func reportDateWindow(rangeValue string) (string, string) {
	now := time.Now()
	switch rangeValue {
	case "90d":
		return now.AddDate(0, 0, -90).Format("2006-01-02"), now.Format("2006-01-02")
	case "180d":
		return now.AddDate(0, 0, -180).Format("2006-01-02"), now.Format("2006-01-02")
	case "1y":
		return now.AddDate(-1, 0, 0).Format("2006-01-02"), now.Format("2006-01-02")
	default:
		return now.AddDate(0, 0, -30).Format("2006-01-02"), now.Format("2006-01-02")
	}
}

func averageFactorScore(items []reportFactorSignal) float64 {
	if len(items) == 0 {
		return 0
	}
	total := 0.0
	for _, item := range items {
		total += item.Score
	}
	return total / float64(len(items))
}

func splitTopFactors(items []reportFactorSignal) ([]string, []string) {
	positive := make([]string, 0, 3)
	negative := make([]string, 0, 3)
	for _, item := range items {
		switch {
		case item.Score > 8 && len(positive) < 3:
			positive = append(positive, item.Name)
		case item.Score < -8 && len(negative) < 3:
			negative = append(negative, item.Name)
		}
	}
	return positive, negative
}

func reportTrend(compositeScore, momentum float64) string {
	switch {
	case compositeScore > 18 && momentum > -0.2:
		return "bullish"
	case compositeScore < -18 && momentum < 0.2:
		return "bearish"
	case math.Abs(compositeScore) <= 10:
		return "range"
	default:
		return "volatile"
	}
}

func buildReportTitle(trend string, keyDrivers []string) string {
	driverText := "关注多空因子再平衡"
	if len(keyDrivers) > 0 {
		driverText = strings.Join(limitStrings(keyDrivers, 2), "与")
	}

	switch trend {
	case "bullish":
		return "黄金短线偏强，重点关注" + driverText
	case "bearish":
		return "黄金存在回压风险，关注" + driverText
	case "range":
		return "黄金维持区间整理，关注" + driverText
	default:
		return "黄金波动或放大，关注" + driverText
	}
}

func buildReportSummary(trend string, keyDrivers, riskPoints []string) string {
	driverText := strings.Join(limitStrings(keyDrivers, 2), "、")
	riskText := strings.Join(limitStrings(riskPoints, 2), "、")
	if driverText == "" {
		driverText = "多空因子暂未形成单边共振"
	}
	if riskText == "" {
		riskText = "关注市场情绪快速切换"
	}

	switch trend {
	case "bullish":
		return fmt.Sprintf("%s对金价形成主要支撑，短线偏多判断有效，但仍需防范%s。", driverText, riskText)
	case "bearish":
		return fmt.Sprintf("%s对金价形成主要压制，短线偏空概率更高，需警惕%s。", driverText, riskText)
	case "range":
		return fmt.Sprintf("%s尚未形成单边趋势，金价更可能延续区间整理，重点观察%s。", driverText, riskText)
	default:
		return fmt.Sprintf("%s相互拉扯，金价波动或放大，重点留意%s。", driverText, riskText)
	}
}

func buildReportContent(reportDate time.Time, referencePrice, predictedLow, predictedHigh, predictedClose float64, keyDrivers, riskPoints, headlines []string) string {
	sections := []string{
		fmt.Sprintf("一、时间窗口：报告日期 %s，参考价格 %.3f 元/克。", reportDate.Format("2006-01-02"), referencePrice),
		fmt.Sprintf("二、走势判断：下一交易日预估区间 %.3f - %.3f 元/克，预估收盘 %.3f 元/克。", predictedLow, predictedHigh, predictedClose),
		"三、核心驱动：" + strings.Join(limitStrings(keyDrivers, 3), "、"),
		"四、风险提示：" + strings.Join(limitStrings(riskPoints, 3), "、"),
	}
	if len(headlines) > 0 {
		sections = append(sections, "五、新闻观察："+strings.Join(limitStrings(headlines, 3), "；"))
	}
	return strings.Join(sections, "\n\n")
}

func closeDirection(openPrice, closePrice float64) string {
	change := closePrice - openPrice
	switch {
	case change > 0.35:
		return "up"
	case change < -0.35:
		return "down"
	default:
		return "flat"
	}
}

func factorNamesBySign(items []reportFactorSignal, positive bool) []string {
	names := make([]string, 0, 3)
	for _, item := range items {
		if positive && item.Score > 8 {
			names = append(names, item.Name)
		}
		if !positive && item.Score < -8 {
			names = append(names, item.Name)
		}
		if len(names) >= 3 {
			break
		}
	}
	return names
}

func limitStrings(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func decodeJSONStringArray(value string) []string {
	var items []string
	_ = json.Unmarshal([]byte(value), &items)
	return items
}

func toReportPredictions(records []model.ReportPredictionRecord) []model.ReportPrediction {
	items := make([]model.ReportPrediction, 0, len(records))
	for _, item := range records {
		items = append(items, model.ReportPrediction{
			TargetDate:         item.TargetDate,
			PredictedDirection: item.PredictedDirection,
			PredictedLow:       roundTo(item.PredictedLow, 3),
			PredictedHigh:      roundTo(item.PredictedHigh, 3),
			PredictedClose:     roundTo(item.PredictedClose, 3),
			FactorFocus:        decodeJSONStringArray(item.FactorFocusJSON),
		})
	}
	return items
}

func sameDay(a, b time.Time) bool {
	aa := a.In(time.Local)
	bb := b.In(time.Local)
	return aa.Year() == bb.Year() && aa.YearDay() == bb.YearDay()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
