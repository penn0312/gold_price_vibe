package model

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type RealtimePrice struct {
	Symbol       string  `json:"symbol"`
	Price        float64 `json:"price"`
	ChangeAmount float64 `json:"change_amount"`
	ChangeRate   float64 `json:"change_rate"`
	Currency     string  `json:"currency"`
	Unit         string  `json:"unit"`
	CapturedAt   string  `json:"captured_at"`
}

type Candle struct {
	Time  string  `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type PriceHistory struct {
	Symbol   string   `json:"symbol"`
	Interval string   `json:"interval"`
	Items    []Candle `json:"items"`
}

type NewsArticle struct {
	ID             int64    `json:"id"`
	SourceName     string   `json:"source_name"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Content        string   `json:"content"`
	URL            string   `json:"url"`
	Region         string   `json:"region"`
	Category       string   `json:"category"`
	Sentiment      string   `json:"sentiment"`
	Importance     int      `json:"importance"`
	ImpactScore    float64  `json:"impact_score"`
	RelatedFactors []string `json:"related_factors"`
	PublishedAt    string   `json:"published_at"`
	CapturedAt     string   `json:"captured_at"`
}

type NewsQuery struct {
	Page       int
	PageSize   int
	Category   string
	Region     string
	Importance int
	FactorCode string
}

type NewsList struct {
	Items    []NewsArticle `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}

type FactorPoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
	Score float64 `json:"score"`
}

type FactorLatest struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Value           float64 `json:"value"`
	Unit            string  `json:"unit"`
	Score           float64 `json:"score"`
	ImpactDirection string  `json:"impact_direction"`
	ImpactStrength  float64 `json:"impact_strength"`
	Confidence      float64 `json:"confidence"`
	CapturedAt      string  `json:"captured_at"`
}

type FactorDefinition struct {
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	Description         string  `json:"description"`
	Unit                string  `json:"unit"`
	ValueType           string  `json:"value_type"`
	DefaultWeight       float64 `json:"default_weight"`
	ImpactDirectionRule string  `json:"impact_direction_rule"`
}

type FactorHistory struct {
	Code  string        `json:"code"`
	Range string        `json:"range"`
	Items []FactorPoint `json:"items"`
}

type ReportQuery struct {
	Page      int
	PageSize  int
	StartDate string
	EndDate   string
}

type ReportSummary struct {
	ID            int64    `json:"id"`
	ReportDate    string   `json:"report_date"`
	Title         string   `json:"title"`
	Trend         string   `json:"trend"`
	Confidence    float64  `json:"confidence"`
	Summary       string   `json:"summary"`
	KeyDrivers    []string `json:"key_drivers"`
	RiskPoints    []string `json:"risk_points"`
	AccuracyScore float64  `json:"accuracy_score"`
	GeneratedAt   string   `json:"generated_at"`
}

type ReportPrediction struct {
	TargetDate         string   `json:"target_date"`
	PredictedDirection string   `json:"predicted_direction"`
	PredictedLow       float64  `json:"predicted_low"`
	PredictedHigh      float64  `json:"predicted_high"`
	PredictedClose     float64  `json:"predicted_close"`
	FactorFocus        []string `json:"factor_focus"`
}

type ReportScoreDetail struct {
	ScoredDate       string  `json:"scored_date"`
	DirectionScore   float64 `json:"direction_score"`
	RangeScore       float64 `json:"range_score"`
	FactorHitScore   float64 `json:"factor_hit_score"`
	RiskScore        float64 `json:"risk_score"`
	TotalScore       float64 `json:"total_score"`
	ActualClose      float64 `json:"actual_close"`
	ActualHigh       float64 `json:"actual_high"`
	ActualLow        float64 `json:"actual_low"`
	ScoreExplanation string  `json:"score_explanation"`
}

type ReportDetail struct {
	ReportSummary
	FullContent   string             `json:"full_content"`
	AIProvider    string             `json:"ai_provider"`
	ModelName     string             `json:"model_name"`
	PromptVersion string             `json:"prompt_version"`
	Predictions   []ReportPrediction `json:"predictions"`
	Score         *ReportScoreDetail `json:"score,omitempty"`
}

type ReportList struct {
	Items    []ReportSummary `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

type AccuracyItem struct {
	ReportDate string  `json:"report_date"`
	Score      float64 `json:"score"`
}

type AccuracyCurve struct {
	AvgScore float64        `json:"avg_score"`
	Items    []AccuracyItem `json:"items"`
}

type DashboardOverview struct {
	RealtimePrice RealtimePrice  `json:"realtime_price"`
	LatestReport  ReportSummary  `json:"latest_report"`
	Factors       []FactorLatest `json:"factors"`
	Headlines     []NewsArticle  `json:"headlines"`
}

type JobRun struct {
	ID         int64  `json:"id"`
	JobName    string `json:"job_name"`
	JobType    string `json:"job_type"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Message    string `json:"message"`
}
