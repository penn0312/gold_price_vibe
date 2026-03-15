package service

import (
	"context"
	"errors"
	"math"
	"time"

	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/source"
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

	sourceRecord, err := c.repo.EnsureSource(c.provider.Metadata())
	if err != nil {
		return err
	}

	quotes, err := c.provider.HistoricalTicks(ctx, 1440, time.Minute)
	if err != nil {
		return err
	}

	if err := c.repo.SaveTicks(sourceRecord.ID, quotes); err != nil {
		return err
	}

	return c.seedCandles(quotes)
}

func (c *PriceCollector) CollectNow(ctx context.Context) (model.JobRun, error) {
	startedAt := time.Now()
	sourceRecord, err := c.repo.EnsureSource(c.provider.Metadata())
	if err != nil {
		return c.failJob(startedAt, err)
	}

	quote, err := c.provider.CurrentPrice(ctx)
	if err != nil {
		return c.failJob(startedAt, err)
	}

	if _, err := c.repo.SaveTick(sourceRecord.ID, quote); err != nil {
		return c.failJob(startedAt, err)
	}

	if err := c.rebuildCandles(quote.Symbol, quote.CapturedAt); err != nil {
		return c.failJob(startedAt, err)
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

	err = c.repo.SaveJobRun(model.JobRunRecord{
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

func (c *PriceCollector) failJob(startedAt time.Time, err error) (model.JobRun, error) {
	finishedAt := time.Now()
	run := model.JobRun{
		JobName:    "collect-price",
		JobType:    "collector",
		Status:     "failed",
		StartedAt:  startedAt.Format(time.RFC3339),
		FinishedAt: finishedAt.Format(time.RFC3339),
		Message:    err.Error(),
	}

	saveErr := c.repo.SaveJobRun(model.JobRunRecord{
		JobName:     run.JobName,
		JobType:     run.JobType,
		Status:      run.Status,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
		DurationMS:  int(finishedAt.Sub(startedAt).Milliseconds()),
		Message:     "price collector failed",
		ErrorDetail: err.Error(),
	})
	if saveErr != nil {
		return run, errors.Join(err, saveErr)
	}

	return run, err
}

func roundPrice(value float64) float64 {
	return math.Round(value*1000) / 1000
}
