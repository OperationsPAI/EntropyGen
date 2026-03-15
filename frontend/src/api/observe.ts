import { apiClient } from './client'
import type { SessionInfo, FileTreeNode, FileContentResponse, DiffResultResponse, JsonlMessage } from '../types/observe'

const observeBase = (agentName: string) => `/agents/${agentName}/observe`

export const observeApi = {
  getSessions: (agentName: string) =>
    apiClient
      .get<SessionInfo[]>(`${observeBase(agentName)}/sessions`)
      .then((r) => r.data),

  getCurrentSession: (agentName: string) =>
    apiClient
      .get<string>(`${observeBase(agentName)}/sessions/current`, {
        transformResponse: [(data: string) => data],
      })
      .then((r) => parseJsonlLines(r.data)),

  getSession: (agentName: string, sessionId: string) =>
    apiClient
      .get<string>(`${observeBase(agentName)}/sessions/${sessionId}`, {
        transformResponse: [(data: string) => data],
      })
      .then((r) => parseJsonlLines(r.data)),

  getWorkspaceTree: (agentName: string) =>
    apiClient
      .get<FileTreeNode>(`${observeBase(agentName)}/workspace/tree`)
      .then((r) => {
        const root = r.data
        return root?.children ?? []
      }),

  getWorkspaceFile: (agentName: string, path: string) =>
    apiClient
      .get<FileContentResponse>(`${observeBase(agentName)}/workspace/file`, {
        params: { path },
      })
      .then((r) => r.data),

  getWorkspaceDiff: (agentName: string) =>
    apiClient
      .get<DiffResultResponse>(`${observeBase(agentName)}/workspace/diff`)
      .then((r) => r.data),
}

function parseJsonlLines(raw: string): JsonlMessage[] {
  if (!raw || !raw.trim()) return []
  return raw
    .trim()
    .split('\n')
    .map((line) => {
      try {
        return JSON.parse(line) as JsonlMessage
      } catch {
        return null
      }
    })
    .filter((m): m is JsonlMessage => m !== null)
}

export function buildObserveWsUrl(agentName: string): string {
  const token = localStorage.getItem('jwt_token')
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/api/agents/${agentName}/observe/ws/live${token ? `?token=${token}` : ''}`
}
