export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

export interface RealtimePrice {
  symbol: string
  price: number
  change_amount: number
  change_rate: number
  currency: string
  unit: string
  captured_at: string
}

export interface Candle {
  time: string
  open: number
  high: number
  low: number
  close: number
}

export interface PriceHistory {
  symbol: string
  interval: string
  items: Candle[]
}

export interface FactorLatest {
  code: string
  name: string
  value: number
  unit: string
  score: number
  impact_direction: 'bullish' | 'bearish' | 'neutral'
  impact_strength: number
  confidence: number
  captured_at: string
}

export interface NewsArticle {
  id: number
  title: string
  summary: string
  url: string
  region: string
  category: string
  sentiment: 'positive' | 'negative' | 'neutral'
  importance: number
  impact_score: number
  related_factors: string[]
  published_at: string
}

export interface ReportSummary {
  id: number
  report_date: string
  title: string
  trend: string
  confidence: number
  summary: string
  key_drivers: string[]
  risk_points: string[]
  accuracy_score: number
  generated_at: string
}

export interface AccuracyItem {
  report_date: string
  score: number
}

export interface AccuracyCurve {
  avg_score: number
  items: AccuracyItem[]
}

export interface DashboardOverview {
  realtime_price: RealtimePrice
  latest_report: ReportSummary
  factors: FactorLatest[]
  headlines: NewsArticle[]
}

