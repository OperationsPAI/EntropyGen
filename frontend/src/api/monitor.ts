import { apiClient } from './client'

export interface TokenUsageSummary {
  agent_id: string
  date: string
  tokens_in: number
  tokens_out: number
  request_count: number
  model: string
}

export interface AgentActivityPoint {
  agent_id: string
  hour: number
  count: number
}

export interface ModelDistributionItem {
  model: string
  count: number
}

export interface LatencyPoint {
  date: string
  avg_ms: number
  p95_ms: number
}

export interface AgentRankingItem {
  agent_id: string
  total_usage: number
}

export const monitorApi = {
  getTokenTrend: (days = 30) =>
    apiClient.get('/monitor/token-trend', { params: { days } }).then((r) =>
      (r.data?.data ?? []) as TokenUsageSummary[],
    ),

  getActivityHeatmap: (days = 7) =>
    apiClient.get('/monitor/activity-heatmap', { params: { days } }).then((r) =>
      (r.data?.data ?? []) as AgentActivityPoint[],
    ),

  getModelDistribution: (days = 30) =>
    apiClient.get('/monitor/model-distribution', { params: { days } }).then((r) =>
      (r.data?.data ?? []) as ModelDistributionItem[],
    ),

  getLatencyTrend: (days = 30) =>
    apiClient.get('/monitor/latency-trend', { params: { days } }).then((r) =>
      (r.data?.data ?? []) as LatencyPoint[],
    ),

  getAgentRanking: () =>
    apiClient.get('/monitor/agent-ranking').then((r) =>
      (r.data?.data ?? []) as AgentRankingItem[],
    ),
}
