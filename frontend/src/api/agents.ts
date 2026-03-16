import { apiClient } from './client'
import type { Agent, AgentSpec, AgentStatus, CreateAgentDto, UpdateAgentDto, AssignTaskDto } from '../types/agent'

/* eslint-disable @typescript-eslint/no-explicit-any */

/** Map a raw K8s CRD Agent object to the frontend Agent type. */
function mapAgent(raw: any): Agent {
  if (!raw) return raw
  // If already in flat format, return as-is
  if (!raw.metadata && raw.name) return raw as Agent

  const meta = raw.metadata ?? {}
  const spec = raw.spec ?? {}
  const status = raw.status ?? {}

  const mappedSpec: AgentSpec = {
    role: spec.role ?? '',
    llm: {
      model: spec.llm?.model ?? '',
      temperature: spec.llm?.temperature ?? 0.7,
      maxTokens: spec.llm?.maxTokens ?? 4096,
    },
    cron: {
      schedule: spec.cron?.schedule ?? '',
    },
    resources: {
      cpuRequest: spec.resources?.requests?.cpu ?? '100m',
      cpuLimit: spec.resources?.limits?.cpu ?? '500m',
      memoryRequest: spec.resources?.requests?.memory ?? '256Mi',
      memoryLimit: spec.resources?.limits?.memory ?? '1Gi',
      workspaceSize: '5Gi',
    },
    gitea: {
      repo: spec.gitea?.repo ?? '',
      permissions: (spec.gitea?.permissions ?? ['read']) as ('read' | 'write' | 'review' | 'merge')[],
    },
    runtimeImage: spec.runtimeImage ?? '',
  }

  const mappedStatus: AgentStatus = {
    phase: (status.phase || (spec.paused ? 'Paused' : 'Pending')) as AgentStatus['phase'],
    conditions: (status.conditions ?? []).map((c: any) => ({
      type: c.type ?? '',
      status: c.status ?? 'Unknown',
      reason: c.reason,
      message: c.message,
      lastTransitionTime: c.lastTransitionTime,
    })),
    lastAction: status.lastAction
      ? { description: status.lastAction.description ?? '', timestamp: status.lastAction.timestamp ?? '' }
      : null,
    tokenUsage: {
      today: status.tokenUsage?.today ?? 0,
      total: status.tokenUsage?.total ?? 0,
    },
    podName: status.podName,
    createdAt: meta.creationTimestamp ?? '',
    giteaUsername: status.giteaUser?.username,
    currentTask: status.currentTask
      ? {
          type: status.currentTask.type as 'issue' | 'pr',
          number: status.currentTask.number,
          title: status.currentTask.title,
          repo: status.currentTask.repo,
        }
      : undefined,
  }

  return {
    name: meta.name ?? '',
    spec: mappedSpec,
    status: mappedStatus,
  }
}

export interface RuntimeImage {
  image: string
  default: boolean
}

export const agentsApi = {
  getRuntimeImages: () =>
    apiClient.get('/agents/runtime-images').then((r) => {
      const body = r.data
      return (Array.isArray(body?.data) ? body.data : []) as RuntimeImage[]
    }),

  getAgents: () =>
    apiClient.get('/agents').then((r) => {
      const body = r.data
      const list = Array.isArray(body?.data) ? body.data : Array.isArray(body) ? body : []
      return list.map(mapAgent) as Agent[]
    }),

  getAgent: (name: string) =>
    apiClient.get(`/agents/${name}`).then((r) => {
      const body = r.data
      const raw = body?.data ?? body
      return mapAgent(raw) as Agent
    }),

  createAgent: (dto: CreateAgentDto) =>
    apiClient.post('/agents', dto).then((r) => {
      const body = r.data
      return mapAgent(body?.data ?? body) as Agent
    }),

  updateAgent: (name: string, dto: UpdateAgentDto) =>
    apiClient.patch(`/agents/${name}`, dto).then((r) => {
      const body = r.data
      return mapAgent(body?.data ?? body) as Agent
    }),

  deleteAgent: (name: string) =>
    apiClient.delete(`/agents/${name}`),

  pauseAgent: (name: string) =>
    apiClient.post(`/agents/${name}/pause`),

  resumeAgent: (name: string) =>
    apiClient.post(`/agents/${name}/resume`),

  resetMemory: (name: string) =>
    apiClient.post(`/agents/${name}/reset-memory`),

  getAgentLogs: (name: string) =>
    apiClient.get(`/agents/${name}/logs`).then((r) => {
      const body = r.data
      // Backend returns { success, data: "log string" }
      return (typeof body?.data === 'string' ? body.data : typeof body === 'string' ? body : '') as string
    }),

  assignTask: (name: string, dto: AssignTaskDto) =>
    apiClient.post<{ issue_number: number; url: string }>(`/agents/${name}/assign-task`, dto).then((r) => r.data),
}
