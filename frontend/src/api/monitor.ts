import { apiClient } from './client'

export const monitorApi = {
  getTokenTrend: (days = 30) =>
    apiClient.get('/monitor/token-trend', { params: { days } }).then((r) => r.data),

  getActivityHeatmap: () =>
    apiClient.get('/monitor/activity-heatmap').then((r) => r.data),

  getModelDistribution: (days = 30) =>
    apiClient.get('/monitor/model-distribution', { params: { days } }).then((r) => r.data),

  getLatencyTrend: (days = 30) =>
    apiClient.get('/monitor/latency-trend', { params: { days } }).then((r) => r.data),

  getAgentRanking: () =>
    apiClient.get('/monitor/agent-ranking').then((r) => r.data),
}
