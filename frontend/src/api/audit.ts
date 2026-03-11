import { apiClient } from './client'
import type { AuditTrace, TraceFilter, TraceListResponse, TokenUsageSummary } from '../types/trace'

export const auditApi = {
  getTraces: (filter: TraceFilter = {}) =>
    apiClient.get<TraceListResponse>('/audit/traces', { params: filter }).then((r) => r.data),

  getTrace: (traceId: string) =>
    apiClient.get<AuditTrace>(`/audit/traces/${traceId}`).then((r) => r.data),

  getTokenUsage: (agentId?: string, days = 30) =>
    apiClient.get<TokenUsageSummary[]>('/audit/token-usage', { params: { agent_id: agentId, days } }).then((r) => r.data),

  getAgentActivity: (agentId: string, days = 7) =>
    apiClient.get<{ hour: number; count: number }[]>(`/audit/activity/${agentId}`, { params: { days } }).then((r) => r.data),

  exportTraces: (filter: TraceFilter) => {
    const token = localStorage.getItem('jwt_token')
    const params = new URLSearchParams(filter as Record<string, string>)
    if (token) params.set('token', token)
    window.open(`/api/audit/export?${params.toString()}`, '_blank')
  },
}
