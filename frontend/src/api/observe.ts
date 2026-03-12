import { apiClient } from './client'
import type { SessionInfo, FileTreeNode, JsonlMessage } from '../types/observe'

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
      .get<FileTreeNode[]>(`${observeBase(agentName)}/workspace/tree`)
      .then((r) => r.data),

  getWorkspaceFile: (agentName: string, path: string) =>
    apiClient
      .get<string>(`${observeBase(agentName)}/workspace/file`, {
        params: { path },
        transformResponse: [(data: string) => data],
      })
      .then((r) => r.data),

  getWorkspaceDiff: (agentName: string) =>
    apiClient
      .get<string>(`${observeBase(agentName)}/workspace/diff`, {
        transformResponse: [(data: string) => data],
      })
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
