package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
	"gorm.io/gorm"
)

const (
	maxRealtimeJumpPercent = 8.0
	maxHistoryJumpPercent  = 12.0
	maxFutureSkew          = 2 * time.Minute
)

type PriceCollector struct {
	repo     *repository.PriceRepository
	provider source.PriceProvider
}

func NewPriceCollector(repo *repository.PriceRepository, provider source.PriceProvider) *PriceCollector {
	return &PriceCollector{repo: repo, provider: provider}
}

func (c *PriceCollector) BootstrapHistory(ctx context.Context) error {
	count, err := c.repo.CountTicks()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	quotes, err := c.provider.HistoricalTicks(ctx, 1440, time.Minute)
	if err != nil {
		return err
	}
	quotes = sanitizeHistoryQuotes(quotes)
	if len(quotes) == 0 {
		return errors.New("no valid bootstrap quotes")
	}

	sourceRecord, err := c.repo.EnsureSource(c.provider.Metadata())
	if err != nil {
		return err
	}

	if err := c.repo.SaveTicks(sourceRecord.ID, quotes); err != nil {
		return err
	}

	return c.seedCandles(quotes)
}

func (c *PriceCollector) CollectNow(ctx context.Context) (model.JobRun, error) {
	return c.CollectWithOptions(ctx, manualJobRunOptions())
}

func (c *PriceCollector) CollectWithOptions(ctx context.Context, options JobRunOptions) (model.JobRun, error) {
	options = normalizeJobRunOptions(options, "manual")
	startedAt := time.Now()
	quote, err := c.provider.CurrentPrice(ctx)
	if err != nil {
		return c.failJob(startedAt, err, options)
	}
	quote, err = c.validateRealtimeQuote(quote)
	if err != nil {
		return c.failJob(startedAt, err, options)
	}

	sourceRecord, err := c.repo.EnsureSource(c.provider.Metadata())
	if err != nil {
		return c.failJob(startedAt, err, options)
	}

	if _, err := c.repo.SaveTick(sourceRecord.ID, quote); err != nil {
		return c.failJob(startedAt, err, options)
	}

	if err := c.rebuildCandles(quote.Symbol, quote.CapturedAt); err != nil {
		return c.failJob(startedAt, err, options)
	}

	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "collect-price",
		JobType:    "collector",
		Status:     "success",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    "gold price collected and candles updated",
	}
	fillJobRunMeta(&run, options, int(finishedAt.Sub(startedAt).Milliseconds()), nil)

	err = c.repo.SaveJobRun(model.JobRunRecord{
		JobName:      run.JobName,
		JobType:      run.JobType,
		Status:       run.Status,
		TriggerMode:  run.TriggerMode,
		Attempt:      run.Attempt,
		MaxAttempts:  run.MaxAttempts,
		ScheduledFor: options.ScheduledFor,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		DurationMS:   run.DurationMS,
		Message:      run.Message,
	})

	return run, err
}

