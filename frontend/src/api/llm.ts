import {
  getLlmModels,
  postLlmModels,
  putLlmModelsById,
  deleteLlmModelsById,
  postLlmHealthById,
  getLlmHealth,
  postLlmChat,
} from './generated/sdk.gen'

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

export interface ChatTestResult {
  reply: string
  model: string
  latencyMs: number
}

/* eslint-disable @typescript-eslint/no-explicit-any */

/** Map a raw LiteLLM /model/info entry to our LLMModel type. */
function mapLiteLLMModel(raw: any): LLMModel {
  const info = raw.model_info ?? {}
  return {
    id: info.id ?? raw.model_name ?? '',
    name: raw.model_name ?? '',
    provider: info.litellm_provider ?? raw.litellm_params?.model?.split('/')[0] ?? 'unknown',
    rpm: info.rpm ?? 0,
    tpm: info.tpm ?? 0,
    status: 'unknown',
  }
}

export const llmApi = {
  getModels: () =>
    getLlmModels().then((r) => {
      const body = r.data as any
      // LiteLLM returns { data: [{model_name, litellm_params, model_info}, ...] }
      const list = Array.isArray(body) ? body : Array.isArray(body?.data) ? body.data : []
      return list.map(mapLiteLLMModel) as LLMModel[]
    }),

  createModel: (dto: CreateModelDto) =>
    postLlmModels({ body: dto as any }).then(() => {
      // LiteLLM returns a success message, not the model object.
      // Return a synthetic LLMModel so the caller can update state.
      return {
        id: dto.name,
        name: dto.name,
        provider: dto.provider,
        rpm: dto.rpm,
        tpm: dto.tpm,
        status: 'unknown',
      } as LLMModel
    }),

  updateModel: (id: string, dto: Partial<CreateModelDto>) =>
    putLlmModelsById({ path: { id }, body: dto as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as LLMModel
    }),

  deleteModel: (id: string) =>
    deleteLlmModelsById({ path: { id } }),

  checkHealth: (id: string) =>
    postLlmHealthById({ path: { id } }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as { status: 'healthy' | 'unhealthy'; latency_ms?: number }
    }),

  /** Check LiteLLM service-level health (not per-model). */
  checkServiceHealth: () =>
    getLlmHealth().then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as Record<string, unknown>
    }),

  /** Send a test chat completion to verify end-to-end connectivity. */
  chatTest: (model: string, message: string): Promise<ChatTestResult> => {
    const start = Date.now()
    return postLlmChat({
      body: {
        model,
        messages: [{ role: 'user', content: message }],
        max_tokens: 64,
      } as any,
    }).then((r) => {
      const data = r.data as any
      return {
        reply: data?.choices?.[0]?.message?.content ?? JSON.stringify(data).slice(0, 200),
        model: data?.model ?? model,
        latencyMs: Date.now() - start,
      }
    })
  },
}
