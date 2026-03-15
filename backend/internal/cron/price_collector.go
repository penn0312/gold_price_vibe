package cron

import (
	"context"
	"log"
	"time"

	"gold_price/backend/internal/config"
	"gold_price/backend/internal/service"
)

func StartPriceCollector(ctx context.Context, cfg config.Config, collector *service.PriceCollector) {
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.PriceCollectIntervalSec) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := collector.CollectNow(ctx); err != nil {
					log.Printf("price collector failed: %v", err)
				}
			}
		}
	}()
}
