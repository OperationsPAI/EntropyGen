export type RequestType = 'llm_inference' | 'gitea_api' | 'git_http' | 'heartbeat'

export interface AuditTrace {
  trace_id: string
  agent_id: string
  request_type: RequestType
  method: string
  path: string
  status_code: number
  model?: string
  tokens_in?: number
  tokens_out?: number
  latency_ms: number
  request_body?: string
  response_body?: string
  created_at: string
}

export interface TraceFilter {
  agent_id?: string[]
  request_type?: RequestType
  status?: 'success' | 'error'
  start_time?: string
  end_time?: string
  page?: number
  limit?: number
}

export interface TraceListResponse {
  items: AuditTrace[]
  total: number
  page: number
  limit: number
}

export interface TokenUsageSummary {
  agent_id: string
  date: string
  tokens_in: number
  tokens_out: number
  request_count: number
  model: string
}
