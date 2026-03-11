import { apiClient } from './client'
import type { Agent, CreateAgentDto, UpdateAgentDto, AssignTaskDto } from '../types/agent'

export const agentsApi = {
  getAgents: () =>
    apiClient.get<{ success: boolean; data: Agent[] }>('/agents').then((r) => r.data.data ?? []),

  getAgent: (name: string) =>
    apiClient.get<{ success: boolean; data: Agent }>(`/agents/${name}`).then((r) => r.data.data),

  createAgent: (dto: CreateAgentDto) =>
    apiClient.post<Agent>('/agents', dto).then((r) => r.data),

  updateAgent: (name: string, dto: UpdateAgentDto) =>
    apiClient.patch<Agent>(`/agents/${name}`, dto).then((r) => r.data),

  deleteAgent: (name: string) =>
    apiClient.delete(`/agents/${name}`),

  pauseAgent: (name: string) =>
    apiClient.post(`/agents/${name}/pause`),

  resumeAgent: (name: string) =>
    apiClient.post(`/agents/${name}/resume`),

  resetMemory: (name: string) =>
    apiClient.post(`/agents/${name}/reset-memory`),

  getAgentLogs: (name: string) =>
    apiClient.get<string>(`/agents/${name}/logs`).then((r) => r.data),

  assignTask: (name: string, dto: AssignTaskDto) =>
    apiClient.post<{ issue_number: number; url: string }>(`/agents/${name}/assign-task`, dto).then((r) => r.data),
}
