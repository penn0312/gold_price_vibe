package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gold_price/backend/internal/config"
)

const (
	DefaultSymbol   = "AU_CNY_G"
	DefaultCurrency = "CNY"
	DefaultUnit     = "g"
	gramsPerTroyOz  = 31.1034768
)

type SourceMeta struct {
	Code     string
	Name     string
	BaseURL  string
	Category string
	Priority int
}

type PriceQuote struct {
	Symbol     string
	Price      float64
	Currency   string
	Unit       string
	FXRate     float64
	CapturedAt time.Time
}

type PriceProvider interface {
	Metadata() SourceMeta
	CurrentPrice(ctx context.Context) (PriceQuote, error)
	HistoricalTicks(ctx context.Context, count int, step time.Duration) ([]PriceQuote, error)
}

func NewPriceProvider(cfg config.Config) PriceProvider {
	providers := make([]PriceProvider, 0, 2)
	switch strings.ToLower(strings.TrimSpace(cfg.GoldSourceMode)) {
	case "real", "sge":
		providers = append(providers, &SGEDelayedPriceProvider{
			client: &http.Client{Timeout: 8 * time.Second},
			url:    cfg.SGEQuoteURL,
		})
		if cfg.GoldAPIURL != "" {
			providers = append(providers, &RemoteHTTPProvider{
				client:       &http.Client{Timeout: 8 * time.Second},
				url:          cfg.GoldAPIURL,
				apiKey:       cfg.GoldAPIKey,
				usdToCNYRate: cfg.USDToCNYRate,
			})
		}
		providers = append(providers, &MockPriceProvider{})
	case "remote":
		providers = append(providers, &RemoteHTTPProvider{
			client:       &http.Client{Timeout: 8 * time.Second},
			url:          cfg.GoldAPIURL,
			apiKey:       cfg.GoldAPIKey,
			usdToCNYRate: cfg.USDToCNYRate,
		})
		providers = append(providers, &MockPriceProvider{})
	default:
		providers = append(providers, &MockPriceProvider{})
	}

	return &SequentialPriceProvider{
		providers:    providers,
		usdToCNYRate: cfg.USDToCNYRate,
	}
}

type SequentialPriceProvider struct {
	providers    []PriceProvider
	usdToCNYRate float64
	activeIndex  atomic.Int64
}

func (p *SequentialPriceProvider) Metadata() SourceMeta {
	if len(p.providers) == 0 {
		return SourceMeta{}
	}

	index := int(p.activeIndex.Load())
	if index < 0 || index >= len(p.providers) {
		index = 0
	}

	return p.providers[index].Metadata()
}

func (p *SequentialPriceProvider) CurrentPrice(ctx context.Context) (PriceQuote, error) {
	var errs []error
	for index, provider := range p.providers {
		quote, err := provider.CurrentPrice(ctx)
		if err != nil {
			errs = append(errs, providerErr(provider.Metadata(), err))
			continue
		}

		normalized, err := normalizeQuote(quote, p.usdToCNYRate)
		if err != nil {
			errs = append(errs, providerErr(provider.Metadata(), err))
			continue
		}

		p.activeIndex.Store(int64(index))
		return normalized, nil
	}

	if len(errs) == 0 {
		return PriceQuote{}, errors.New("no price provider configured")
	}

	return PriceQuote{}, errors.Join(errs...)
}

func (p *SequentialPriceProvider) HistoricalTicks(ctx context.Context, count int, step time.Duration) ([]PriceQuote, error) {
	var errs []error
	for index, provider := range p.providers {
		quotes, err := provider.HistoricalTicks(ctx, count, step)
		if err != nil {
			errs = append(errs, providerErr(provider.Metadata(), err))
			continue
		}

		normalized := make([]PriceQuote, 0, len(quotes))
		failed := false
		for _, quote := range quotes {
			cleaned, cleanErr := normalizeQuote(quote, p.usdToCNYRate)
			if cleanErr != nil {
				errs = append(errs, providerErr(provider.Metadata(), cleanErr))
				failed = true
				break
			}
			normalized = append(normalized, cleaned)
		}
		if failed {
			continue
		}

		p.activeIndex.Store(int64(index))
		return normalized, nil
	}

	if len(errs) == 0 {
		return nil, errors.New("no price provider configured")
	}

	return nil, errors.Join(errs...)
}

type MockPriceProvider struct{}

func (p *MockPriceProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "mock_gold_feed",
		Name:     "Mock Gold Feed",
		BaseURL:  "local://mock",
		Category: "gold",
		Priority: 9,
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
	client       *http.Client
	url          string
	apiKey       string
	usdToCNYRate float64
}

type SGEDelayedPriceProvider struct {
	client *http.Client
	url    string
}

var sgePriceRowPattern = regexp.MustCompile(`(?is)<tr[^>]*>\s*<td[^>]*>\s*(Au99\.99|Au99\.95|Au\(T\+D\))\s*</td>(.*?)</tr>`)
var sgeCellPattern = regexp.MustCompile(`(?is)<td[^>]*>\s*([^<]+?)\s*</td>`)

type remotePriceResponse struct {
	Symbol     string  `json:"symbol"`
	Price      float64 `json:"price"`
	Currency   string  `json:"currency"`
	Unit       string  `json:"unit"`
	FXRateCNY  float64 `json:"fx_rate_cny"`
	CapturedAt string  `json:"captured_at"`
}

