package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"gold_price/backend/internal/config"
	"gold_price/backend/internal/model"
	"gold_price/backend/internal/service"
)

type Handler struct {
	service service.MarketService
}

func NewRouter(cfg config.Config, svc service.MarketService) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	handler := Handler{service: svc}

	api := router.Group("/api/v1")
	{
		api.GET("/health", handler.Health)
		api.GET("/dashboard/overview", handler.GetDashboardOverview)
		api.GET("/prices/realtime", handler.GetRealtimePrice)
		api.GET("/prices/history", handler.GetPriceHistory)
		api.GET("/prices/stream", handler.GetPriceStream)

		api.GET("/news", handler.GetNews)
		api.GET("/news/:id", handler.GetNewsDetail)

		api.GET("/factors/latest", handler.GetLatestFactors)
		api.GET("/factors/history", handler.GetFactorHistory)
		api.GET("/factors/definitions", handler.GetFactorDefinitions)

		api.GET("/reports/latest", handler.GetLatestReport)
		api.GET("/reports", handler.GetReports)
		api.GET("/reports/:id", handler.GetReportDetail)
		api.GET("/reports/accuracy/curve", handler.GetAccuracyCurve)

		admin := api.Group("/admin/jobs")
		{
			admin.POST("/collect-price", handler.TriggerCollectPrice)
			admin.POST("/fetch-news", handler.TriggerFetchNews)
			admin.POST("/update-factors", handler.TriggerUpdateFactors)
			admin.POST("/generate-report", handler.TriggerGenerateReport)
			admin.POST("/score-report", handler.TriggerScoreReport)
			admin.GET("/runs", handler.GetJobRuns)
		}
	}

	_ = cfg
	return router
}

func (h Handler) Health(c *gin.Context) {
	success(c, gin.H{
		"status":      "up",
		"server_time": nowRFC3339(),
	})
}

func (h Handler) GetDashboardOverview(c *gin.Context) {
	success(c, h.service.GetDashboardOverview())
}

func (h Handler) GetRealtimePrice(c *gin.Context) {
	success(c, h.service.GetRealtimePrice())
}

func (h Handler) GetPriceHistory(c *gin.Context) {
	rangeValue := c.DefaultQuery("range", "1d")
	interval := c.Query("interval")
	if !allowed(rangeValue, "1d", "7d", "30d", "90d", "1y") {
		errorResponse(c, http.StatusBadRequest, 4001, "invalid range")
		return
	}

	success(c, h.service.GetPriceHistory(rangeValue, interval))
}

func (h Handler) GetPriceStream(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, 5001, "streaming is not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	lastCapturedAt := ""
	sendPrice := func() {
		price := h.service.GetRealtimePrice()
		if price.CapturedAt == "" || price.CapturedAt == lastCapturedAt {
			return
		}

		lastCapturedAt = price.CapturedAt
		c.SSEvent("price_tick", price)
		flusher.Flush()
	}

	c.SSEvent("price_status", gin.H{
		"status":      "connected",
		"server_time": nowRFC3339(),
	})
	flusher.Flush()
	sendPrice()

	priceTicker := time.NewTicker(3 * time.Second)
	statusTicker := time.NewTicker(15 * time.Second)
	defer priceTicker.Stop()
	defer statusTicker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-priceTicker.C:
			sendPrice()
		case <-statusTicker.C:
			c.SSEvent("price_status", gin.H{
				"status":      "alive",
				"server_time": nowRFC3339(),
			})
			flusher.Flush()
		}
	}
}

func (h Handler) GetNews(c *gin.Context) {
	query := model.NewsQuery{
		Page:       parseQueryInt(c.Query("page"), 1),
		PageSize:   parseQueryInt(c.Query("page_size"), 10),
		Category:   c.Query("category"),
		Region:     c.Query("region"),
		Importance: parseQueryInt(c.Query("importance"), 0),
		FactorCode: c.Query("factor_code"),
	}
	success(c, h.service.ListNews(query))
}

func (h Handler) GetNewsDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, 4001, "invalid id")
		return
	}

	item, ok := h.service.GetNewsDetail(id)
	if !ok {
		errorResponse(c, http.StatusNotFound, 4004, "news not found")
		return
	}

	success(c, item)
}

func (h Handler) GetLatestFactors(c *gin.Context) {
	success(c, h.service.GetLatestFactors())
}

func (h Handler) GetFactorHistory(c *gin.Context) {
	code := c.Query("code")
	rangeValue := c.DefaultQuery("range", "30d")
	if code == "" {
		errorResponse(c, http.StatusBadRequest, 4001, "code is required")
		return
	}
	if !allowed(rangeValue, "7d", "30d", "90d", "1y") {
		errorResponse(c, http.StatusBadRequest, 4001, "invalid range")
		return
	}

	success(c, h.service.GetFactorHistory(code, rangeValue))
}

func (h Handler) GetFactorDefinitions(c *gin.Context) {
	success(c, h.service.GetFactorDefinitions())
}

func (h Handler) GetLatestReport(c *gin.Context) {
	success(c, h.service.GetLatestReport())
}

func (h Handler) GetReports(c *gin.Context) {
	success(c, h.service.GetReports())
}

func (h Handler) GetReportDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, 4001, "invalid id")
		return
	}

	item, ok := h.service.GetReportDetail(id)
	if !ok {
		errorResponse(c, http.StatusNotFound, 4004, "report not found")
		return
	}

	success(c, item)
}

func (h Handler) GetAccuracyCurve(c *gin.Context) {
	rangeValue := c.DefaultQuery("range", "30d")
	if !allowed(rangeValue, "30d", "90d", "180d", "1y") {
		errorResponse(c, http.StatusBadRequest, 4001, "invalid range")
		return
	}

	success(c, h.service.GetAccuracyCurve(rangeValue))
}

func (h Handler) TriggerCollectPrice(c *gin.Context) {
	success(c, h.service.TriggerJob("collect-price"))
}

func (h Handler) TriggerFetchNews(c *gin.Context) {
	success(c, h.service.TriggerJob("fetch-news"))
}

func (h Handler) TriggerUpdateFactors(c *gin.Context) {
	success(c, h.service.TriggerJob("update-factors"))
}

func (h Handler) TriggerGenerateReport(c *gin.Context) {
	success(c, h.service.TriggerJob("generate-report"))
}

func (h Handler) TriggerScoreReport(c *gin.Context) {
	success(c, h.service.TriggerJob("score-report"))
}

func (h Handler) GetJobRuns(c *gin.Context) {
	success(c, h.service.GetJobRuns())
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, model.APIResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func errorResponse(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, model.APIResponse{
		Code:    code,
		Message: message,
		Data:    gin.H{},
	})
}

func allowed(target string, items ...string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}

	return false
}

func parseQueryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
