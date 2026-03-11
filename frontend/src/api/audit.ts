import { apiClient } from './client'
import type { AuditTrace, TraceFilter, TraceListResponse, TokenUsageSummary, RequestType } from '../types/trace'

export const auditApi = {
  getTraces: (filter: TraceFilter = {}) =>
    apiClient.get('/audit/traces', { params: filter }).then((r) => {
      const body = r.data
      return {
        items: (body.data ?? []).map(mapTrace),
        total: body.meta?.count ?? 0,
        page: filter.page ?? 1,
        limit: filter.limit ?? 20,
      } as TraceListResponse
    }),

  getTrace: (traceId: string) =>
    apiClient.get(`/audit/traces/${traceId}`).then((r) => {
      const d = r.data?.data
      return d ? mapTrace(d) : null
    }),

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

// Backend returns Go struct field names (PascalCase); frontend uses snake_case.
function mapTrace(t: Record<string, unknown>): AuditTrace {
  return {
    trace_id: (t.TraceID ?? t.trace_id ?? '') as string,
    agent_id: (t.AgentID ?? t.agent_id ?? '') as string,
    request_type: (t.RequestType ?? t.request_type ?? '') as RequestType,
    method: (t.Method ?? t.method ?? '') as string,
    path: (t.Path ?? t.path ?? '') as string,
    status_code: (t.StatusCode ?? t.status_code ?? 0) as number,
    model: (t.Model ?? t.model ?? '') as string,
    tokens_in: (t.TokensIn ?? t.tokens_in ?? 0) as number,
    tokens_out: (t.TokensOut ?? t.tokens_out ?? 0) as number,
    latency_ms: (t.LatencyMs ?? t.latency_ms ?? 0) as number,
    request_body: (t.RequestBody ?? t.request_body ?? '') as string,
    response_body: (t.ResponseBody ?? t.response_body ?? '') as string,
    created_at: (t.CreatedAt ?? t.created_at ?? '') as string,
  }
}
