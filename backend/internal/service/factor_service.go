package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
)

type FactorService struct {
	repo      *repository.FactorRepository
	priceRepo *repository.PriceRepository
	newsRepo  *repository.NewsRepository
}

type factorBlueprint struct {
	Code                string
	Name                string
	Category            string
	Description         string
	ValueType           string
	Unit                string
	DefaultWeight       float64
	ImpactDirectionRule string
}

type factorContext struct {
	priceChangeRate float64
	newsSignals     map[string]float64
}

var factorBlueprints = []factorBlueprint{
	{Code: "usd_index", Name: "美元指数", Category: "macro", Description: "美元走强通常压制黄金表现。", ValueType: "number", Unit: "", DefaultWeight: 0.96, ImpactDirectionRule: "value 上行通常利空黄金"},
	{Code: "fed_rate", Name: "美联储利率", Category: "macro", Description: "实际利率上行通常利空黄金。", ValueType: "percent", Unit: "%", DefaultWeight: 0.92, ImpactDirectionRule: "利率上行通常利空黄金"},
	{Code: "inflation", Name: "通胀", Category: "macro", Description: "通胀升温提升黄金抗通胀配置价值。", ValueType: "percent", Unit: "%", DefaultWeight: 0.86, ImpactDirectionRule: "通胀上行通常利多黄金"},
	{Code: "cny_fx", Name: "人民币汇率", Category: "macro", Description: "人民币汇率波动影响人民币计价黄金价格。", ValueType: "number", Unit: "", DefaultWeight: 0.74, ImpactDirectionRule: "美元兑人民币上行通常利多人民币金价"},
	{Code: "safe_haven_sentiment", Name: "避险情绪", Category: "event", Description: "风险事件升温时黄金避险需求增强。", ValueType: "score", Unit: "score", DefaultWeight: 0.91, ImpactDirectionRule: "避险情绪上行通常利多黄金"},
	{Code: "central_bank_gold_buying", Name: "央行购金", Category: "demand", Description: "央行持续购金为长期需求支撑。", ValueType: "number", Unit: "ton", DefaultWeight: 0.79, ImpactDirectionRule: "购金规模上行通常利多黄金"},
	{Code: "oil_price", Name: "石油", Category: "market", Description: "油价变化会影响通胀预期与风险偏好。", ValueType: "number", Unit: "USD", DefaultWeight: 0.58, ImpactDirectionRule: "油价上行通常通过通胀预期间接利多黄金"},
	{Code: "equity_market", Name: "股市", Category: "market", Description: "风险偏好升温时股市走强可能分流黄金资金。", ValueType: "number", Unit: "pt", DefaultWeight: 0.55, ImpactDirectionRule: "股市走强通常利空黄金"},
	{Code: "geopolitics", Name: "地缘政治", Category: "event", Description: "地缘冲突通常抬升避险资产配置需求。", ValueType: "score", Unit: "score", DefaultWeight: 0.93, ImpactDirectionRule: "地缘风险上行通常利多黄金"},
	{Code: "physical_demand", Name: "实物需求", Category: "demand", Description: "珠宝消费和投资金条需求影响价格底部支撑。", ValueType: "score", Unit: "score", DefaultWeight: 0.63, ImpactDirectionRule: "实物需求上行通常利多黄金"},
}

var factorSourceMeta = source.SourceMeta{
	Code:     "internal_factor_engine",
	Name:     "Internal Factor Engine",
	Category: "macro",
	BaseURL:  "local://factor-engine",
	Priority: 1,
}

func NewFactorService(repo *repository.FactorRepository, priceRepo *repository.PriceRepository, newsRepo *repository.NewsRepository) *FactorService {
	return &FactorService{
		repo:      repo,
		priceRepo: priceRepo,
		newsRepo:  newsRepo,
	}
}

