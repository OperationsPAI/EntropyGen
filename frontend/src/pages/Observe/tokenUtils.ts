// Token usage utilities.
// JSONL-based token aggregation has been removed.
// Token data now comes from agent.status.tokenUsage only.

export interface AggregatedTokenUsage {
  inputTokens: number
  outputTokens: number
  totalTokens: number
}
