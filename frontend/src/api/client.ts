import type {
  AccuracyCurve,
  ApiResponse,
  DashboardOverview,
  PriceHistory
} from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080/api/v1'

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`)
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`)
  }

  const payload = (await response.json()) as ApiResponse<T>
  if (payload.code !== 0) {
    throw new Error(payload.message)
  }

  return payload.data
}

export function getDashboardOverview() {
  return request<DashboardOverview>('/dashboard/overview')
}

export function getPriceHistory(range = '1d', interval = '1m') {
  return request<PriceHistory>(`/prices/history?range=${range}&interval=${interval}`)
}

export function getAccuracyCurve(range = '30d') {
  return request<AccuracyCurve>(`/reports/accuracy/curve?range=${range}`)
}