func (s *FactorService) Bootstrap(ctx context.Context) error {
	if err := s.ensureDefinitions(); err != nil {
		return err
	}

	count, err := s.repo.CountSnapshots()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	sourceRecord, err := s.repo.EnsureSource(factorSourceMeta)
	if err != nil {
		return err
	}

	definitions, err := s.repo.ListDefinitions()
	if err != nil {
		return err
	}

	now := time.Now()
	snapshots := make([]model.FactorSnapshotRecord, 0, len(definitions)*90)
	for dayOffset := 89; dayOffset >= 0; dayOffset-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		capturedAt := dailySnapshotTime(now.AddDate(0, 0, -dayOffset), now.Location())
		if dayOffset == 0 {
			capturedAt = now
		}
		for _, definition := range definitions {
			snapshots = append(snapshots, buildFactorSnapshot(definition, sourceRecord.ID, capturedAt, factorContext{}))
		}
	}

	return s.repo.SaveSnapshots(snapshots)
}

func (s *FactorService) UpdateNow(ctx context.Context) (model.JobRun, error) {
	startedAt := time.Now()
	if err := s.ensureDefinitions(); err != nil {
		return s.failJob(startedAt, err)
	}

	sourceRecord, err := s.repo.EnsureSource(factorSourceMeta)
	if err != nil {
		return s.failJob(startedAt, err)
	}

	definitions, err := s.repo.ListDefinitions()
	if err != nil {
		return s.failJob(startedAt, err)
	}

	contextData := s.buildContext()
	capturedAt := time.Now()
	snapshots := make([]model.FactorSnapshotRecord, 0, len(definitions))
	for _, definition := range definitions {
		select {
		case <-ctx.Done():
			return s.failJob(startedAt, ctx.Err())
		default:
		}
		snapshots = append(snapshots, buildFactorSnapshot(definition, sourceRecord.ID, capturedAt, contextData))
	}

	if err := s.repo.SaveSnapshots(snapshots); err != nil {
		return s.failJob(startedAt, err)
	}

	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "update-factors",
		JobType:    "collector",
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    fmt.Sprintf("factor snapshots updated: %d", len(snapshots)),
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

func (s *FactorService) GetLatestFactors() []model.FactorLatest {
	if err := s.ensureReady(); err != nil {
		return fallbackLatestFactors(time.Now())
	}

	definitions, err := s.repo.ListDefinitions()
	if err != nil || len(definitions) == 0 {
		return fallbackLatestFactors(time.Now())
	}

	items := make([]model.FactorLatest, 0, len(definitions))
	for _, definition := range definitions {
		snapshot, err := s.repo.GetLatestSnapshotByFactorID(definition.ID)
		if err != nil {
			return fallbackLatestFactors(time.Now())
		}
		items = append(items, model.FactorLatest{
			Code:            definition.Code,
			Name:            definition.Name,
			Value:           roundTo(snapshot.ValueNum, 3),
			Unit:            definition.Unit,
			Score:           roundTo(snapshot.Score, 2),
			ImpactDirection: snapshot.ImpactDirection,
			ImpactStrength:  roundTo(snapshot.ImpactStrength, 2),
			Confidence:      roundTo(snapshot.Confidence, 2),
			CapturedAt:      snapshot.CapturedAt.Format(time.RFC3339),
		})
	}

	return items
}

func (s *FactorService) GetFactorDefinitions() []model.FactorDefinition {
	if err := s.ensureDefinitions(); err != nil {
		return fallbackFactorDefinitions()
	}

	definitions, err := s.repo.ListDefinitions()
	if err != nil || len(definitions) == 0 {
		return fallbackFactorDefinitions()
	}

	definitionMap := make(map[string]model.FactorDefinitionRecord, len(definitions))
	for _, item := range definitions {
		definitionMap[item.Code] = item
	}

	items := make([]model.FactorDefinition, 0, len(factorBlueprints))
	for _, blueprint := range factorBlueprints {
		record, ok := definitionMap[blueprint.Code]
		if !ok {
			continue
		}
		items = append(items, model.FactorDefinition{
			Code:                record.Code,
			Name:                record.Name,
			Category:            record.Category,
			Description:         record.Description,
			Unit:                record.Unit,
			ValueType:           record.ValueType,
			DefaultWeight:       roundTo(record.DefaultWeight, 3),
			ImpactDirectionRule: record.ImpactDirectionRule,
		})
	}
	return items
}

func (s *FactorService) GetFactorHistory(code, rangeValue string) model.FactorHistory {
	if err := s.ensureReady(); err != nil {
		return fallbackFactorHistory(code, rangeValue)
	}

	definition, err := s.repo.GetDefinitionByCode(code)
	if err != nil {
		return fallbackFactorHistory(code, rangeValue)
	}

	start, end := factorTimeWindow(rangeValue)
	records, err := s.repo.ListSnapshotsByFactorID(definition.ID, start, end)
	if err != nil || len(records) == 0 {
		return fallbackFactorHistory(code, rangeValue)
	}

	items := make([]model.FactorPoint, 0, len(records))
	for _, item := range records {
		items = append(items, model.FactorPoint{
			Time:  item.CapturedAt.Format(time.RFC3339),
			Value: roundTo(item.ValueNum, 3),
			Score: roundTo(item.Score, 2),
		})
	}

	return model.FactorHistory{
		Code:  code,
		Range: rangeValue,
		Items: items,
	}
}

func (s *FactorService) ensureReady() error {
	if err := s.ensureDefinitions(); err != nil {
		return err
	}

	count, err := s.repo.CountSnapshots()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.Bootstrap(context.Background())
}

func (s *FactorService) ensureDefinitions() error {
	records := make([]model.FactorDefinitionRecord, 0, len(factorBlueprints))
	for _, item := range factorBlueprints {
		records = append(records, model.FactorDefinitionRecord{
			Code:                item.Code,
			Name:                item.Name,
			Category:            item.Category,
			Description:         item.Description,
			ValueType:           item.ValueType,
			Unit:                item.Unit,
			DefaultWeight:       item.DefaultWeight,
			ImpactDirectionRule: item.ImpactDirectionRule,
		})
	}
	return s.repo.UpsertDefinitions(records)
}

func (s *FactorService) buildContext() factorContext {
	contextData := factorContext{
		newsSignals: make(map[string]float64),
	}

	if s.priceRepo != nil {
		if latest, err := s.priceRepo.GetLatestTick(); err == nil {
			contextData.priceChangeRate = latest.ChangeRate
		}
	}

	if s.newsRepo == nil {
		return contextData
	}

	records, err := s.newsRepo.ListLatest(30)
	if err != nil {
		return contextData
	}

	for _, item := range records {
		weight := float64(item.Importance) + item.ImpactScore/25
		switch item.Sentiment {
		case "positive":
			weight *= 1
		case "negative":
			weight *= -0.8
		default:
			weight *= 0.25
		}

		var related []string
		_ = json.Unmarshal([]byte(item.RelatedFactorsJSON), &related)
		for _, code := range related {
			contextData.newsSignals[code] += weight
		}

		if item.Region == "CN" {
			contextData.newsSignals["physical_demand"] += weight * 0.2
			contextData.newsSignals["cny_fx"] += math.Abs(weight) * 0.08
		}
		if item.Category == "geopolitics" {
			contextData.newsSignals["geopolitics"] += math.Abs(weight) * 0.3
			contextData.newsSignals["safe_haven_sentiment"] += math.Abs(weight) * 0.2
		}
	}

	for code, signal := range contextData.newsSignals {
		contextData.newsSignals[code] = clamp(signal, -25, 25)
	}
	return contextData
}

func (s *FactorService) failJob(startedAt time.Time, err error) (model.JobRun, error) {
	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "update-factors",
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
		Message:     "factor update failed",
		ErrorDetail: err.Error(),
	})
	if saveErr != nil {
		return run, errors.Join(err, saveErr)
	}
	return run, err
}

