package main

import (
	"context"
	"log"

	"gold_price/backend/internal/api"
	"gold_price/backend/internal/config"
	"gold_price/backend/internal/cron"
	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/service"
	"gold_price/backend/internal/source"
)

func main() {
	cfg := config.Load()
	db, err := model.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}

	priceRepo := repository.NewPriceRepository(db)
	newsRepo := repository.NewNewsRepository(db)
	factorRepo := repository.NewFactorRepository(db)
	reportRepo := repository.NewReportRepository(db)
	jobRepo := repository.NewJobRepository(db)
	provider := source.NewPriceProvider(cfg)
	newsProvider := source.NewNewsProvider(cfg)
	collector := service.NewPriceCollector(priceRepo, provider)
	newsIngestion := service.NewNewsIngestionService(newsRepo, newsProvider)
	factorService := service.NewFactorService(factorRepo, priceRepo, newsRepo)
	reportService := service.NewReportService(reportRepo, priceRepo, factorRepo, newsRepo)
	jobRunner := service.NewJobRunner(cfg, jobRepo, collector, newsIngestion, factorService, reportService)
	if err := collector.BootstrapHistory(context.Background()); err != nil {
		log.Printf("bootstrap history failed: %v", err)
	}
	if err := newsIngestion.Bootstrap(context.Background()); err != nil {
		log.Printf("bootstrap news failed: %v", err)
	}
	if err := factorService.Bootstrap(context.Background()); err != nil {
		log.Printf("bootstrap factors failed: %v", err)
	}
	if err := reportService.Bootstrap(context.Background()); err != nil {
		log.Printf("bootstrap reports failed: %v", err)
	}
	if err := jobRunner.EnsureDefinitions(); err != nil {
		log.Printf("bootstrap jobs failed: %v", err)
	}
	cron.StartPriceCollector(context.Background(), cfg, collector)
	service.NewJobScheduler(cfg, jobRepo, jobRunner).Start(context.Background())

	svc := service.NewAppService(priceRepo, newsRepo, collector, newsIngestion, factorService, reportService, jobRunner)
	router := api.NewRouter(cfg, svc)

	log.Printf("sqlite ready at %s", cfg.DatabasePath)
	log.Printf("gold_price backend listening on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}

}