func (p *RemoteHTTPProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "remote_gold_feed",
		Name:     "Remote Gold Feed",
		BaseURL:  p.url,
		Category: "gold",
		Priority: 1,
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
		FXRate:     firstPositive(payload.FXRateCNY, p.usdToCNYRate),
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

func (p *SGEDelayedPriceProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "sge_delayed_quote",
		Name:     "Shanghai Gold Exchange Delayed Quote",
		BaseURL:  p.url,
		Category: "gold",
		Priority: 1,
	}
}

func (p *SGEDelayedPriceProvider) CurrentPrice(ctx context.Context) (PriceQuote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return PriceQuote{}, err
	}
	req.Header.Set("User-Agent", "gold-price-bot/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return PriceQuote{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return PriceQuote{}, errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PriceQuote{}, err
	}

	price, err := parseSGEDelayedPrice(body)
	if err != nil {
		return PriceQuote{}, err
	}

	return PriceQuote{
		Symbol:     DefaultSymbol,
		Price:      price,
		Currency:   DefaultCurrency,
		Unit:       DefaultUnit,
		CapturedAt: time.Now(),
	}, nil
}

func (p *SGEDelayedPriceProvider) HistoricalTicks(ctx context.Context, count int, step time.Duration) ([]PriceQuote, error) {
	current, err := p.CurrentPrice(ctx)
	if err != nil {
		return nil, err
	}

	if count <= 0 {
		count = 1
	}

	quotes := make([]PriceQuote, 0, count)
	for i := count - 1; i >= 0; i-- {
		quotes = append(quotes, PriceQuote{
			Symbol:     current.Symbol,
			Price:      current.Price,
			Currency:   current.Currency,
			Unit:       current.Unit,
			CapturedAt: current.CapturedAt.Add(-time.Duration(i) * step),
		})
	}
	return quotes, nil
}

func parseSGEDelayedPrice(body []byte) (float64, error) {
	matches := sgePriceRowPattern.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		return 0, errors.New("sge delayed price row not found")
	}

	for _, match := range matches {
		cells := sgeCellPattern.FindAllStringSubmatch(match[0], -1)
		if len(cells) < 3 {
			continue
		}

		for _, cell := range cells[1:] {
			value := normalizeNumericText(cell[1])
			if value == "" {
				continue
			}
			price, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}
			if price > 50 && price < 10000 {
				return round(price), nil
			}
		}
	}

	return 0, errors.New("sge delayed price value not found")
}

func round(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func normalizeQuote(quote PriceQuote, fallbackFXRate float64) (PriceQuote, error) {
	if quote.CapturedAt.IsZero() {
		quote.CapturedAt = time.Now()
	}
	if quote.Symbol == "" {
		quote.Symbol = DefaultSymbol
	}

	currency := strings.ToUpper(strings.TrimSpace(quote.Currency))
	unit := normalizeUnit(quote.Unit)
	if currency == "" {
		currency = DefaultCurrency
	}
	if unit == "" {
		unit = DefaultUnit
	}
	if quote.Price <= 0 {
		return PriceQuote{}, errors.New("price must be positive")
	}

	price := quote.Price
	switch {
	case currency == "CNY" && unit == "g":
	case currency == "CNY" && unit == "kg":
		price = price / 1000
	case currency == "CNY" && unit == "oz":
		price = price / gramsPerTroyOz
	case currency == "USD" && unit == "g":
		price = price * firstPositive(quote.FXRate, fallbackFXRate)
	case currency == "USD" && unit == "kg":
		price = price * firstPositive(quote.FXRate, fallbackFXRate) / 1000
	case currency == "USD" && unit == "oz":
		price = price * firstPositive(quote.FXRate, fallbackFXRate) / gramsPerTroyOz
	default:
		return PriceQuote{}, fmt.Errorf("unsupported currency/unit: %s/%s", currency, unit)
	}

	if currency == "USD" && firstPositive(quote.FXRate, fallbackFXRate) <= 0 {
		return PriceQuote{}, errors.New("missing usd/cny fx rate")
	}
	if price < 50 || price > 10000 {
		return PriceQuote{}, fmt.Errorf("price out of expected range: %.3f", price)
	}

	return PriceQuote{
		Symbol:     quote.Symbol,
		Price:      round(price),
		Currency:   DefaultCurrency,
		Unit:       DefaultUnit,
		FXRate:     firstPositive(quote.FXRate, fallbackFXRate),
		CapturedAt: quote.CapturedAt,
	}, nil
}

func normalizeUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "g", "gram", "grams":
		return "g"
	case "kg", "kilogram", "kilograms":
		return "kg"
	case "oz", "ozt", "troy_oz", "troy-ounce", "troy ounce":
		return "oz"
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}

	return 0
}

func providerErr(meta SourceMeta, err error) error {
	if meta.Code == "" {
		return err
	}

	return fmt.Errorf("%s: %w", meta.Code, err)
}

func normalizeNumericText(value string) string {
	replacer := strings.NewReplacer(",", "", "&nbsp;", "", "\u00a0", "", " ", "", "\n", "", "\t", "", "\r", "")
	cleaned := replacer.Replace(strings.TrimSpace(value))
	cleaned = strings.Trim(cleaned, "+")
	if cleaned == "-" || cleaned == "" {
		return ""
	}
	return cleaned
}
