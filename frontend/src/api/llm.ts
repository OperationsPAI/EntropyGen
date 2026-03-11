import { apiClient } from './client'

export interface LLMModel {
  id: string
  name: string
  provider: string
  apiKey?: string
  baseUrl?: string
  rpm: number
  tpm: number
  status: 'healthy' | 'unhealthy' | 'unknown'
}

export interface CreateModelDto {
  name: string
  provider: string
  apiKey: string
  baseUrl?: string
  rpm: number
  tpm: number
}

export const llmApi = {
  getModels: () =>
    apiClient.get<LLMModel[]>('/llm/models').then((r) => r.data),

  createModel: (dto: CreateModelDto) =>
    apiClient.post<LLMModel>('/llm/models', dto).then((r) => r.data),

  updateModel: (id: string, dto: Partial<CreateModelDto>) =>
    apiClient.patch<LLMModel>(`/llm/models/${id}`, dto).then((r) => r.data),

  deleteModel: (id: string) =>
    apiClient.delete(`/llm/models/${id}`),

  checkHealth: (id: string) =>
    apiClient.post<{ status: 'healthy' | 'unhealthy'; latency_ms?: number }>(`/llm/health/${id}`).then((r) => r.data),
}
