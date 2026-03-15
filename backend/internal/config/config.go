package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                    string
	DatabasePath            string
	GoldSourceMode          string
	SGEQuoteURL             string
	GoldAPIURL              string
	GoldAPIKey              string
	NewsSourceMode          string
	GoogleNewsRSSBaseURL    string
	GoogleNewsHL            string
	GoogleNewsGL            string
	GoogleNewsCEID          string
	NewsFeedURL             string
	NewsAPIKey              string
	USDToCNYRate            float64
	PriceCollectIntervalSec int
	NewsFetchEnabled        bool
	NewsFetchIntervalSec    int
	FactorUpdateEnabled     bool
	FactorUpdateIntervalSec int
	ReportGenerateEnabled   bool
	ReportGenerateTime      string
	ReportScoreEnabled      bool
	ReportScoreTime         string
	JobRetryLimit           int
	JobRetryBackoffSec      int
	JobTimeoutSec           int
	JobAlertWebhook         string
}

func Load() Config {
	return Config{
		Port:                    getEnv("APP_PORT", "8080"),
		DatabasePath:            getEnv("APP_DB_PATH", "./gold_price.db"),
		GoldSourceMode:          getEnv("GOLD_SOURCE_MODE", "real"),
		SGEQuoteURL:             getEnv("SGE_QUOTE_URL", "https://www.sge.com.cn/h5_sjzx/yshq"),
		GoldAPIURL:              getEnv("GOLD_API_URL", ""),
		GoldAPIKey:              getEnv("GOLD_API_KEY", ""),
		NewsSourceMode:          getEnv("NEWS_SOURCE_MODE", "real"),
		GoogleNewsRSSBaseURL:    getEnv("GOOGLE_NEWS_RSS_BASE_URL", "https://news.google.com/rss/search"),
		GoogleNewsHL:            getEnv("GOOGLE_NEWS_HL", "zh-CN"),
		GoogleNewsGL:            getEnv("GOOGLE_NEWS_GL", "CN"),
		GoogleNewsCEID:          getEnv("GOOGLE_NEWS_CEID", "CN:zh-Hans"),
		NewsFeedURL:             getEnv("NEWS_FEED_URL", ""),
		NewsAPIKey:              getEnv("NEWS_API_KEY", ""),
		USDToCNYRate:            getEnvAsFloat("USD_CNY_RATE", 7.2),
		PriceCollectIntervalSec: getEnvAsInt("PRICE_COLLECT_INTERVAL_SEC", 30),
		NewsFetchEnabled:        getEnvAsBool("NEWS_FETCH_ENABLED", true),
		NewsFetchIntervalSec:    getEnvAsInt("NEWS_FETCH_INTERVAL_SEC", 600),
		FactorUpdateEnabled:     getEnvAsBool("FACTOR_UPDATE_ENABLED", true),
		FactorUpdateIntervalSec: getEnvAsInt("FACTOR_UPDATE_INTERVAL_SEC", 900),
		ReportGenerateEnabled:   getEnvAsBool("REPORT_GENERATE_ENABLED", true),
		ReportGenerateTime:      getEnv("REPORT_GENERATE_TIME", "09:00"),
		ReportScoreEnabled:      getEnvAsBool("REPORT_SCORE_ENABLED", true),
		ReportScoreTime:         getEnv("REPORT_SCORE_TIME", "09:10"),
		JobRetryLimit:           getEnvAsInt("JOB_RETRY_LIMIT", 2),
		JobRetryBackoffSec:      getEnvAsInt("JOB_RETRY_BACKOFF_SEC", 10),
		JobTimeoutSec:           getEnvAsInt("JOB_TIMEOUT_SEC", 20),
		JobAlertWebhook:         getEnv("JOB_ALERT_WEBHOOK", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}

	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}

	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}

	return fallback
}
