import type { JsonlMessage, MessageEnvelope } from '../../types/observe'

export interface AggregatedTokenUsage {
  inputTokens: number
  outputTokens: number
  totalTokens: number
  messageCount: number
}

export function computeTokenUsage(messages: JsonlMessage[]): AggregatedTokenUsage {
  let inputTokens = 0
  let outputTokens = 0
  let messageCount = 0

  for (const msg of messages) {
    if (msg.type !== 'message') continue
    const { message: payload } = msg as MessageEnvelope
    if (payload.role === 'assistant' && payload.usage) {
      inputTokens += payload.usage.input
      outputTokens += payload.usage.output
      messageCount++
    }
  }

  return {
    inputTokens,
    outputTokens,
    totalTokens: inputTokens + outputTokens,
    messageCount,
  }
}
