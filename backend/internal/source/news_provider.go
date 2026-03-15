package source

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"gold_price/backend/internal/config"
)

type NewsItem struct {
	Title       string
	Content     string
	URL         string
	SourceName  string
	PublishedAt time.Time
	CapturedAt  time.Time
}

type NewsProvider interface {
	Metadata() SourceMeta
	Fetch(ctx context.Context) ([]NewsItem, error)
}

func NewNewsProvider(cfg config.Config) NewsProvider {
	providers := make([]NewsProvider, 0, 2)
	switch strings.ToLower(strings.TrimSpace(cfg.NewsSourceMode)) {
	case "real", "google":
		providers = append(providers, &GoogleNewsRSSProvider{
			client:     &http.Client{Timeout: 8 * time.Second},
			baseURL:    cfg.GoogleNewsRSSBaseURL,
			hl:         cfg.GoogleNewsHL,
			gl:         cfg.GoogleNewsGL,
			ceid:       cfg.GoogleNewsCEID,
			searches:   defaultGoogleNewsSearches(),
			sourceName: "Google News RSS",
		})
		if cfg.NewsFeedURL != "" {
			providers = append(providers, &RemoteNewsProvider{
				client: &http.Client{Timeout: 8 * time.Second},
				url:    cfg.NewsFeedURL,
				apiKey: cfg.NewsAPIKey,
			})
		}
		providers = append(providers, &MockNewsProvider{})
	case "remote":
		providers = append(providers, &RemoteNewsProvider{
			client: &http.Client{Timeout: 8 * time.Second},
			url:    cfg.NewsFeedURL,
			apiKey: cfg.NewsAPIKey,
		})
		providers = append(providers, &MockNewsProvider{})
	default:
		providers = append(providers, &MockNewsProvider{})
	}

	return &SequentialNewsProvider{providers: providers}
}

type SequentialNewsProvider struct {
	providers   []NewsProvider
	activeIndex atomic.Int64
}

func (p *SequentialNewsProvider) Metadata() SourceMeta {
	if len(p.providers) == 0 {
		return SourceMeta{}
	}

	index := int(p.activeIndex.Load())
	if index < 0 || index >= len(p.providers) {
		index = 0
	}

	return p.providers[index].Metadata()
}

func (p *SequentialNewsProvider) Fetch(ctx context.Context) ([]NewsItem, error) {
	var errs []error
	for index, provider := range p.providers {
		items, err := provider.Fetch(ctx)
		if err != nil {
			errs = append(errs, providerErr(provider.Metadata(), err))
			continue
		}
		if len(items) == 0 {
			errs = append(errs, providerErr(provider.Metadata(), errors.New("no news items returned")))
			continue
		}

		cleaned := make([]NewsItem, 0, len(items))
		for _, item := range items {
			if strings.TrimSpace(item.Title) == "" {
				continue
			}
			if item.CapturedAt.IsZero() {
				item.CapturedAt = time.Now()
			}
			if item.PublishedAt.IsZero() {
				item.PublishedAt = item.CapturedAt
			}
			if item.SourceName == "" {
				item.SourceName = provider.Metadata().Name
			}
			cleaned = append(cleaned, item)
		}
		if len(cleaned) == 0 {
			errs = append(errs, providerErr(provider.Metadata(), errors.New("all news items invalid")))
			continue
		}

		p.activeIndex.Store(int64(index))
		return cleaned, nil
	}

	if len(errs) == 0 {
		return nil, errors.New("no news provider configured")
	}

	return nil, errors.Join(errs...)
}

type MockNewsProvider struct{}

func (p *MockNewsProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "mock_news_feed",
		Name:     "Mock News Feed",
		BaseURL:  "local://mock-news",
		Category: "news",
		Priority: 9,
	}
}

func (p *MockNewsProvider) Fetch(_ context.Context) ([]NewsItem, error) {
	now := time.Now()
	return []NewsItem{
		{
			Title:       "美元指数回落，黄金短线获得支撑",
			Content:     "美元指数回调压低持有黄金的机会成本，市场对黄金短线配置意愿有所回升。",
			URL:         "https://example.com/news/usd-gold",
			SourceName:  "Mock Macro Desk",
			PublishedAt: now.Add(-35 * time.Minute),
			CapturedAt:  now,
		},
		{
			Title:       "中东局势升温，避险情绪推动贵金属关注度上升",
			Content:     "地缘政治冲突升级抬升避险需求，黄金与其他避险资产成交活跃度明显增加。",
			URL:         "https://example.com/news/geopolitics",
			SourceName:  "Mock Global Wire",
			PublishedAt: now.Add(-2 * time.Hour),
			CapturedAt:  now,
		},
		{
			Title:       "原油价格反弹，通胀预期边际回升",
			Content:     "国际油价走高可能抬升后续通胀预期，强化黄金的抗通胀配置逻辑。",
			URL:         "https://example.com/news/oil-inflation",
			SourceName:  "Mock Market Journal",
			PublishedAt: now.Add(-4 * time.Hour),
			CapturedAt:  now,
		},
		{
			Title:       "亚洲实物金需求回暖，金店终端成交改善",
			Content:     "亚洲市场节后珠宝和投资金条需求回暖，对人民币计价金价形成阶段性支撑。",
			URL:         "https://example.com/news/physical-demand",
			SourceName:  "Mock China Metals",
			PublishedAt: now.Add(-7 * time.Hour),
			CapturedAt:  now,
		},
	}, nil
}

