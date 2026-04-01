import { apiClient } from './client'
import type { FileTreeNode, FileContentResponse, DiffResultResponse } from '../types/observe'

const observeBase = (agentName: string) => `/agents/${agentName}/observe`

export const observeApi = {
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

export function buildObserveWsUrl(agentName: string): string {
  const token = localStorage.getItem('jwt_token')
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/api/agents/${agentName}/observe/ws/live${token ? `?token=${token}` : ''}`
}
