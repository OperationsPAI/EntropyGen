export type AgentPhase = 'Pending' | 'Initializing' | 'Running' | 'Paused' | 'Error'

export interface LLMConfig {
  model: string
  temperature: number
  maxTokens: number
}

export interface CronConfig {
  schedule: string
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
  role: string
  llm: LLMConfig
  cron: CronConfig
  resources: ResourceConfig
  gitea: GiteaConfig
  runtimeImage?: string
}

export interface CurrentTask {
  type: 'issue' | 'pr'
  number: number
  title?: string
  repo?: string
}

export interface AgentStatus {
  phase: AgentPhase
  conditions: Condition[]
  lastAction: LastAction | null
  tokenUsage: TokenUsage
  podName?: string
  createdAt: string
  giteaUsername?: string
  currentTask?: CurrentTask
}

export interface Agent {
  name: string
  spec: AgentSpec
  status: AgentStatus
}

export interface CreateAgentDto {
  name: string
  spec: AgentSpec
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

// --- Role types ---

export interface RoleFile {
  name: string
  content: string
  updated_at: string
}

export interface Role {
  name: string
  description: string
  files: RoleFile[]
  file_count: number
  agent_count: number
  created_at: string
  updated_at: string
}

export interface CreateRoleDto {
  name: string
  description: string
  role?: string // role type for builtin template selection (e.g. developer, reviewer)
  initial_files?: string[]
  files?: Record<string, string> // explicit file contents (overrides builtin defaults)
}