type RemoteNewsProvider struct {
	client *http.Client
	url    string
	apiKey string
}

type GoogleNewsRSSProvider struct {
	client     *http.Client
	baseURL    string
	hl         string
	gl         string
	ceid       string
	searches   []string
	sourceName string
}

type googleNewsRSS struct {
	Channel googleNewsChannel `xml:"channel"`
}

type googleNewsChannel struct {
	Items []googleNewsItem `xml:"item"`
}

type googleNewsItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type remoteNewsPayload struct {
	Items []remoteNewsItem `json:"items"`
}

type remoteNewsItem struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	SourceName  string `json:"source_name"`
	PublishedAt string `json:"published_at"`
	CapturedAt  string `json:"captured_at"`
}

func (p *RemoteNewsProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "remote_news_feed",
		Name:     "Remote News Feed",
		BaseURL:  p.url,
		Category: "news",
		Priority: 1,
	}
}

func (p *RemoteNewsProvider) Fetch(ctx context.Context) ([]NewsItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(resp.Status)
	}

	var payload remoteNewsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	items := make([]NewsItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		publishedAt := parseOptionalTime(item.PublishedAt)
		capturedAt := parseOptionalTime(item.CapturedAt)
		if capturedAt.IsZero() {
			capturedAt = time.Now()
		}
		if publishedAt.IsZero() {
			publishedAt = capturedAt
		}

		items = append(items, NewsItem{
			Title:       item.Title,
			Content:     item.Content,
			URL:         item.URL,
			SourceName:  item.SourceName,
			PublishedAt: publishedAt,
			CapturedAt:  capturedAt,
		})
	}

	return items, nil
}

func parseOptionalTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

func (p *GoogleNewsRSSProvider) Metadata() SourceMeta {
	return SourceMeta{
		Code:     "google_news_rss",
		Name:     p.sourceName,
		BaseURL:  p.baseURL,
		Category: "news",
		Priority: 1,
	}
}

func (p *GoogleNewsRSSProvider) Fetch(ctx context.Context) ([]NewsItem, error) {
	items := make([]NewsItem, 0, len(p.searches)*5)
	for _, search := range p.searches {
		feedItems, err := p.fetchFeed(ctx, search)
		if err != nil {
			return nil, err
		}
		items = append(items, feedItems...)
	}

	return items, nil
}

func (p *GoogleNewsRSSProvider) fetchFeed(ctx context.Context, search string) ([]NewsItem, error) {
	if strings.TrimSpace(p.baseURL) == "" {
		return nil, errors.New("google news rss base url is empty")
	}

	query := url.Values{}
	query.Set("q", search)
	query.Set("hl", p.hl)
	query.Set("gl", p.gl)
	query.Set("ceid", p.ceid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gold-price-bot/0.1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload googleNewsRSS
	if err := xml.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	items := make([]NewsItem, 0, len(payload.Channel.Items))
	now := time.Now()
	for _, item := range payload.Channel.Items {
		title, sourceName := splitGoogleNewsTitle(item.Title)
		if title == "" {
			continue
		}
		publishedAt := parseGoogleNewsTime(item.PubDate)
		if publishedAt.IsZero() {
			publishedAt = now
		}

		description := strings.TrimSpace(item.Description)
		if description == "" {
			description = title
		}

		items = append(items, NewsItem{
			Title:       title,
			Content:     description,
			URL:         strings.TrimSpace(item.Link),
			SourceName:  firstNonEmpty(sourceName, p.sourceName),
			PublishedAt: publishedAt,
			CapturedAt:  now,
		})
	}

	return items, nil
}

func defaultGoogleNewsSearches() []string {
	return []string{
		`黄金 OR 金价 OR 上海黄金交易所 when:1d`,
		`美元指数 OR 美联储 OR 通胀 黄金 when:1d`,
		`gold price OR Federal Reserve OR dollar index when:1d`,
		`geopolitics OR central bank gold OR oil gold when:1d`,
	}
}

func splitGoogleNewsTitle(value string) (string, string) {
	parts := strings.Split(strings.TrimSpace(value), " - ")
	if len(parts) < 2 {
		return strings.TrimSpace(value), ""
	}
	sourceName := strings.TrimSpace(parts[len(parts)-1])
	title := strings.TrimSpace(strings.Join(parts[:len(parts)-1], " - "))
	return title, sourceName
}

func parseGoogleNewsTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC1123Z, value)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse(time.RFC1123, value)
	if err == nil {
		return parsed
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
