package source

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"

	"gold_price/backend/internal/config"
)

const (
	DefaultSymbol   = "AU_CNY_G"
	DefaultCurrency = "CNY"
	DefaultUnit     = "g"
)

type SourceMeta struct {
	Code     string
	Name     string
	BaseURL  string
	Category string
}

type PriceQuote struct {
	Symbol     string
	Price      float64
	Currency   string
	Unit       string
	CapturedAt time.Time
}

type PriceProvider interface {
	Metadata() SourceMeta
	CurrentPrice(ctx context.Context) (PriceQuote, error)
	HistoricalTicks(ctx context.Context, count int, step time.Duration) ([]PriceQuote, error)
}

func NewPriceProvider(cfg config.Config) PriceProvider {
	if cfg.GoldSourceMode == "remote" && cfg.GoldAPIURL != "" {
		return &RemoteHTTPProvider{
			client: &http.Client{Timeout: 8 * time.Second},
			url:    cfg.GoldAPIURL,
			apiKey: cfg.GoldAPIKey,
		}
	}

	return &MockPriceProvider{}
}

type MockPriceProvider struct{}

func (p *MockPriceProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "mock_gold_feed",
		Name:     "Mock Gold Feed",
		BaseURL:  "local://mock",
		Category: "gold",
	}
}

func (p *MockPriceProvider) CurrentPrice(_ context.Context) (PriceQuote, error) {
	now := time.Now()
	seconds := float64(now.Unix() % 3600)
	price := 562.2 + math.Sin(seconds/240.0)*3.8 + math.Cos(seconds/90.0)*0.6

	return PriceQuote{
		Symbol:     DefaultSymbol,
		Price:      round(price),
		Currency:   DefaultCurrency,
		Unit:       DefaultUnit,
		CapturedAt: now,
	}, nil
}

func (p *MockPriceProvider) HistoricalTicks(_ context.Context, count int, step time.Duration) ([]PriceQuote, error) {
	now := time.Now()
	quotes := make([]PriceQuote, 0, count)
	base := 558.0

	for i := count - 1; i >= 0; i-- {
		pointTime := now.Add(-time.Duration(i) * step)
		angle := float64(i) / 4.0
		price := base + math.Sin(angle)*4.2 + math.Cos(angle/2.5)*1.3 + rand.Float64()*0.15

		quotes = append(quotes, PriceQuote{
			Symbol:     DefaultSymbol,
			Price:      round(price),
			Currency:   DefaultCurrency,
			Unit:       DefaultUnit,
			CapturedAt: pointTime,
		})
	}

	return quotes, nil
}

type RemoteHTTPProvider struct {
	client *http.Client
	url    string
	apiKey string
}

type remotePriceResponse struct {
	Symbol     string  `json:"symbol"`
	Price      float64 `json:"price"`
	Currency   string  `json:"currency"`
	Unit       string  `json:"unit"`
	CapturedAt string  `json:"captured_at"`
}

func (p *RemoteHTTPProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "remote_gold_feed",
		Name:     "Remote Gold Feed",
		BaseURL:  p.url,
		Category: "gold",
	}
}

func (p *RemoteHTTPProvider) CurrentPrice(ctx context.Context) (PriceQuote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return PriceQuote{}, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return PriceQuote{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return PriceQuote{}, errors.New(resp.Status)
	}

	var payload remotePriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return PriceQuote{}, err
	}

	capturedAt := time.Now()
	if payload.CapturedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, payload.CapturedAt); err == nil {
			capturedAt = parsed
		}
	}

	if payload.Symbol == "" {
		payload.Symbol = DefaultSymbol
	}
	if payload.Currency == "" {
		payload.Currency = DefaultCurrency
	}
	if payload.Unit == "" {
		payload.Unit = DefaultUnit
	}

	return PriceQuote{
		Symbol:     payload.Symbol,
		Price:      round(payload.Price),
		Currency:   payload.Currency,
		Unit:       payload.Unit,
		CapturedAt: capturedAt,
	}, nil
}

func (p *RemoteHTTPProvider) HistoricalTicks(ctx context.Context, count int, step time.Duration) ([]PriceQuote, error) {
	current, err := p.CurrentPrice(ctx)
	if err != nil {
		return nil, err
	}

	quotes := make([]PriceQuote, 0, count)
	for i := count - 1; i >= 0; i-- {
		drift := math.Sin(float64(i)/3.3)*2.4 + math.Cos(float64(i)/4.7)*0.8
		quotes = append(quotes, PriceQuote{
			Symbol:     current.Symbol,
			Price:      round(current.Price - drift),
			Currency:   current.Currency,
			Unit:       current.Unit,
			CapturedAt: current.CapturedAt.Add(-time.Duration(i) * step),
		})
	}

	return quotes, nil
}

func round(value float64) float64 {
	return math.Round(value*1000) / 1000
}
