export type AlertType = 'agent.crash_loop' | 'agent.oom_expanded' | 'agent.heartbeat_timeout'

export interface RealtimeEvent {
  event_type: string
  agent_id: string
  payload: Record<string, unknown>
  timestamp: string
}

export interface AlertEvent {
  id: string
  alert_type: AlertType
  agent_id: string
  message: string
  timestamp: string
  details?: Record<string, unknown>
}

export interface LLMInferencePayload {
  trace_id: string
  model: string
  tokens_in: number
  tokens_out: number
  latency_ms: number
}

export interface GiteaEventPayload {
  action: string
  repo: string
  url?: string
  title?: string
  number?: number
  sha?: string
}
