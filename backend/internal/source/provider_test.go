package source

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type stubProvider struct {
	meta       SourceMeta
	current    PriceQuote
	historical []PriceQuote
	err        error
}

func (p stubProvider) Metadata() SourceMeta {
	return p.meta
}

func (p stubProvider) CurrentPrice(context.Context) (PriceQuote, error) {
	if p.err != nil {
		return PriceQuote{}, p.err
	}

	return p.current, nil
}

func (p stubProvider) HistoricalTicks(context.Context, int, time.Duration) ([]PriceQuote, error) {
	if p.err != nil {
		return nil, p.err
	}

	return p.historical, nil
}

func TestSequentialPriceProviderFallsBackAndNormalizes(t *testing.T) {
	t.Parallel()

	provider := &SequentialPriceProvider{
		providers: []PriceProvider{
			stubProvider{meta: SourceMeta{Code: "primary_remote"}},
			stubProvider{
				meta: SourceMeta{Code: "backup_mock"},
				current: PriceQuote{
					Symbol:     DefaultSymbol,
					Price:      2300,
					Currency:   "USD",
					Unit:       "oz",
					CapturedAt: time.Now(),
				},
			},
		},
		usdToCNYRate: 7.2,
	}
	provider.providers[0] = stubProvider{
		meta: SourceMeta{Code: "primary_remote"},
		err:  errors.New("upstream unavailable"),
	}

	quote, err := provider.CurrentPrice(context.Background())
	if err != nil {
		t.Fatalf("current price: %v", err)
	}

	expected := 2300 * 7.2 / gramsPerTroyOz
	if math.Abs(quote.Price-expected) > 0.001 {
		t.Fatalf("expected normalized price %.3f, got %.3f", expected, quote.Price)
	}
	if quote.Currency != DefaultCurrency || quote.Unit != DefaultUnit {
		t.Fatalf("expected %s/%s, got %s/%s", DefaultCurrency, DefaultUnit, quote.Currency, quote.Unit)
	}
	if provider.Metadata().Code != "backup_mock" {
		t.Fatalf("expected active provider to switch to backup")
	}
}

func TestNormalizeQuoteConvertsCNYPerKgToPerGram(t *testing.T) {
	t.Parallel()

	quote, err := normalizeQuote(PriceQuote{
		Symbol:     DefaultSymbol,
		Price:      560000,
		Currency:   "CNY",
		Unit:       "kg",
		CapturedAt: time.Now(),
	}, 0)
	if err != nil {
		t.Fatalf("normalize quote: %v", err)
	}

	if quote.Price != 560 {
		t.Fatalf("expected 560 CNY/g, got %.3f", quote.Price)
	}
}

func TestParseSGEDelayedPrice(t *testing.T) {
	t.Parallel()

	body := []byte(`
		<table>
			<tr>
				<td>Au99.99</td>
				<td>688.50</td>
				<td>689.00</td>
				<td>687.20</td>
			</tr>
		</table>
	`)

	price, err := parseSGEDelayedPrice(body)
	if err != nil {
		t.Fatalf("parse sge price: %v", err)
	}
	if price != 688.5 {
		t.Fatalf("expected 688.500, got %.3f", price)
	}
}

func TestSplitGoogleNewsTitle(t *testing.T) {
	t.Parallel()

	title, sourceName := splitGoogleNewsTitle("金价上涨，市场关注美联储路径 - 财经日报")
	if title != "金价上涨，市场关注美联储路径" {
		t.Fatalf("unexpected title %q", title)
	}
	if sourceName != "财经日报" {
		t.Fatalf("unexpected source name %q", sourceName)
	}
}
