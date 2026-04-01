import {
  getAgents,
  getAgentsRuntimeTypes,
  getAgentsByName,
  postAgents,
  putAgentsByName,
  deleteAgentsByName,
  postAgentsByNamePause,
  postAgentsByNameResume,
  postAgentsByNameResetMemory,
  getAgentsByNameLogs,
  postAgentsByNameAssignIssue,
} from './generated/sdk.gen'
import type { Agent, AgentSpec, AgentStatus, CreateAgentDto, AssignTaskDto } from '../types/agent'

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
      maxTokens: spec.llm?.maxTokens ?? 65536,
    },
    runtime: {
      type: spec.runtime?.type ?? spec.runtimeImage ?? 'openclaw',
      image: spec.runtime?.image ?? spec.runtimeImage,
      env: spec.runtime?.env,
    },
    resources: {
      cpuRequest: spec.resources?.requests?.cpu ?? '500m',
      cpuLimit: spec.resources?.limits?.cpu ?? '5000m',
      memoryRequest: spec.resources?.requests?.memory ?? '1Gi',
      memoryLimit: spec.resources?.limits?.memory ?? '2Gi',
      workspaceSize: '5Gi',
    },
    gitea: {
      repo: spec.gitea?.repo ?? (spec.gitea?.repos?.[0] ?? ''),
      repos: spec.gitea?.repos ?? (spec.gitea?.repo ? [spec.gitea.repo] : []),
      permissions: (spec.gitea?.permissions ?? ['read']) as ('read' | 'write' | 'review' | 'merge')[],
    },
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

export interface RuntimeType {
  type: string
  image: string
  default: boolean
}

export const agentsApi = {
  getRuntimeTypes: () =>
    getAgentsRuntimeTypes().then((r) => {
      const body = r.data as any
      const items = (Array.isArray(body?.data) ? body.data : []) as Array<{ image: string; default: boolean }>
      return items.map((item) => ({
        type: item.image,
        image: item.image,
        default: item.default,
      })) as RuntimeType[]
    }),

  getAgents: () =>
    getAgents().then((r) => {
      const body = r.data as any
      const list = Array.isArray(body?.data) ? body.data : Array.isArray(body) ? body : []
      return list.map(mapAgent) as Agent[]
    }),

  getAgent: (name: string) =>
    getAgentsByName({ path: { name } }).then((r) => {
      const body = r.data as any
      const raw = body?.data ?? body
      return mapAgent(raw) as Agent
    }),

  createAgent: (dto: CreateAgentDto) =>
    postAgents({ body: dto as any }).then((r) => {
      const body = r.data as any
      return mapAgent(body?.data ?? body) as Agent
    }),

  updateAgent: (name: string, spec: Partial<AgentSpec>) =>
    putAgentsByName({ path: { name }, body: spec as any }).then((r) => {
      const body = r.data as any
      return mapAgent(body?.data ?? body) as Agent
    }),

  deleteAgent: (name: string) =>
    deleteAgentsByName({ path: { name } }),

  pauseAgent: (name: string) =>
    postAgentsByNamePause({ path: { name } }),

  resumeAgent: (name: string) =>
    postAgentsByNameResume({ path: { name } }),

  resetMemory: (name: string) =>
    postAgentsByNameResetMemory({ path: { name } }),

  getAgentLogs: (name: string) =>
    getAgentsByNameLogs({ path: { name } }).then((r) => {
      const body = r.data as any
      // Backend returns { success, data: "log string" }
      return (typeof body?.data === 'string' ? body.data : typeof body === 'string' ? body : '') as string
    }),

  assignTask: (name: string, dto: AssignTaskDto) =>
    postAgentsByNameAssignIssue({ path: { name }, body: dto as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as { issue_number: number; url: string }
    }),
}
