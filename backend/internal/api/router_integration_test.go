package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gold_price/backend/internal/config"
	"gold_price/backend/internal/model"
	"gold_price/backend/internal/repository"
	"gold_price/backend/internal/service"
	"gold_price/backend/internal/source"
)

func TestRouterCoreEndpoints(t *testing.T) {
	t.Parallel()

	router := newIntegrationRouter(t)

	t.Run("health", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		assertJSONCode(t, recorder.Body.Bytes(), 0)
	})

	t.Run("dashboard overview", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/overview", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		assertJSONCode(t, recorder.Body.Bytes(), 0)
	})

	t.Run("job definitions", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs/definitions", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}

		var payload struct {
			Code int                   `json:"code"`
			Data []model.JobDefinition `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if payload.Code != 0 {
			t.Fatalf("expected code 0, got %d", payload.Code)
		}
		if len(payload.Data) < 4 {
			t.Fatalf("expected at least 4 job definitions, got %d", len(payload.Data))
		}
	})

	t.Run("invalid price history range", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/prices/history?range=2d", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", recorder.Code)
		}
		assertJSONCode(t, recorder.Body.Bytes(), 4001)
	})

	t.Run("accuracy curve", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/reports/accuracy/curve?range=30d", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
		assertJSONCode(t, recorder.Body.Bytes(), 0)
	})
}

func newIntegrationRouter(t *testing.T) http.Handler {
	t.Helper()

	cfg := config.Config{
		Port:                    "8080",
		DatabasePath:            filepath.Join(t.TempDir(), "integration.db"),
		GoldSourceMode:          "mock",
		NewsSourceMode:          "mock",
		USDToCNYRate:            7.2,
		PriceCollectIntervalSec: 30,
		NewsFetchEnabled:        true,
		NewsFetchIntervalSec:    600,
		FactorUpdateEnabled:     true,
		FactorUpdateIntervalSec: 900,
		ReportGenerateEnabled:   true,
		ReportGenerateTime:      "09:00",
		ReportScoreEnabled:      true,
		ReportScoreTime:         "09:10",
		JobRetryLimit:           1,
		JobRetryBackoffSec:      1,
		JobTimeoutSec:           5,
	}

	db, err := model.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	priceRepo := repository.NewPriceRepository(db)
	newsRepo := repository.NewNewsRepository(db)
	factorRepo := repository.NewFactorRepository(db)
	reportRepo := repository.NewReportRepository(db)
	jobRepo := repository.NewJobRepository(db)

	collector := service.NewPriceCollector(priceRepo, source.NewPriceProvider(cfg))
	newsIngestion := service.NewNewsIngestionService(newsRepo, source.NewNewsProvider(cfg))
	factorService := service.NewFactorService(factorRepo, priceRepo, newsRepo)
	reportService := service.NewReportService(reportRepo, priceRepo, factorRepo, newsRepo)
	jobRunner := service.NewJobRunner(cfg, jobRepo, collector, newsIngestion, factorService, reportService)

	ctx := context.Background()
	if err := collector.BootstrapHistory(ctx); err != nil {
		t.Fatalf("bootstrap history: %v", err)
	}
	if err := newsIngestion.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap news: %v", err)
	}
	if err := factorService.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap factors: %v", err)
	}
	if err := reportService.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap reports: %v", err)
	}
	if err := jobRunner.EnsureDefinitions(); err != nil {
		t.Fatalf("ensure definitions: %v", err)
	}

	appService := service.NewAppService(priceRepo, newsRepo, collector, newsIngestion, factorService, reportService, jobRunner)
	return NewRouter(cfg, appService)
}

func assertJSONCode(t *testing.T, body []byte, expected int) {
	t.Helper()

	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Code != expected {
		t.Fatalf("expected code %d, got %d", expected, payload.Code)
	}
}
