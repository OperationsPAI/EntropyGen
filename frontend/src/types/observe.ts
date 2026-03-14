// --- JSONL message types from OpenClaw completions ---

export interface SessionMessage {
  type: 'session'
  id: string
  cwd: string
  timestamp: string
}

export interface ModelChangeMessage {
  type: 'model_change'
  provider: string
  modelId: string
}

export interface ThinkingLevelChangeMessage {
  type: 'thinking_level_change'
  level: string
}

export interface ThinkingBlock {
  type: 'thinking'
  thinking: string
}

export interface TextBlock {
  type: 'text'
  text: string
}

export interface ToolCallBlock {
  type: 'toolCall'
  name: string
  arguments: Record<string, unknown>
}

export type ContentBlock = ThinkingBlock | TextBlock | ToolCallBlock

export interface TokenUsage {
  inputTokens: number
  outputTokens: number
  cacheReadInputTokens?: number
  cacheCreationInputTokens?: number
}

export interface UserMessage {
  type: 'message'
  role: 'user'
  content: string
  parentId?: string
}

export interface AssistantMessage {
  type: 'message'
  role: 'assistant'
  content: ContentBlock[]
  model?: string
  usage?: TokenUsage
  stopReason?: string
  parentId?: string
}

export interface ToolResultMessage {
  type: 'message'
  role: 'toolResult'
  name: string
  content: string
  isError?: boolean
  parentId?: string
}

export type JsonlMessage =
  | SessionMessage
  | ModelChangeMessage
  | ThinkingLevelChangeMessage
  | UserMessage
  | AssistantMessage
  | ToolResultMessage

// --- Sidecar API types ---

export interface SessionInfo {
  id: string
  started_at: string
  message_count: number
  is_current: boolean
  filename: string
}

export interface FileTreeNode {
  name: string
  type: 'file' | 'dir'
  modified: boolean
  children?: FileTreeNode[]
}

// --- WebSocket events from sidecar ---

export interface JsonlLiveEvent {
  type: 'jsonl'
  data: JsonlMessage
}

export interface FileChangeEvent {
  type: 'file_change'
  path: string
  action: 'modified' | 'created' | 'deleted'
}

export type SidecarWsEvent = JsonlLiveEvent | FileChangeEvent