func buildFactorSnapshot(definition model.FactorDefinitionRecord, sourceID uint, capturedAt time.Time, contextData factorContext) model.FactorSnapshotRecord {
	value, score, strength, confidence := calculateFactorMetrics(definition.Code, capturedAt, contextData)
	return model.FactorSnapshotRecord{
		FactorID:        definition.ID,
		SourceID:        sourceID,
		ValueNum:        roundTo(value, 4),
		Score:           roundTo(score, 2),
		ImpactDirection: scoreDirection(score),
		ImpactStrength:  roundTo(strength, 2),
		Confidence:      roundTo(confidence, 2),
		Summary:         buildFactorSummary(definition.Name, value, definition.Unit, score),
		CapturedAt:      capturedAt,
		CreatedAt:       time.Now(),
	}
}

func calculateFactorMetrics(code string, capturedAt time.Time, contextData factorContext) (float64, float64, float64, float64) {
	priceBias := clamp(contextData.priceChangeRate*2.5, -8, 8)
	newsBias := contextData.newsSignals[code]
	day := float64(capturedAt.Unix()) / 86400

	switch code {
	case "usd_index":
		value := 103.2 + 1.6*math.Sin(day/17+0.4) + 0.4*math.Cos(day/5)
		score := -((value - 103.2) * 28) - newsBias*1.3 - priceBias*0.2
		return value, clamp(score, -100, 100), 42 + math.Abs(score)*0.62, 86
	case "fed_rate":
		value := 4.7 + 0.35*math.Sin(day/29+1.2) + 0.12*math.Cos(day/11)
		score := -((value - 4.6) * 95) - newsBias*0.9
		return value, clamp(score, -100, 100), 38 + math.Abs(score)*0.58, 82
	case "inflation":
		value := 2.35 + 0.55*math.Sin(day/23+2.1) + 0.18*math.Cos(day/9)
		score := ((value - 2.2) * 70) + newsBias*0.8
		return value, clamp(score, -100, 100), 34 + math.Abs(score)*0.57, 78
	case "cny_fx":
		value := 7.14 + 0.09*math.Sin(day/19+0.8) + math.Abs(priceBias)*0.002
		score := ((value - 7.05) * 180) + newsBias*0.9
		return value, clamp(score, -100, 100), 28 + math.Abs(score)*0.65, 73
	case "safe_haven_sentiment":
		value := 52 + 8*math.Sin(day/13+0.9) + math.Max(newsBias, 0)*1.1
		score := (value - 50) * 1.6
		return value, clamp(score, -100, 100), 35 + math.Abs(score)*0.72, 88
	case "central_bank_gold_buying":
		value := 49 + 9*math.Sin(day/41+0.6) + math.Max(newsBias, 0)*0.35
		score := (value-45)*3.1 + newsBias*0.7
		return value, clamp(score, -100, 100), 26 + math.Abs(score)*0.69, 76
	case "oil_price":
		value := 79 + 4.5*math.Sin(day/15+1.7) + 1.3*math.Cos(day/6)
		score := (value-78)*5.2 + newsBias*0.7
		return value, clamp(score, -100, 100), 22 + math.Abs(score)*0.66, 68
	case "equity_market":
		value := 4920 + 180*math.Sin(day/18+2.4) + 55*math.Cos(day/7)
		score := -((value - 4900) / 7.5) - newsBias*0.8 + priceBias*0.1
		return value, clamp(score, -100, 100), 30 + math.Abs(score)*0.54, 64
	case "geopolitics":
		value := 46 + 9*math.Sin(day/21+1.1) + math.Max(newsBias, 0)*1.35
		score := (value-45)*1.9 + newsBias*0.9
		return value, clamp(score, -100, 100), 40 + math.Abs(score)*0.68, 90
	case "physical_demand":
		value := 55 + 6.5*math.Sin(day/27+2.8) + math.Max(newsBias, 0)*0.8
		score := (value-52)*2.7 + newsBias*0.8
		return value, clamp(score, -100, 100), 24 + math.Abs(score)*0.61, 72
	default:
		value := 50 + 5*math.Sin(day/20)
		score := (value - 50) * 1.5
		return value, clamp(score, -100, 100), 20 + math.Abs(score)*0.5, 70
	}
}

