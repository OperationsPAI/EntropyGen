import {
  getAuditStatsTokenUsage,
  getAuditStatsAgentActivity,
  getMonitorModelDistribution,
  getMonitorLatencyTrend,
  getMonitorAgentRanking,
} from './generated/sdk.gen'

/* eslint-disable @typescript-eslint/no-explicit-any */

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
    getAuditStatsTokenUsage({ query: { days } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as TokenUsageSummary[]
    }),

  getActivityHeatmap: (days = 7) =>
    getAuditStatsAgentActivity({ query: { days } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as AgentActivityPoint[]
    }),

  getModelDistribution: (days = 30) =>
    getMonitorModelDistribution({ query: { days } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as ModelDistributionItem[]
    }),

  getLatencyTrend: (days = 30) =>
    getMonitorLatencyTrend({ query: { days } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as LatencyPoint[]
    }),

  getAgentRanking: () =>
    getMonitorAgentRanking().then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as AgentRankingItem[]
    }),
}
