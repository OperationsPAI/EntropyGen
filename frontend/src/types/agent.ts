export type AgentPhase = 'Pending' | 'Initializing' | 'Running' | 'Paused' | 'Error'
export type AgentRole = 'observer' | 'developer' | 'reviewer' | 'sre'

export interface LLMConfig {
  model: string
  temperature: number
  maxTokens: number
}

export interface CronConfig {
  schedule: string
  prompt: string
}

export interface ResourceConfig {
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  workspaceSize: string
}

export interface GiteaConfig {
  repo: string
  permissions: ('read' | 'write' | 'review' | 'merge')[]
}

export interface Condition {
  type: string
  status: 'True' | 'False' | 'Unknown'
  reason?: string
  message?: string
  lastTransitionTime?: string
}

export interface LastAction {
  description: string
  timestamp: string
}

export interface TokenUsage {
  today: number
  total: number
}

export interface AgentSpec {
  role: AgentRole
  soul: string
  llm: LLMConfig
  cron: CronConfig
  resources: ResourceConfig
  gitea: GiteaConfig
}

export interface AgentStatus {
  phase: AgentPhase
  conditions: Condition[]
  lastAction: LastAction | null
  tokenUsage: TokenUsage
  podName?: string
  createdAt: string
  giteaUsername?: string
}

export interface Agent {
  name: string
  spec: AgentSpec
  status: AgentStatus
}

export interface CreateAgentDto {
  name: string
  spec: Omit<AgentSpec, 'soul'>
}

export interface UpdateAgentDto {
  spec: Partial<AgentSpec>
}

export interface AssignTaskDto {
  repo: string
  title: string
  description: string
  labels: string[]
  priority: 'low' | 'medium' | 'high'
}