func buildFactorSummary(name string, value float64, unit string, score float64) string {
	unitSuffix := unit
	if unitSuffix != "" {
		unitSuffix = " " + unitSuffix
	}
	return fmt.Sprintf("%s最新值 %.3f%s，当前对黄金%s。", name, value, unitSuffix, directionLabel(score))
}

func fallbackLatestFactors(capturedAt time.Time) []model.FactorLatest {
	definitions := fallbackFactorDefinitions()
	items := make([]model.FactorLatest, 0, len(definitions))
	for _, definition := range definitions {
		record := model.FactorDefinitionRecord{
			Code: definition.Code,
			Name: definition.Name,
			Unit: definition.Unit,
		}
		value, score, strength, confidence := calculateFactorMetrics(definition.Code, capturedAt, factorContext{})
		items = append(items, model.FactorLatest{
			Code:            record.Code,
			Name:            record.Name,
			Value:           roundTo(value, 3),
			Unit:            record.Unit,
			Score:           roundTo(score, 2),
			ImpactDirection: scoreDirection(score),
			ImpactStrength:  roundTo(strength, 2),
			Confidence:      roundTo(confidence, 2),
			CapturedAt:      capturedAt.Format(time.RFC3339),
		})
	}
	return items
}

func fallbackFactorDefinitions() []model.FactorDefinition {
	items := make([]model.FactorDefinition, 0, len(factorBlueprints))
	for _, item := range factorBlueprints {
		items = append(items, model.FactorDefinition{
			Code:                item.Code,
			Name:                item.Name,
			Category:            item.Category,
			Description:         item.Description,
			Unit:                item.Unit,
			ValueType:           item.ValueType,
			DefaultWeight:       item.DefaultWeight,
			ImpactDirectionRule: item.ImpactDirectionRule,
		})
	}
	return items
}

