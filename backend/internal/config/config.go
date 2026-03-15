package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                    string
	DatabasePath            string
	GoldSourceMode          string
	GoldAPIURL              string
	GoldAPIKey              string
	PriceCollectIntervalSec int
}

func Load() Config {
	return Config{
		Port:                    getEnv("APP_PORT", "8080"),
		DatabasePath:            getEnv("APP_DB_PATH", "./gold_price.db"),
		GoldSourceMode:          getEnv("GOLD_SOURCE_MODE", "mock"),
		GoldAPIURL:              getEnv("GOLD_API_URL", ""),
		GoldAPIKey:              getEnv("GOLD_API_KEY", ""),
		PriceCollectIntervalSec: getEnvAsInt("PRICE_COLLECT_INTERVAL_SEC", 30),
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