func (c *PriceCollector) seedCandles(quotes []source.PriceQuote) error {
	grouped := map[string]map[time.Time][]source.PriceQuote{
		"1m":  {},
		"5m":  {},
		"15m": {},
		"1h":  {},
		"1d":  {},
	}

	durations := map[string]time.Duration{
		"1m":  time.Minute,
		"5m":  5 * time.Minute,
		"15m": 15 * time.Minute,
		"1h":  time.Hour,
		"1d":  24 * time.Hour,
	}

	for _, quote := range quotes {
		for interval, duration := range durations {
			windowStart := quote.CapturedAt.Truncate(duration)
			grouped[interval][windowStart] = append(grouped[interval][windowStart], quote)
		}
	}

	for interval, buckets := range grouped {
		for windowStart, bucket := range buckets {
			if err := c.repo.UpsertCandle(buildCandle(interval, windowStart, durations[interval], bucket)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *PriceCollector) rebuildCandles(symbol string, capturedAt time.Time) error {
	durations := map[string]time.Duration{
		"1m":  time.Minute,
		"5m":  5 * time.Minute,
		"15m": 15 * time.Minute,
		"1h":  time.Hour,
		"1d":  24 * time.Hour,
	}

	for interval, duration := range durations {
		windowStart := capturedAt.Truncate(duration)
		windowEnd := windowStart.Add(duration)
		ticks, err := c.repo.ListTicks(symbol, windowStart, windowEnd)
		if err != nil {
			return err
		}
		if len(ticks) == 0 {
			continue
		}

		quotes := make([]source.PriceQuote, 0, len(ticks))
		for _, tick := range ticks {
			quotes = append(quotes, source.PriceQuote{
				Symbol:     tick.Symbol,
				Price:      tick.Price,
				Currency:   tick.Currency,
				Unit:       tick.Unit,
				CapturedAt: tick.CapturedAt,
			})
		}

		if err := c.repo.UpsertCandle(buildCandle(interval, windowStart, duration, quotes)); err != nil {
			return err
		}
	}

	return nil
}

func buildCandle(interval string, windowStart time.Time, duration time.Duration, quotes []source.PriceQuote) model.GoldPriceCandle {
	openPrice := quotes[0].Price
	closePrice := quotes[len(quotes)-1].Price
	highPrice := quotes[0].Price
	lowPrice := quotes[0].Price
	sum := 0.0

	for _, item := range quotes {
		sum += item.Price
		highPrice = math.Max(highPrice, item.Price)
		lowPrice = math.Min(lowPrice, item.Price)
	}

	return model.GoldPriceCandle{
		Symbol:      quotes[0].Symbol,
		Interval:    interval,
		OpenPrice:   roundPrice(openPrice),
		HighPrice:   roundPrice(highPrice),
		LowPrice:    roundPrice(lowPrice),
		ClosePrice:  roundPrice(closePrice),
		AvgPrice:    roundPrice(sum / float64(len(quotes))),
		SampleCount: len(quotes),
		WindowStart: windowStart,
		WindowEnd:   windowStart.Add(duration),
	}
}

func (c *PriceCollector) validateRealtimeQuote(quote source.PriceQuote) (source.PriceQuote, error) {
	if quote.Price <= 0 {
		return source.PriceQuote{}, errors.New("price must be positive")
	}
	if quote.CapturedAt.After(time.Now().Add(maxFutureSkew)) {
		return source.PriceQuote{}, errors.New("quote captured_at is too far in the future")
	}

	latest, err := c.repo.GetLatestTick()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return source.PriceQuote{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return quote, nil
	}
	if !quote.CapturedAt.After(latest.CapturedAt) {
		return source.PriceQuote{}, errors.New("stale quote ignored")
	}
	if latest.Price > 0 {
		jump := math.Abs(quote.Price-latest.Price) / latest.Price * 100
		if jump > maxRealtimeJumpPercent {
			return source.PriceQuote{}, errors.New("quote filtered as abnormal jump")
		}
	}

	return quote, nil
}

func (c *PriceCollector) failJob(startedAt time.Time, err error, options JobRunOptions) (model.JobRun, error) {
	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "collect-price",
		JobType:    "collector",
		Status:     "failed",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    err.Error(),
	}
	fillJobRunMeta(&run, options, int(finishedAt.Sub(startedAt).Milliseconds()), err)

	saveErr := c.repo.SaveJobRun(model.JobRunRecord{
		JobName:      run.JobName,
		JobType:      run.JobType,
		Status:       run.Status,
		TriggerMode:  run.TriggerMode,
		Attempt:      run.Attempt,
		MaxAttempts:  run.MaxAttempts,
		ScheduledFor: options.ScheduledFor,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		DurationMS:   run.DurationMS,
		Message:      "price collector failed",
		ErrorDetail:  err.Error(),
	})
	if saveErr != nil {
		return run, errors.Join(err, saveErr)
	}

	return run, err
}

func roundPrice(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func sanitizeHistoryQuotes(quotes []source.PriceQuote) []source.PriceQuote {
	if len(quotes) == 0 {
		return nil
	}

	items := append([]source.PriceQuote(nil), quotes...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CapturedAt.Before(items[j].CapturedAt)
	})

	sanitized := make([]source.PriceQuote, 0, len(items))
	var lastAccepted *source.PriceQuote
	for _, quote := range items {
		if quote.Price <= 0 || quote.CapturedAt.IsZero() {
			continue
		}
		if lastAccepted != nil {
			if !quote.CapturedAt.After(lastAccepted.CapturedAt) {
				continue
			}
			if lastAccepted.Price > 0 {
				jump := math.Abs(quote.Price-lastAccepted.Price) / lastAccepted.Price * 100
				if jump > maxHistoryJumpPercent {
					continue
				}
			}
		}

		sanitized = append(sanitized, quote)
		lastAccepted = &sanitized[len(sanitized)-1]
	}

	return sanitized
}