func fallbackFactorHistory(code, rangeValue string) model.FactorHistory {
	start, end := factorTimeWindow(rangeValue)
	if start.After(end) {
		start, end = end, start
	}

	items := make([]model.FactorPoint, 0)
	current := dailySnapshotTime(start, time.Local)
	for !current.After(end) {
		value, score, _, _ := calculateFactorMetrics(code, current, factorContext{})
		items = append(items, model.FactorPoint{
			Time:  current.Format(time.RFC3339),
			Value: roundTo(value, 3),
			Score: roundTo(score, 2),
		})
		current = current.Add(24 * time.Hour)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Time < items[j].Time
	})
	return model.FactorHistory{Code: code, Range: rangeValue, Items: items}
}

func factorTimeWindow(rangeValue string) (time.Time, time.Time) {
	now := time.Now()
	switch rangeValue {
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

func dailySnapshotTime(value time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	return time.Date(value.In(loc).Year(), value.In(loc).Month(), value.In(loc).Day(), 9, 0, 0, 0, loc)
}

func scoreDirection(score float64) string {
	switch {
	case score > 8:
		return "bullish"
	case score < -8:
		return "bearish"
	default:
		return "neutral"
	}
}

func directionLabel(score float64) string {
	switch scoreDirection(score) {
	case "bullish":
		return "偏利多"
	case "bearish":
		return "偏利空"
	default:
		return "偏中性"
	}
}

func clamp(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}

func roundTo(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
