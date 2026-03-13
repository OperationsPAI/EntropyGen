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
    apiClient.get('/llm/models').then((r) => {
      const body = r.data
      // LiteLLM returns { data: [...] }, extract the array
      const list = Array.isArray(body) ? body : Array.isArray(body?.data) ? body.data : []
      return list as LLMModel[]
    }),

  createModel: (dto: CreateModelDto) =>
    apiClient.post<LLMModel>('/llm/models', dto).then((r) => r.data),

  updateModel: (id: string, dto: Partial<CreateModelDto>) =>
    apiClient.patch<LLMModel>(`/llm/models/${id}`, dto).then((r) => r.data),

  deleteModel: (id: string) =>
    apiClient.delete(`/llm/models/${id}`),

  checkHealth: (id: string) =>
    apiClient.post<{ status: 'healthy' | 'unhealthy'; latency_ms?: number }>(`/llm/health/${id}`).then((r) => r.data),
}
